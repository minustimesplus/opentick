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
)

type habitsBefore20261114130000 struct {
	ID     int64  `xorm:"bigint autoincr not null unique pk"`
	UserID int64  `xorm:"bigint not null index"`
	Title  string `xorm:"varchar(250) not null"`
}

func (habitsBefore20261114130000) TableName() string {
	return "habits"
}

type habitsAfter20261114130000 struct {
	ID       int64   `xorm:"bigint autoincr not null unique pk"`
	UserID   int64   `xorm:"bigint not null index"`
	Title    string  `xorm:"varchar(250) not null"`
	Position float64 `xorm:"double not null default 0"`
}

func (habitsAfter20261114130000) TableName() string {
	return "habits"
}

func TestAddHabitPosition20261114130000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)

	before := habitsBefore20261114130000{}
	t.Cleanup(func() {
		require.NoError(t, x.DropTables(before))
	})
	require.NoError(t, x.DropTables(before))
	require.NoError(t, x.Sync2(before))
	_, err = x.Insert(&habitsBefore20261114130000{ID: 1, UserID: 1, Title: "a"}, &habitsBefore20261114130000{ID: 2, UserID: 1, Title: "b"})
	require.NoError(t, err)

	oldMetas, err := x.DBMetas()
	require.NoError(t, err)

	require.NoError(t, addHabitPosition20261114130000(x))
	// Running it twice must be harmless and keep the backfilled order.
	require.NoError(t, addHabitPosition20261114130000(x))

	newMetas, err := x.DBMetas()
	require.NoError(t, err)
	for _, oldTable := range oldMetas {
		if oldTable.Name != "habits" {
			continue
		}
		for _, newTable := range newMetas {
			if newTable.Name != "habits" {
				continue
			}
			assert.NotNil(t, newTable.GetColumn("position"))
			for name, index := range oldTable.Indexes {
				preserved, found := newTable.Indexes[name]
				require.Truef(t, found, "migration dropped index %s", name)
				assert.Equal(t, index.Type, preserved.Type)
			}
		}
	}

	rows := []habitsAfter20261114130000{}
	require.NoError(t, x.OrderBy("id").Find(&rows))
	require.Len(t, rows, 2)
	assert.InDelta(t, habitPositionStep20261114130000, rows[0].Position, 0)
	assert.InDelta(t, 2*habitPositionStep20261114130000, rows[1].Position, 0)
}
