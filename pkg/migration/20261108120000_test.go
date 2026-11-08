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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
	"xorm.io/xorm/schemas"
)

type usersBefore20261108120000 struct {
	ID       int64  `xorm:"bigint autoincr not null unique pk"`
	Username string `xorm:"varchar(250) not null unique"`
	Email    string `xorm:"varchar(250) null INDEX"`
}

func (usersBefore20261108120000) TableName() string {
	return "users"
}

type usersAfter20261108120000 struct {
	ID                    int64  `xorm:"bigint autoincr not null unique pk"`
	Username              string `xorm:"varchar(250) not null unique"`
	ConfirmProjectInvites bool   `xorm:"bool not null default false"`
	NotifySharedTaskDone  bool   `xorm:"bool not null default false"`
}

func (usersAfter20261108120000) TableName() string {
	return "users"
}

func usersTable20261108120000(t *testing.T, x *xorm.Engine) *schemas.Table {
	tables, err := x.DBMetas()
	require.NoError(t, err)
	for _, table := range tables {
		if table.Name == "users" {
			return table
		}
	}
	t.Fatal("users table missing")
	return nil
}

func TestAddCollaboration20261108120000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)

	before := usersBefore20261108120000{}
	t.Cleanup(func() {
		require.NoError(t, x.DropTables(before, projectInvitations20261108120000{}))
	})
	require.NoError(t, x.DropTables(before, projectInvitations20261108120000{}))
	require.NoError(t, x.Sync2(before))
	_, err = x.Insert(&usersBefore20261108120000{ID: 1, Username: "existing", Email: "a@example.com"})
	require.NoError(t, err)

	old := usersTable20261108120000(t, x)
	require.NoError(t, addCollaboration20261108120000(x))
	// Running it twice must be harmless.
	require.NoError(t, addCollaboration20261108120000(x))

	after := usersTable20261108120000(t, x)
	for _, col := range usersCollaborationColumns20261108120000 {
		assert.NotNilf(t, after.GetColumn(col), "column %s missing", col)
	}
	for name, index := range old.Indexes {
		preserved, found := after.Indexes[name]
		require.Truef(t, found, "migration dropped index %s", name)
		assert.Equal(t, index.Type, preserved.Type)
	}

	got := &usersAfter20261108120000{}
	has, err := x.ID(1).Get(got)
	require.NoError(t, err)
	require.True(t, has)
	assert.False(t, got.ConfirmProjectInvites, "existing users keep joining shared projects right away")
	assert.False(t, got.NotifySharedTaskDone)

	exists, err := x.IsTableExist("project_invitations")
	require.NoError(t, err)
	assert.True(t, exists)
}
