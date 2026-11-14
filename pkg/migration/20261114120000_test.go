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

func TestAddContacts20261114120000(t *testing.T) {
	x, err := db.CreateTestEngine()
	require.NoError(t, err)
	require.NoError(t, dropContacts20261114120000(x))

	require.NoError(t, addContacts20261114120000(x))
	// Re-running on existing tables must be harmless.
	require.NoError(t, addContacts20261114120000(x))

	contact := &contacts20261114120000{
		UserID:       1,
		FullName:     "Ada Lovelace",
		Source:       "manual",
		LockedFields: []string{"company"},
	}
	_, err = x.Insert(contact)
	require.NoError(t, err)
	stored := &contacts20261114120000{}
	has, err := x.ID(contact.ID).Get(stored)
	require.NoError(t, err)
	require.True(t, has)
	assert.Equal(t, []string{"company"}, stored.LockedFields)

	_, err = x.Insert(&contactGroupMembers20261114120000{ContactID: contact.ID, GroupID: 1})
	require.NoError(t, err)
	_, err = x.Insert(&contactGroupMembers20261114120000{ContactID: contact.ID, GroupID: 1})
	require.Error(t, err, "a contact can only be in a group once")
	_, err = x.Insert(&contactTagLinks20261114120000{ContactID: contact.ID, TagID: 1})
	require.NoError(t, err)
	_, err = x.Insert(&contactTagLinks20261114120000{ContactID: contact.ID, TagID: 1})
	require.Error(t, err, "a contact can only have a tag once")

	tables, err := x.DBMetas()
	require.NoError(t, err)
	var userIndexed bool
	for _, table := range tables {
		if table.Name != "contacts" {
			continue
		}
		for _, index := range table.Indexes {
			if len(index.Cols) == 1 && index.Cols[0] == "user_id" {
				userIndexed = true
			}
		}
	}
	assert.True(t, userIndexed, "user_id must be indexed: every list query filters on it")

	require.NoError(t, dropContacts20261114120000(x))
	for _, name := range []string{"contacts", "contact_experiences", "contact_groups", "contact_tag_links", "contact_linkedin_status"} {
		exists, err := x.IsTableExist(name)
		require.NoError(t, err)
		assert.False(t, exists, name)
	}
}
