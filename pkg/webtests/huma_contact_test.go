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

package webtests

import (
	"encoding/json"
	"net/http"
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures (pkg/db/fixtures/contacts.yml and contact_*.yml): contacts #1 Ada
// Lovelace (company locked, in group Pioneers), #2 Grace Hopper (tagged
// friends) and #3 Jean Dupont belong to user1. Contact #4, group #2 and tag
// #2 belong to user2.
func TestHumaContact(t *testing.T) {
	testHandler := webHandlerTestV2{
		user:     &testuser1,
		basePath: "/api/v2/contacts",
		idParam:  "contact",
		t:        t,
	}

	t.Run("ReadAll", func(t *testing.T) {
		rec, err := testHandler.testReadAllWithUser(nil, nil)
		require.NoError(t, err)
		var body struct {
			Items []*models.Contact `json:"items"`
			Total int64             `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Items, 3)
		assert.EqualValues(t, 3, body.Total)
		assert.Equal(t, "Ada Lovelace", body.Items[0].Name)
		assert.NotContains(t, rec.Body.String(), "Someone Else")
	})

	t.Run("Overview", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/contacts/overview", "")
		require.NoError(t, err)
		var body models.ContactOverview
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Contacts, 3)
		require.Len(t, body.Groups, 1)
		require.Len(t, body.Tags, 1)
		assert.EqualValues(t, 1, body.Groups[0].Members)
		assert.Equal(t, []string{"Yale University"}, body.Contacts[1].Schools)
		assert.Contains(t, body.Contacts[1].SearchText, "Remington Rand")
		assert.NotContains(t, rec.Body.String(), "Someone")
	})

	t.Run("ReadOne", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"contact": "1"})
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"full_name":"Ada Lovelace"`)
			assert.Contains(t, rec.Body.String(), `"groups":["Pioneers"]`)
			assert.Contains(t, rec.Body.String(), `"body":"Met at the Royal Society"`)
			assert.Contains(t, rec.Body.String(), `"max_permission":2`)
			assert.NotEmpty(t, rec.Result().Header.Get("ETag"))
		})
		t.Run("Nonexisting", func(t *testing.T) {
			_, err := testHandler.testReadOneWithUser(nil, map[string]string{"contact": "9999"})
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
		})
		t.Run("Forbidden - other user's contact", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"contact": "4"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			assert.NotContains(t, rec.Body.String(), "Someone Else")
		})
	})

	t.Run("Create", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testCreateWithUser(nil, nil, `{"full_name":"Alan Turing","company":"Bletchley Park"}`)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.Contains(t, rec.Body.String(), `"first_name":"Alan"`)
			db.AssertExists(t, "contacts", map[string]any{"full_name": "Alan Turing", "user_id": 1}, false)
		})
		t.Run("Duplicate", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"full_name":"Ada","email":"ada@example.com"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusConflict, getHTTPErrorCode(err))
			rec, err := testHandler.serve(http.MethodPost, "/api/v2/contacts?allow_duplicate=true", `{"full_name":"Ada","email":"ada@example.com"}`)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)
		})
		t.Run("Nothing to identify it by", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"company":"Nowhere"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusBadRequest, getHTTPErrorCode(err))
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testUpdateWithUser(nil, map[string]string{"contact": "3"}, `{"full_name":"Jean Dupont","email":"jean@example.com","location":"Paris, France","headline":"Étudiant à SciencesPo"}`)
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"locked_fields":["location"]`)
			db.AssertExists(t, "contacts", map[string]any{"id": 3, "location": "Paris, France"}, false)
		})
		t.Run("Forbidden - other user's contact", func(t *testing.T) {
			_, err := testHandler.testUpdateWithUser(nil, map[string]string{"contact": "4"}, `{"full_name":"Mine now"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "contacts", map[string]any{"id": 4, "full_name": "Someone Else"}, false)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("Forbidden - other user's contact", func(t *testing.T) {
			_, err := testHandler.testDeleteWithUser(nil, map[string]string{"contact": "4"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "contacts", map[string]any{"id": 4}, false)
		})
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testDeleteWithUser(nil, map[string]string{"contact": "3"})
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, rec.Code)
			db.AssertMissing(t, "contacts", map[string]any{"id": 3})
			db.AssertMissing(t, "contact_educations", map[string]any{"contact_id": 3})
		})
	})
}

func TestHumaContact_Actions(t *testing.T) {
	testHandler := webHandlerTestV2{
		user: &testuser1,
		t:    t,
	}

	t.Run("Search", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/search", `{"org":["sciences po"]}`)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"total":1`)
		assert.Contains(t, rec.Body.String(), `school: Sciences Po (Master), 2019–2021`)

		_, err = testHandler.serve(http.MethodPost, "/api/v2/contacts/search", `{"has":["shoe_size"]}`)
		require.Error(t, err)
	})

	t.Run("Distinct values", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/contacts/distinct-values?field=school", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"total_values":2`)
	})

	t.Run("Groups and tags", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/bulk/add-to-group", `{"group":"Pioneers","contact_ids":[2,4],"dry_run":true}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"changed":1`)
		assert.Contains(t, rec.Body.String(), `"unknown_ids":[4]`)
		db.AssertMissing(t, "contact_group_members", map[string]any{"contact_id": 2})

		_, err = testHandler.serve(http.MethodPost, "/api/v2/contacts/bulk/add-to-group", `{"group":"Someone else's group","contact_ids":[2]}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))

		_, err = testHandler.serve(http.MethodPost, "/api/v2/contacts/bulk/tag", `{"tag":"legends","contact_ids":[1]}`)
		require.NoError(t, err)
		db.AssertExists(t, "contact_tags", map[string]any{"name": "legends", "user_id": 1}, false)

		rec, err = testHandler.serve(http.MethodGet, "/api/v2/contact-groups", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"name":"Pioneers"`)
		assert.NotContains(t, rec.Body.String(), "Someone else")

		_, err = testHandler.serve(http.MethodGet, "/api/v2/contact-groups/2", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		_, err = testHandler.serve(http.MethodDelete, "/api/v2/contact-groups/2", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		_, err = testHandler.serve(http.MethodDelete, "/api/v2/contact-tags/2", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
	})

	t.Run("History and notes", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/2/educations", `{"school":"Vassar College","degree":"BA"}`)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		db.AssertExists(t, "contact_educations", map[string]any{"contact_id": 2, "school": "Vassar College"}, false)

		_, err = testHandler.serve(http.MethodPost, "/api/v2/contacts/4/notes", `{"body":"hello"}`)
		require.Error(t, err, "another user's contact")
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
		db.AssertMissing(t, "contact_notes", map[string]any{"body": "hello"})

		_, err = testHandler.serve(http.MethodPut, "/api/v2/contacts/1/notes/2", `{"body":"mine"}`)
		require.Error(t, err, "another contact's note")
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))

		_, err = testHandler.serve(http.MethodDelete, "/api/v2/contacts/1/notes/1", "")
		require.NoError(t, err)
		db.AssertMissing(t, "contact_notes", map[string]any{"id": 1})
	})

	t.Run("Photo", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, "/api/v2/contacts/1/photo", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
	})

	t.Run("Imports", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodPost, "/api/v2/contacts/import/linkedin-csv", `{"csv":"First Name,Last Name,URL\nKatherine,Johnson,https://www.linkedin.com/in/katherine\n"}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"created":1`)

		rec, err = testHandler.serve(http.MethodPost, "/api/v2/contacts/import/bundle", `{"contacts":[{"import_id":"abc","full_name":"Mary Jackson","groups":["NASA"]}]}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"created":1`)
		db.AssertExists(t, "contact_groups", map[string]any{"name": "NASA", "user_id": 1}, false)
	})

}
