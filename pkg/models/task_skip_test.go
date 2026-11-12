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
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkipTaskOccurrence(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("moves on without completing", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// Task 28 repeats every hour and has a due, start and end date.
		before, err := GetTaskByIDSimple(s, 28)
		require.NoError(t, err)
		next, deleted, err := SkipTaskOccurrence(s, u, 28)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		assert.False(t, deleted)
		assert.False(t, next.Done)
		assert.True(t, next.DoneAt.IsZero())
		// Interval repeats move every date past now, the way completing the task does.
		assert.True(t, next.DueDate.After(before.DueDate))
		assert.True(t, next.DueDate.After(time.Now()))
		assert.True(t, next.StartDate.After(before.StartDate))
		db.AssertExists(t, "tasks", map[string]any{"id": 28, "done": false}, false)
	})

	t.Run("the last occurrence trashes the task", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		task := &Task{ID: 1}
		require.NoError(t, task.ReadOne(s, u))
		task.DueDate = time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
		task.RepeatRule = "FREQ=WEEKLY;COUNT=1"
		require.NoError(t, task.UpdateColumns(s, u, []string{"due_date", "repeat_rule"}))

		_, deleted, err := SkipTaskOccurrence(s, u, 1)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		assert.True(t, deleted)
		trashed := &Task{}
		s2 := db.NewSession()
		defer s2.Close()
		has, err := s2.Unscoped().ID(1).Get(trashed)
		require.NoError(t, err)
		require.True(t, has)
		assert.False(t, trashed.DeletedAt.IsZero())
	})

	t.Run("a task that doesn't repeat", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, err := SkipTaskOccurrence(s, u, 1)
		require.Error(t, err)
		assert.True(t, IsErrTaskNotRepeating(err))
	})
}
