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

package migration

import (
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddHabits20261022110000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)
	require.NoError(t, dropHabits20261022110000(x))

	require.NoError(t, addHabits20261022110000(x))
	// Re-running on existing tables must be harmless.
	require.NoError(t, addHabits20261022110000(x))

	habit := &habits20261022110000{UserID: 1, Title: "Read", Frequency: "daily", Weekdays: 127, Goal: 1}
	_, err = x.Insert(habit)
	require.NoError(t, err)

	day := time.Date(2026, 10, 22, 0, 0, 0, 0, time.UTC)
	_, err = x.Insert(&habitCheckins20261022110000{HabitID: habit.ID, Day: day, Value: 1})
	require.NoError(t, err)
	_, err = x.Insert(&habitCheckins20261022110000{HabitID: habit.ID, Day: day, Value: 2})
	require.Error(t, err, "a habit can only have one check-in per day")
	_, err = x.Insert(&habitCheckins20261022110000{HabitID: habit.ID, Day: day.AddDate(0, 0, 1), Value: 2})
	require.NoError(t, err)

	tables, err := x.DBMetas()
	require.NoError(t, err)
	var userIndexed bool
	for _, table := range tables {
		if table.Name != "habits" {
			continue
		}
		for _, index := range table.Indexes {
			if len(index.Cols) == 1 && index.Cols[0] == "user_id" {
				userIndexed = true
			}
		}
	}
	assert.True(t, userIndexed, "user_id must be indexed: every list query filters on it")

	require.NoError(t, dropHabits20261022110000(x))
	for _, name := range []string{"habits", "habit_checkins"} {
		exists, err := x.IsTableExist(name)
		require.NoError(t, err)
		assert.False(t, exists, name)
	}
}
