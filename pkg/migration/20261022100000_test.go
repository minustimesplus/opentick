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

func TestAddCountdowns20261022100000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)
	require.NoError(t, dropCountdowns20261022100000(x))

	require.NoError(t, addCountdowns20261022100000(x))
	// Re-running on an existing table must be harmless.
	require.NoError(t, addCountdowns20261022100000(x))

	exists, err := x.IsTableExist("countdowns")
	require.NoError(t, err)
	require.True(t, exists)

	date := time.Date(2027, time.February, 28, 0, 0, 0, 0, time.UTC)
	row := &countdowns20261022100000{UserID: 1, Title: "Trip", Date: date, Type: "birthday", RepeatYearly: true, Icon: "🎂", HexColor: "12b886"}
	_, err = x.Insert(row)
	require.NoError(t, err)

	got := &countdowns20261022100000{}
	has, err := x.ID(row.ID).Get(got)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, "🎂", got.Icon, "emoji icons must survive the round trip")
	assert.True(t, got.RepeatYearly)
	assert.Equal(t, date.Format(time.DateOnly), got.Date.UTC().Format(time.DateOnly))

	tables, err := x.DBMetas()
	require.NoError(t, err)
	var indexed bool
	for _, table := range tables {
		if table.Name != "countdowns" {
			continue
		}
		for _, index := range table.Indexes {
			if len(index.Cols) == 1 && index.Cols[0] == "user_id" {
				indexed = true
			}
		}
	}
	assert.True(t, indexed, "user_id must be indexed: every list query filters on it")

	require.NoError(t, dropCountdowns20261022100000(x))
	exists, err = x.IsTableExist("countdowns")
	require.NoError(t, err)
	assert.False(t, exists)
}
