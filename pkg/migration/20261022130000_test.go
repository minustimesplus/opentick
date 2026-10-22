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

	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type projectsBefore20261022130000 struct {
	ID         int64  `xorm:"bigint autoincr not null unique pk"`
	Title      string `xorm:"varchar(250) not null"`
	Identifier string `xorm:"varchar(10) null unique"`
	OwnerID    int64  `xorm:"bigint INDEX not null"`
}

func (projectsBefore20261022130000) TableName() string {
	return "projects"
}

type projectsAfter20261022130000 struct {
	ID               int64     `xorm:"bigint autoincr not null unique pk"`
	Title            string    `xorm:"varchar(250) not null"`
	TasksPurgedUntil time.Time `xorm:"datetime null 'tasks_purged_until'"`
}

func (projectsAfter20261022130000) TableName() string {
	return "projects"
}

func TestAddTasksPurgedUntil20261022130000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)

	table := projectsBefore20261022130000{}
	require.NoError(t, x.DropTables(table))
	t.Cleanup(func() {
		require.NoError(t, x.DropTables(table))
	})
	require.NoError(t, x.Sync2(table))

	_, err = x.Insert(&projectsBefore20261022130000{ID: 1, Title: "existing", Identifier: "EX", OwnerID: 1})
	require.NoError(t, err)

	before := projectsTable20261022130000(t, x)
	require.NotEmpty(t, before.Indexes)
	require.NoError(t, addTasksPurgedUntil20261022130000(x))

	after := projectsTable20261022130000(t, x)
	require.NotNil(t, after.GetColumn("tasks_purged_until"))
	for _, column := range before.ColumnsSeq() {
		require.NotNilf(t, after.GetColumn(column), "migration dropped column %s", column)
	}
	for name, index := range before.Indexes {
		preserved, found := after.Indexes[name]
		require.Truef(t, found, "migration dropped index %s", name)
		require.Equal(t, index.Type, preserved.Type)
		require.Equal(t, index.Cols, preserved.Cols)
	}

	got := &projectsAfter20261022130000{}
	found, err := x.ID(1).Get(got)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "existing", got.Title)
	require.True(t, got.TasksPurgedUntil.IsZero())
}

func projectsTable20261022130000(t *testing.T, x *xorm.Engine) *schemas.Table {
	t.Helper()
	tables, err := x.DBMetas()
	require.NoError(t, err)
	for _, table := range tables {
		if table.Name == "projects" {
			return table
		}
	}
	t.Fatal("projects table not found")
	return nil
}
