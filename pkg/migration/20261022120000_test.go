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

func TestAddFocusSessions20261022120000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)
	require.NoError(t, dropFocusSessions20261022120000(x))

	require.NoError(t, addFocusSessions20261022120000(x))
	// Re-running on an existing table must be harmless.
	require.NoError(t, addFocusSessions20261022120000(x))

	start := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)
	row := &focusSessions20261022120000{UserID: 1, Type: "pomo", StartTime: start, EndTime: start.Add(25 * time.Minute), Duration: 1500}
	_, err = x.Insert(row)
	require.NoError(t, err)

	got := &focusSessions20261022120000{}
	has, err := x.ID(row.ID).Get(got)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, int64(1500), got.Duration)
	assert.Equal(t, int64(0), got.TaskID, "task_id is optional")

	tables, err := x.DBMetas()
	require.NoError(t, err)
	indexed := map[string]bool{}
	for _, table := range tables {
		if table.Name != "focus_sessions" {
			continue
		}
		for _, index := range table.Indexes {
			if len(index.Cols) == 1 {
				indexed[index.Cols[0]] = true
			}
		}
	}
	assert.True(t, indexed["user_id"], "user_id must be indexed: every query filters on it")
	assert.True(t, indexed["start_time"], "start_time must be indexed: the stats and range queries filter on it")

	require.NoError(t, dropFocusSessions20261022120000(x))
	exists, err := x.IsTableExist("focus_sessions")
	require.NoError(t, err)
	assert.False(t, exists)
}
