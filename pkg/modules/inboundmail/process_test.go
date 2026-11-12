// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package inboundmail

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	vmail "code.vikunja.io/api/pkg/mail"
	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// Fixtures: user1 (user1@example.com) has inbound secret abcdef123456 and
// Instinct address instinct@example.com in agent_configs.yml; project 1 is theirs.
const user1Secret = "abcdef123456"

var mailCounter atomic.Int64

func rawMail(from, to, subject, body string) []byte {
	id := mailCounter.Add(1)
	return []byte(fmt.Sprintf("From: Agent <%s>\r\nTo: %s\r\nSubject: %s\r\nMessage-ID: <test-%d@example.com>\r\n"+
		"Date: Thu, 12 Nov 2026 10:00:00 +0000\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n",
		from, to, subject, id, strings.ReplaceAll(body, "\n", "\r\n")))
}

type mailHarness struct {
	t       *testing.T
	replies []*vmail.Opts
	loc     *time.Location
	now     time.Time
}

func setupMail(t *testing.T) *mailHarness {
	t.Helper()
	db.LoadAndAssertFixtures(t)
	h := &mailHarness{t: t, loc: config.GetTimeZone()}
	h.now = time.Date(2026, 11, 12, 10, 0, 0, 0, h.loc)

	config.InboundMailEnabled.Set(true)
	config.InboundMailHost.Set("imap.example.com")
	config.InboundMailAddress.Set("inbox@example.com")
	config.MailerEnabled.Set(true)
	origSend, origNow := sendReply, now
	sendReply = func(opts *vmail.Opts) { h.replies = append(h.replies, opts) }
	now = func() time.Time { return h.now }
	t.Cleanup(func() {
		config.InboundMailEnabled.Set(false)
		config.InboundMailHost.Set("")
		config.InboundMailAddress.Set("")
		config.MailerEnabled.Set(false)
		sendReply, now = origSend, origNow
	})
	return h
}

// send applies a mail from Instinct to user1's address (or a task reply address) and returns the reply text.
func (h *mailHarness) send(to, subject, body string) string {
	return h.sendFrom("instinct@example.com", to, subject, body)
}

func (h *mailHarness) sendFrom(from, to, subject, body string) string {
	h.t.Helper()
	before := len(h.replies)
	require.NoError(h.t, Process(rawMail(from, to, subject, body)))
	if len(h.replies) == before {
		return ""
	}
	return h.replies[len(h.replies)-1].Message
}

func (h *mailHarness) taskByTitle(title string) *models.Task {
	h.t.Helper()
	s := db.NewSession()
	defer s.Close()
	task := &models.Task{}
	has, err := s.Where("title = ?", title).Get(task)
	require.NoError(h.t, err)
	require.True(h.t, has, "task %q was not created", title)
	return task
}

func (h *mailHarness) task(id int64) *models.Task {
	h.t.Helper()
	s := db.NewSession()
	defer s.Close()
	task := &models.Task{}
	has, err := s.Unscoped().ID(id).Get(task)
	require.NoError(h.t, err)
	require.True(h.t, has)
	return task
}

func address(taskID int64) string {
	return models.InboundAddressFor(user1Secret, taskID)
}

func TestProcess_Add(t *testing.T) {
	h := setupMail(t)
	reply := h.send(address(0), "From your agent", `add: Dentist appointment
list: Test1
date: 2026-11-20 14:30
duration: 45m
repeat: every 6 months
priority: high
labels: Label #1, Health
notes: Bring the **insurance card**.

Ask about the bill.`)

	require.Contains(t, reply, "OK Created #")
	task := h.taskByTitle("Dentist appointment")
	assert.Equal(t, int64(1), task.ProjectID)
	start := time.Date(2026, 11, 20, 14, 30, 0, 0, h.loc)
	assert.True(t, start.Equal(task.DueDate), "due %s", task.DueDate)
	assert.True(t, start.Equal(task.StartDate), "start %s", task.StartDate)
	assert.True(t, start.Add(45*time.Minute).Equal(task.EndDate), "end %s", task.EndDate)
	assert.Equal(t, "FREQ=MONTHLY;INTERVAL=6", task.RepeatRule)
	assert.Equal(t, int64(3), task.Priority)
	assert.Contains(t, task.Description, "<strong>insurance card</strong>")
	assert.Contains(t, task.Description, "Ask about the bill.")
	db.AssertExists(t, "label_tasks", map[string]any{"task_id": task.ID, "label_id": 1}, false)
	db.AssertExists(t, "labels", map[string]any{"title": "Health", "created_by_id": 1}, false)

	t.Run("all-day over several days", func(t *testing.T) {
		reply := h.send(address(0), "", "add: Conference\ndate: 2026-12-01\nduration: 3d")
		require.Contains(t, reply, "OK Created")
		task := h.taskByTitle("Conference")
		assert.True(t, time.Date(2026, 12, 1, 0, 0, 0, 0, h.loc).Equal(task.StartDate))
		assert.True(t, time.Date(2026, 12, 3, 23, 59, 59, 0, h.loc).Equal(task.EndDate))
		assert.True(t, time.Date(2026, 12, 1, 23, 59, 59, 0, h.loc).Equal(task.DueDate))
	})
	t.Run("an all-day date with minutes is refused", func(t *testing.T) {
		reply := h.send(address(0), "", "add: Call mum\ndate: 2026-12-01\nduration: 30m")
		assert.Contains(t, reply, "FAILED add: Call mum: A 30 minute block needs a time")
		db.AssertMissing(t, "tasks", map[string]any{"title": "Call mum"})
	})
	t.Run("repeat needs a date", func(t *testing.T) {
		reply := h.send(address(0), "", "add: Water plants\nrepeat: weekly")
		assert.Contains(t, reply, "needs a date")
		db.AssertMissing(t, "tasks", map[string]any{"title": "Water plants"})
	})
	t.Run("unknown list", func(t *testing.T) {
		reply := h.send(address(0), "", "add: Lost\nlist: Nowhere")
		assert.Contains(t, reply, `There's no list called "Nowhere"`)
	})
	t.Run("one failing command doesn't stop the next", func(t *testing.T) {
		reply := h.send(address(0), "", "add: First\ncolour: blue\n\nadd: Second")
		assert.Contains(t, reply, "FAILED add: First")
		assert.Contains(t, reply, "OK Created")
		h.taskByTitle("Second")
	})
}

func TestProcess_Update(t *testing.T) {
	h := setupMail(t)
	require.Contains(t, h.send(address(0), "", "add: Workout\ndate: 2026-11-13 07:00\nduration: 1h"), "OK")
	task := h.taskByTitle("Workout")

	reply := h.send(address(0), "", fmt.Sprintf("update: #%d\ntitle: Morning workout\ndate: 2026-11-14 08:00\nrepeat: weekdays\nrepeat from: completion", task.ID))
	require.Contains(t, reply, "OK Updated")
	task = h.task(task.ID)
	assert.Equal(t, "Morning workout", task.Title)
	start := time.Date(2026, 11, 14, 8, 0, 0, 0, h.loc)
	assert.True(t, start.Equal(task.DueDate))
	assert.True(t, start.Add(time.Hour).Equal(task.EndDate), "a new date keeps the duration")
	assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR", task.RepeatRule)
	assert.Equal(t, models.TaskRepeatModeFromCurrentDate, task.RepeatMode)

	reply = h.send(address(0), "", fmt.Sprintf("update: %d\nduration: none\nrepeat: none", task.ID))
	require.Contains(t, reply, "OK Updated")
	task = h.task(task.ID)
	assert.True(t, task.StartDate.IsZero())
	assert.True(t, task.EndDate.IsZero())
	assert.True(t, start.Equal(task.DueDate))
	assert.Empty(t, task.RepeatRule)
}

func TestProcess_DeleteRepeating(t *testing.T) {
	h := setupMail(t)
	require.Contains(t, h.send(address(0), "", "add: Team sync\ndate: 2026-11-16 09:00\nduration: 30m\nrepeat: weekly"), "OK")
	task := h.taskByTitle("Team sync")
	first := time.Date(2026, 11, 16, 9, 0, 0, 0, h.loc)

	reply := h.send(address(0), "", fmt.Sprintf("delete: %d", task.ID))
	assert.Contains(t, reply, "repeats. Add \"scope: this\"")
	assert.True(t, first.Equal(h.task(task.ID).DueDate), "nothing changes without a scope")

	reply = h.send(address(0), "", fmt.Sprintf("delete: %d\nscope: this", task.ID))
	require.Contains(t, reply, "OK Skipped this occurrence")
	skipped := h.task(task.ID)
	assert.False(t, skipped.Done)
	assert.True(t, skipped.DoneAt.IsZero(), "skipping is not completing")
	assert.True(t, first.AddDate(0, 0, 7).Equal(skipped.DueDate), "due %s", skipped.DueDate)
	assert.True(t, first.AddDate(0, 0, 7).Add(30*time.Minute).Equal(skipped.EndDate))
	assert.True(t, skipped.DeletedAt.IsZero())

	reply = h.send(address(0), "", fmt.Sprintf("delete: %d\nscope: future", task.ID))
	require.Contains(t, reply, "OK Moved repeating task")
	assert.False(t, h.task(task.ID).DeletedAt.IsZero(), "the series is in the trash")
}

func TestProcess_DoneAndDelete(t *testing.T) {
	h := setupMail(t)
	require.Contains(t, h.send(address(0), "", "add: Pay rent\ndate: 2026-12-01\nrepeat: monthly"), "OK")
	task := h.taskByTitle("Pay rent")

	reply := h.send(address(0), "", fmt.Sprintf("done: %d", task.ID))
	require.Contains(t, reply, "OK Completed this occurrence; next:")
	next := h.task(task.ID)
	assert.False(t, next.Done)
	assert.Equal(t, time.January, next.DueDate.In(h.loc).Month())

	require.Contains(t, h.send(address(0), "", "add: One-off"), "OK")
	oneOff := h.taskByTitle("One-off")
	assert.Contains(t, h.send(address(0), "", fmt.Sprintf("skip: %d", oneOff.ID)), "doesn't repeat")
	assert.Contains(t, h.send(address(0), "", fmt.Sprintf("delete: %d", oneOff.ID)), "OK Moved task")
	assert.False(t, h.task(oneOff.ID).DeletedAt.IsZero())
	assert.Contains(t, h.send(address(0), "", fmt.Sprintf("done: %d", oneOff.ID)), "doesn't exist")
}

func TestProcess_TaskReplies(t *testing.T) {
	h := setupMail(t)

	t.Run("plain reply becomes a comment", func(t *testing.T) {
		reply := h.send(address(1), "Re: OpenTick task: task #1", "I booked the table for 8pm.\n\nOn Thu, 12 Nov 2026 at 10:00, OpenTick <inbox@example.com> wrote:\n> add: Quoted command\n")
		require.Contains(t, reply, "OK Commented on task 1.")
		db.AssertExists(t, "task_comments", map[string]any{"task_id": 1, "author_id": 1, "comment": "<p>I booked the table for 8pm.</p>"}, false)
		db.AssertMissing(t, "tasks", map[string]any{"title": "Quoted command"})
	})
	t.Run("commands default to the task replied to", func(t *testing.T) {
		reply := h.send(address(1), "Re: task", "comment:\ntext: Done and dusted.\n\ndone:")
		require.Contains(t, reply, "OK Commented on task 1.")
		require.Contains(t, reply, "OK Completed")
		assert.True(t, h.task(1).Done)
	})
	t.Run("replies go back to the task address", func(t *testing.T) {
		h.send(address(1), "Re: task", "Thanks")
		last := h.replies[len(h.replies)-1]
		assert.Equal(t, address(1), last.ReplyTo)
		assert.Equal(t, "instinct@example.com", last.To)
		assert.True(t, last.AutoReply)
	})
}

func TestProcess_Fallbacks(t *testing.T) {
	h := setupMail(t)

	reply := h.send(address(0), "Fwd: Renew passport", "The old one runs out in March.")
	require.Contains(t, reply, "OK Created")
	task := h.taskByTitle("Renew passport")
	assert.Contains(t, task.Description, "The old one runs out in March.")

	reply = h.send(address(0), "", "lists")
	assert.Contains(t, reply, "Test1 (ID 1)")

	reply = h.send(address(0), "", "find: passport")
	assert.Contains(t, reply, fmt.Sprintf("#%d \"Renew passport\"", task.ID))

	reply = h.send(address(0), "", "help")
	assert.Contains(t, reply, models.InboundCommandReferenceMarker)
}

func TestProcess_Refused(t *testing.T) {
	h := setupMail(t)

	t.Run("unknown sender", func(t *testing.T) {
		reply := h.sendFrom("stranger@example.org", address(0), "", "add: Spam")
		assert.Empty(t, reply, "strangers get no answer")
		db.AssertMissing(t, "tasks", map[string]any{"title": "Spam"})
	})
	t.Run("the account address may send", func(t *testing.T) {
		require.Contains(t, h.sendFrom("user1@example.com", address(0), "", "add: From me"), "OK")
	})
	t.Run("unknown secret", func(t *testing.T) {
		require.NoError(t, Process(rawMail("instinct@example.com", "inbox+zzzzzzzzzzzz@example.com", "", "add: Nobody")))
		db.AssertMissing(t, "tasks", map[string]any{"title": "Nobody"})
	})
	t.Run("failed dmarc", func(t *testing.T) {
		raw := append([]byte("Authentication-Results: mx.example.com; dmarc=fail header.from=example.com\r\n"),
			rawMail("instinct@example.com", address(0), "", "add: Spoofed")...)
		require.NoError(t, Process(raw))
		db.AssertMissing(t, "tasks", map[string]any{"title": "Spoofed"})
	})
	t.Run("the same mail twice", func(t *testing.T) {
		raw := rawMail("instinct@example.com", address(0), "", "add: Only once")
		require.NoError(t, Process(raw))
		require.NoError(t, Process(raw))
		db.AssertCount(t, "tasks", builder.Eq{"title": "Only once"}, 1)
	})
	t.Run("no access to the task", func(t *testing.T) {
		s := db.NewSession()
		_, err := s.Insert(&models.AgentConfig{UserID: 2, InboundSecret: "user2secret1"})
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		s.Close()
		reply := h.sendFrom("user2@example.com", models.InboundAddressFor("user2secret1", 0), "", "done: 1")
		assert.Contains(t, reply, "FAILED done: 1")
		assert.False(t, h.task(1).Done)
	})
	t.Run("automatic mail gets no answer", func(t *testing.T) {
		raw := append([]byte("Auto-Submitted: auto-replied\r\n"), rawMail("instinct@example.com", address(0), "", "add: Auto")...)
		before := len(h.replies)
		require.NoError(t, Process(raw))
		assert.Len(t, h.replies, before)
		h.taskByTitle("Auto")
	})
}
