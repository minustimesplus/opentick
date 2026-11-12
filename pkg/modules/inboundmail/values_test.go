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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseWhen(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	// Thursday, 12 November 2026, 10:00 in Berlin.
	now := time.Date(2026, 11, 12, 10, 0, 0, 0, loc)

	cases := []struct {
		in    string
		want  time.Time
		timed bool
	}{
		{"2026-11-20", time.Date(2026, 11, 20, 0, 0, 0, 0, loc), false},
		{"2026-11-20 17:00", time.Date(2026, 11, 20, 17, 0, 0, 0, loc), true},
		{"2026-11-20T09:30", time.Date(2026, 11, 20, 9, 30, 0, 0, loc), true},
		{"2026-11-20T17:00:00Z", time.Date(2026, 11, 20, 18, 0, 0, 0, loc), true},
		{"today", time.Date(2026, 11, 12, 0, 0, 0, 0, loc), false},
		{"Tomorrow 9am", time.Date(2026, 11, 13, 9, 0, 0, 0, loc), true},
		{"tomorrow at 12pm", time.Date(2026, 11, 13, 12, 0, 0, 0, loc), true},
		{"friday 14:30", time.Date(2026, 11, 13, 14, 30, 0, 0, loc), true},
		{"next monday", time.Date(2026, 11, 16, 0, 0, 0, 0, loc), false},
		{"thursday", time.Date(2026, 11, 12, 0, 0, 0, 0, loc), false},
		{"2026-11-20 12am", time.Date(2026, 11, 20, 0, 0, 0, 0, loc), true},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := parseWhen(c.in, now, loc)
			require.NoError(t, err)
			assert.True(t, c.want.Equal(got.at), "got %s", got.at)
			assert.Equal(t, c.timed, got.timed)
		})
	}

	for _, bad := range []string{"", "someday", "2026-13-01", "tomorrow 25:00", "friday noonish"} {
		_, err := parseWhen(bad, now, loc)
		assert.Error(t, err, bad)
	}
}

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"45m":               45 * time.Minute,
		"1h30m":             90 * time.Minute,
		"1.5h":              90 * time.Minute,
		"2 hours":           2 * time.Hour,
		"90 minutes":        90 * time.Minute,
		"3d":                72 * time.Hour,
		"1 hour and 15 min": 75 * time.Minute,
		"30":                30 * time.Minute,
	}
	for in, want := range cases {
		got, err := parseDuration(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", "soon", "0m", "5 weeks"} {
		_, err := parseDuration(bad)
		assert.Error(t, err, bad)
	}
}

func TestParseRepeat(t *testing.T) {
	cases := map[string]string{
		"daily":                         "FREQ=DAILY",
		"Every week":                    "FREQ=WEEKLY",
		"every 2 weeks":                 "FREQ=WEEKLY;INTERVAL=2",
		"every 3 months":                "FREQ=MONTHLY;INTERVAL=3",
		"yearly":                        "FREQ=YEARLY",
		"weekdays":                      "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR",
		"every monday and thursday":     "FREQ=WEEKLY;BYDAY=MO,TH",
		"every Mon, Fri":                "FREQ=WEEKLY;BYDAY=MO,FR",
		"every 2 weeks on friday":       "FREQ=WEEKLY;INTERVAL=2;BYDAY=FR",
		"daily until 2026-12-31":        "FREQ=DAILY;UNTIL=20261231T235959Z",
		"weekly 10 times":               "FREQ=WEEKLY;COUNT=10",
		"RRULE:FREQ=MONTHLY;BYDAY=-1FR": "FREQ=MONTHLY;BYDAY=-1FR",
		"freq=monthly;bymonthday=1":     "FREQ=MONTHLY;BYMONTHDAY=1",
	}
	for in, want := range cases {
		got, err := parseRepeat(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	for _, bad := range []string{"", "sometimes", "every 2 months on friday", "every blursday"} {
		_, err := parseRepeat(bad)
		assert.Error(t, err, bad)
	}
}

func TestParsePriority(t *testing.T) {
	for in, want := range map[string]int64{"none": 0, "Low": 1, "medium": 2, "high": 3, "urgent": 3, "2": 2} {
		got, err := parsePriority(in)
		require.NoError(t, err)
		assert.Equal(t, want, got, in)
	}
	_, err := parsePriority("very")
	assert.Error(t, err)
}

func TestCleanBody(t *testing.T) {
	body := "done: 12\r\n\r\nThanks!\r\n-- \r\nSent by a bot\r\n"
	assert.Equal(t, "done: 12\n\nThanks!", cleanBody(body))

	quoted := "comment: 4\ntext: All set.\n\nOn Thu, 12 Nov 2026 at 10:00, OpenTick <a@b.c> wrote:\n> add: should not run\n"
	assert.Equal(t, "comment: 4\ntext: All set.", cleanBody(quoted))

	withReference := "Booked it.\n\n== OpenTick email commands ==\nadd: <title>\n"
	assert.Equal(t, "Booked it.", cleanBody(withReference))

	inline := "> add: quoted\ndone: 3"
	assert.Equal(t, "done: 3", cleanBody(inline))
}

func TestParseCommands(t *testing.T) {
	body := `Add: Buy milk
List: Groceries
date: tomorrow 17:00
- duration: 30m
notes: Two litres.

Organic if possible.

lists

delete: 42
scope: this`
	cmds := parseCommands(body)
	require.Len(t, cmds, 3)

	assert.Equal(t, "add", cmds[0].Verb)
	assert.Equal(t, "Buy milk", cmds[0].Arg)
	assert.Equal(t, "Groceries", cmds[0].Fields["list"])
	assert.Equal(t, "tomorrow 17:00", cmds[0].Fields["date"])
	assert.Equal(t, "30m", cmds[0].Fields["duration"])
	assert.Equal(t, "Two litres.\n\nOrganic if possible.", cmds[0].Fields["notes"])
	assert.Empty(t, cmds[0].Unknown)

	assert.Equal(t, "lists", cmds[1].Verb)

	assert.Equal(t, "delete", cmds[2].Verb)
	assert.Equal(t, "42", cmds[2].Arg)
	assert.Equal(t, "this", cmds[2].Fields["scope"])

	t.Run("plain text has no commands", func(t *testing.T) {
		assert.Empty(t, parseCommands("Hi! I booked the table for Friday.\nSee you there."))
	})
	t.Run("unknown lines are kept for the error", func(t *testing.T) {
		cmds := parseCommands("add: X\ncolour: red")
		require.Len(t, cmds, 1)
		assert.Equal(t, []string{"colour: red"}, cmds[0].Unknown)
	})
	t.Run("repeat from with a space", func(t *testing.T) {
		cmds := parseCommands("add: X\nrepeat: daily\nrepeat from: completion")
		require.Len(t, cmds, 1)
		assert.Equal(t, "completion", cmds[0].Fields["repeat_from"])
	})
}
