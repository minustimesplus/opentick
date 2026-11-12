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

func TestAddInboundMail20261112120000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)

	before := agentConfigs20261110120000{}
	t.Cleanup(func() {
		require.NoError(t, x.DropTables(before, inboundMailMessages20261112120000{}))
	})
	require.NoError(t, x.DropTables(before, inboundMailMessages20261112120000{}))
	require.NoError(t, x.Sync2(before))
	_, err = x.Insert(&agentConfigs20261110120000{ID: 1, UserID: 7, InstinctEmail: "bot@example.com"})
	require.NoError(t, err)

	tableOf := func() map[string]bool {
		tables, err := x.DBMetas()
		require.NoError(t, err)
		for _, table := range tables {
			if table.Name == "agent_configs" {
				uniques := map[string]bool{}
				for _, col := range table.Columns() {
					uniques["col:"+col.Name] = true
				}
				for name, index := range table.Indexes {
					uniques[name] = index.Type == 2 // schemas.UniqueType
				}
				return uniques
			}
		}
		t.Fatal("agent_configs table missing")
		return nil
	}
	old := tableOf()
	require.NoError(t, addInboundMail20261112120000(x))
	// Running it twice must be harmless.
	require.NoError(t, addInboundMail20261112120000(x))
	after := tableOf()

	for name, unique := range old {
		got, found := after[name]
		require.Truef(t, found, "migration dropped %s", name)
		assert.Equalf(t, unique, got, "%s changed uniqueness", name)
	}
	for _, col := range []string{"inbound_secret", "inbound_senders", "inbound_project_id"} {
		assert.Truef(t, after["col:"+col], "column %s missing", col)
	}

	row := map[string]any{}
	has, err := x.Table("agent_configs").Where("id = 1").Get(&row)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, "bot@example.com", string(asBytes(row["instinct_email"])))

	exists, err := x.IsTableExist("inbound_mail_messages")
	require.NoError(t, err)
	assert.True(t, exists)
}

func asBytes(v any) []byte {
	switch x := v.(type) {
	case []byte:
		return x
	case string:
		return []byte(x)
	}
	return nil
}
