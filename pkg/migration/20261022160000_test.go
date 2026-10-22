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

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type taskWontDoBefore20261022160000 struct {
	ID        int64  `xorm:"bigint autoincr not null unique pk"`
	Title     string `xorm:"TEXT not null"`
	ProjectID int64  `xorm:"bigint INDEX not null index(project_done_due_date)"`
	Done      bool   `xorm:"INDEX null index(project_done_due_date)"`
}

func (taskWontDoBefore20261022160000) TableName() string {
	return "tasks"
}

func TestAddTaskWontDo20261022160000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)

	table := taskWontDoBefore20261022160000{}
	t.Cleanup(func() {
		require.NoError(t, x.DropTables(table))
	})
	require.NoError(t, x.DropTables(table))
	require.NoError(t, x.Sync2(table))

	_, err = x.Insert(&taskWontDoBefore20261022160000{ID: 1, Title: "done task", ProjectID: 1, Done: true})
	require.NoError(t, err)

	before := tasksTable20261022160000(t, x)
	require.NotEmpty(t, before.Indexes)
	require.NoError(t, addTaskWontDo20261022160000(x))
	// Re-running on a migrated table must be harmless.
	require.NoError(t, addTaskWontDo20261022160000(x))

	after := tasksTable20261022160000(t, x)
	require.NotNil(t, after.GetColumn("wont_do"))
	for _, column := range before.ColumnsSeq() {
		require.NotNilf(t, after.GetColumn(column), "migration dropped column %s", column)
	}
	for name, index := range before.Indexes {
		preserved, found := after.Indexes[name]
		require.Truef(t, found, "migration dropped index %s", name)
		require.Equal(t, index.Type, preserved.Type)
		require.Equal(t, index.Cols, preserved.Cols)
	}

	got := &struct {
		Done   bool `xorm:"done"`
		WontDo bool `xorm:"wont_do"`
	}{}
	found, err := x.Table("tasks").Where("id = ?", 1).Get(got)
	require.NoError(t, err)
	require.True(t, found)
	require.True(t, got.Done)
	require.False(t, got.WontDo, "existing done tasks stay completed, not won't do")
}

func tasksTable20261022160000(t *testing.T, x *xorm.Engine) *schemas.Table {
	t.Helper()
	tables, err := x.DBMetas()
	require.NoError(t, err)
	for _, table := range tables {
		if table.Name == "tasks" {
			return table
		}
	}
	t.Fatal("tasks table not found")
	return nil
}
