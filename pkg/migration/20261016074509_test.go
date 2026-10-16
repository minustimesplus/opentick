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

type calendarSubscriptionRow20261016074509 struct {
	ID       int64     `xorm:"bigint autoincr not null unique pk"`
	UserID   int64     `xorm:"bigint not null index"`
	Title    string    `xorm:"varchar(250) not null"`
	URL      string    `xorm:"text not null"`
	HexColor string    `xorm:"varchar(6) null"`
	Created  time.Time `xorm:"created not null"`
	Updated  time.Time `xorm:"updated not null"`
}

func (calendarSubscriptionRow20261016074509) TableName() string {
	return "calendar_subscriptions"
}

func TestAddCalendarSubscriptions20261016074509(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)
	require.NoError(t, dropCalendarSubscriptions20261016074509(x))

	require.NoError(t, addCalendarSubscriptions20261016074509(x))
	// Re-running on an existing table must be harmless.
	require.NoError(t, addCalendarSubscriptions20261016074509(x))

	exists, err := x.IsTableExist("calendar_subscriptions")
	require.NoError(t, err)
	require.True(t, exists)

	long := "https://calendar.example.com/" + "feeds/private-0123456789abcdef0123456789abcdef/basic.ics?token=" +
		"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	row := &calendarSubscriptionRow20261016074509{UserID: 1, Title: "Work", URL: long, HexColor: "12b886"}
	_, err = x.Insert(row)
	require.NoError(t, err)

	got := &calendarSubscriptionRow20261016074509{}
	has, err := x.ID(row.ID).Get(got)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, long, got.URL, "urls longer than 255 characters must be stored intact")

	tables, err := x.DBMetas()
	require.NoError(t, err)
	var indexed bool
	for _, table := range tables {
		if table.Name != "calendar_subscriptions" {
			continue
		}
		for _, index := range table.Indexes {
			if len(index.Cols) == 1 && index.Cols[0] == "user_id" {
				indexed = true
			}
		}
	}
	assert.True(t, indexed, "user_id must be indexed: every list query filters on it")

	require.NoError(t, dropCalendarSubscriptions20261016074509(x))
	exists, err = x.IsTableExist("calendar_subscriptions")
	require.NoError(t, err)
	assert.False(t, exists)
}
