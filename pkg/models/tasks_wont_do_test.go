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

package models

import (
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTask_WontDo(t *testing.T) {
	u := &user.User{ID: 1}

	readTask := func(t *testing.T, id int64) *Task {
		t.Helper()
		s := db.NewSession()
		defer s.Close()
		task := &Task{ID: id}
		require.NoError(t, task.ReadOne(s, u))
		return task
	}

	update := func(t *testing.T, task *Task, fields ...string) {
		t.Helper()
		s := db.NewSession()
		defer s.Close()
		var err error
		if len(fields) > 0 {
			err = task.updateSingleTask(s, u, fields)
		} else {
			err = task.Update(s, u)
		}
		require.NoError(t, err)
		require.NoError(t, s.Commit())
	}

	t.Run("marking won't do closes the task", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)

		update(t, &Task{ID: 1, Title: "task #1", WontDo: true})

		got := readTask(t, 1)
		assert.True(t, got.WontDo)
		assert.True(t, got.Done, "won't do implies done")
		assert.False(t, got.DoneAt.IsZero(), "closing as won't do sets done_at")
	})
	t.Run("marking won't do with only the wont_do field", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)

		update(t, &Task{ID: 1, WontDo: true}, "wont_do")

		got := readTask(t, 1)
		assert.True(t, got.WontDo)
		assert.True(t, got.Done)
		assert.Equal(t, "task #1", got.Title)
	})
	t.Run("re-opening a won't do task clears the flag", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		update(t, &Task{ID: 1, Title: "task #1", WontDo: true})

		// A full update that still carries the stale wont_do flag re-opens the task.
		update(t, &Task{ID: 1, Title: "task #1", Done: false, WontDo: true})

		got := readTask(t, 1)
		assert.False(t, got.Done)
		assert.False(t, got.WontDo)
		assert.True(t, got.DoneAt.IsZero())
	})
	t.Run("re-opening with only the done field clears the flag", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		update(t, &Task{ID: 1, Title: "task #1", WontDo: true})

		update(t, &Task{ID: 1, Done: false}, "done")

		got := readTask(t, 1)
		assert.False(t, got.Done)
		assert.False(t, got.WontDo)
	})
	t.Run("completing a won't do task turns it into done", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		update(t, &Task{ID: 1, Title: "task #1", WontDo: true})

		update(t, &Task{ID: 1, Title: "task #1", Done: true, WontDo: false})

		got := readTask(t, 1)
		assert.True(t, got.Done)
		assert.False(t, got.WontDo)
	})
	t.Run("unrelated partial updates keep the flag", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		update(t, &Task{ID: 1, Title: "task #1", WontDo: true})

		update(t, &Task{ID: 1, Title: "renamed"}, "title")

		got := readTask(t, 1)
		assert.Equal(t, "renamed", got.Title)
		assert.True(t, got.Done)
		assert.True(t, got.WontDo)
	})
	t.Run("an already done task can be changed to won't do", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)

		update(t, &Task{ID: 2, Title: "task #2 done", Done: true, WontDo: true})

		got := readTask(t, 2)
		assert.True(t, got.Done)
		assert.True(t, got.WontDo)
	})
	t.Run("won't do on a repeating task skips the occurrence", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		before := readTask(t, 28)

		task := &Task{ID: 28, Title: before.Title, DueDate: before.DueDate, RepeatAfter: 3600, WontDo: true}
		update(t, task)

		got := readTask(t, 28)
		assert.False(t, got.Done, "the next occurrence is open")
		assert.False(t, got.WontDo, "the next occurrence is not won't do")
		assert.True(t, got.DueDate.After(before.DueDate), "due date moves to the next occurrence")
	})
	t.Run("creating a task as won't do closes it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		task := &Task{Title: "abandoned", ProjectID: 1, WontDo: true}
		require.NoError(t, task.Create(s, u))
		require.NoError(t, s.Commit())

		db.AssertExists(t, "tasks", map[string]interface{}{
			"id":      task.ID,
			"done":    true,
			"wont_do": true,
		}, false)
	})
	t.Run("filter by wont_do", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		update(t, &Task{ID: 1, Title: "task #1", WontDo: true})

		ids := func(filter string) []int64 {
			s := db.NewSession()
			defer s.Close()
			c := &TaskCollection{ProjectID: 1, Filter: filter}
			res, _, _, err := c.ReadAll(s, u, "", 0, 50)
			require.NoError(t, err)
			tasks, ok := res.([]*Task)
			require.True(t, ok)
			out := []int64{}
			for _, tsk := range tasks {
				out = append(out, tsk.ID)
			}
			return out
		}

		assert.Equal(t, []int64{1}, ids("wont_do = true"))
		assert.ElementsMatch(t, []int64{1, 2}, ids("done = true"), "won't do tasks count as closed")
		assert.Equal(t, []int64{2}, ids("done = true && wont_do = false"), "but not as completed")
		assert.NotContains(t, ids("done = false"), int64(1))
	})
}
