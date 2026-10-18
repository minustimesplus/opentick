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

type userBackgroundBefore20261018120000 struct {
	ID       int64  `xorm:"bigint autoincr not null unique pk"`
	Username string `xorm:"varchar(250) not null unique"`
	Email    string `xorm:"varchar(250) null"`
	// Plain indexes the migration must leave alone.
	DiscoverableByName  bool `xorm:"bool default false index"`
	DiscoverableByEmail bool `xorm:"bool default false index"`
}

func (userBackgroundBefore20261018120000) TableName() string {
	return "users"
}

func TestAddUserBackground20261018120000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)

	table := userBackgroundBefore20261018120000{}
	t.Cleanup(func() {
		require.NoError(t, x.DropTables(table))
	})
	require.NoError(t, x.DropTables(table))
	require.NoError(t, x.Sync2(table))

	_, err = x.Insert(&userBackgroundBefore20261018120000{ID: 1, Username: "user1"})
	require.NoError(t, err)

	before := usersTable20261018120000(t, x)
	require.NotEmpty(t, before.Indexes)
	require.NoError(t, addUserBackground20261018120000(x))
	// Re-running on a migrated table must be harmless.
	require.NoError(t, addUserBackground20261018120000(x))

	after := usersTable20261018120000(t, x)
	require.NotNil(t, after.GetColumn("background_file_id"))
	for _, column := range before.ColumnsSeq() {
		require.NotNilf(t, after.GetColumn(column), "migration dropped column %s", column)
	}
	for name, index := range before.Indexes {
		preserved, found := after.Indexes[name]
		require.Truef(t, found, "migration dropped index %s", name)
		require.Equal(t, index.Type, preserved.Type)
		require.Equal(t, index.Cols, preserved.Cols)
	}

	got := &users20261018120000{}
	found, err := x.Table("users").Where("id = ?", 1).Get(got)
	require.NoError(t, err)
	require.True(t, found)
	require.Zero(t, got.BackgroundFileID, "existing users start without a background")
}

func usersTable20261018120000(t *testing.T, x *xorm.Engine) *schemas.Table {
	t.Helper()
	tables, err := x.DBMetas()
	require.NoError(t, err)
	for _, table := range tables {
		if table.Name == "users" {
			return table
		}
	}
	t.Fatal("users table not found")
	return nil
}
