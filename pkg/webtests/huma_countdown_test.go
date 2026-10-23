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

// Fixtures (pkg/db/fixtures/countdowns.yml): #1 and #2 belong to user1, #3 to user2.
func TestHumaCountdown(t *testing.T) {
	testHandler := webHandlerTestV2{
		user:     &testuser1,
		basePath: "/api/v2/countdowns",
		idParam:  "countdown",
		t:        t,
	}

	t.Run("ReadAll", func(t *testing.T) {
		rec, err := testHandler.testReadAllWithUser(nil, nil)
		require.NoError(t, err)
		var body struct {
			Items []*models.Countdown `json:"items"`
			Total int64               `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Items, 2)
		assert.Equal(t, int64(2), body.Items[0].ID)
		assert.Equal(t, int64(1), body.Items[1].ID)
		assert.EqualValues(t, 2, body.Total)
		assert.NotContains(t, rec.Body.String(), "Someone else", "another user's countdown must not be listed")
	})

	t.Run("ReadOne", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"countdown": "2"})
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"title":"Anna's birthday"`)
			assert.Contains(t, rec.Body.String(), `"type":"birthday"`)
			assert.Contains(t, rec.Body.String(), `"repeat_yearly":true`)
			assert.Contains(t, rec.Body.String(), `"max_permission":2`)
			assert.NotEmpty(t, rec.Result().Header.Get("ETag"))
		})
		t.Run("Nonexisting", func(t *testing.T) {
			_, err := testHandler.testReadOneWithUser(nil, map[string]string{"countdown": "9999"})
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
		})
		t.Run("Forbidden - other user's countdown", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"countdown": "3"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			assert.NotContains(t, rec.Body.String(), "Someone else")
		})
	})

	t.Run("Create", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testCreateWithUser(nil, nil, `{"title":"Exam","date":"2027-01-10T15:00:00Z","icon":"📚","hex_color":"ff8800"}`)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.Contains(t, rec.Body.String(), `"date":"2027-01-10T00:00:00Z"`)
			assert.Contains(t, rec.Body.String(), `"type":"countdown"`)
			db.AssertExists(t, "countdowns", map[string]any{"title": "Exam", "user_id": 1, "icon": "📚"}, false)
		})
		t.Run("Invalid type", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"title":"Bad","date":"2027-01-10T00:00:00Z","type":"holiday"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusUnprocessableEntity, getHTTPErrorCode(err))
		})
		t.Run("Missing title", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"title":"","date":"2027-01-10T00:00:00Z"}`)
			require.Error(t, err)
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testUpdateWithUser(nil, map[string]string{"countdown": "1"}, `{"title":"Trip to Kyoto","date":"2027-05-02T00:00:00Z","type":"countdown"}`)
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"title":"Trip to Kyoto"`)
			db.AssertExists(t, "countdowns", map[string]any{"id": 1, "title": "Trip to Kyoto"}, false)
		})
		t.Run("Forbidden - other user's countdown", func(t *testing.T) {
			_, err := testHandler.testUpdateWithUser(nil, map[string]string{"countdown": "3"}, `{"title":"Mine now","date":"2027-05-02T00:00:00Z"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "countdowns", map[string]any{"id": 3, "title": "Someone else's anniversary"}, false)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("Forbidden - other user's countdown", func(t *testing.T) {
			_, err := testHandler.testDeleteWithUser(nil, map[string]string{"countdown": "3"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "countdowns", map[string]any{"id": 3}, false)
		})
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testDeleteWithUser(nil, map[string]string{"countdown": "2"})
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, rec.Code)
			db.AssertMissing(t, "countdowns", map[string]any{"id": 2})
		})
	})
}
