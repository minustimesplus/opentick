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
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func trashItemIDs(t *testing.T, rec *httptest.ResponseRecorder) []int64 {
	t.Helper()
	ids := []int64{}
	for _, raw := range decodePaginatedTaskItems(t, rec) {
		var item struct {
			ID        int64  `json:"id"`
			DeletedAt string `json:"deleted_at"`
		}
		require.NoError(t, json.Unmarshal(raw, &item))
		assert.NotEmpty(t, item.DeletedAt, "trashed task %d has no deleted_at", item.ID)
		ids = append(ids, item.ID)
	}
	return ids
}

func TestHumaTaskTrash(t *testing.T) {
	h := webHandlerTestV2{user: &testuser1, t: t}
	require.NoError(t, h.ensureEnv())
	user1 := humaTokenFor(t, &testuser1)
	user2 := humaTokenFor(t, &testuser2)

	req := func(method, path, token string) *httptest.ResponseRecorder {
		return humaRequest(t, h.e, method, "/api/v2"+path, "", token, "")
	}

	t.Run("List", func(t *testing.T) {
		rec := req(http.MethodGet, "/tasks/trash", user1)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Equal(t, []int64{51}, trashItemIDs(t, rec))

		rec = req(http.MethodGet, "/tasks/trash", user2)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Empty(t, trashItemIDs(t, rec))

		rec = req(http.MethodGet, "/tasks/trash", "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Delete and restore", func(t *testing.T) {
		rec := req(http.MethodDelete, "/tasks/1", user1)
		require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
		assert.Equal(t, http.StatusNotFound, req(http.MethodGet, "/tasks/1", user1).Code)

		rec = req(http.MethodGet, "/tasks/trash", user1)
		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, []int64{1, 51}, trashItemIDs(t, rec))

		rec = req(http.MethodPost, "/tasks/trash/1/restore", user1)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"id":1,`)
		assert.NotContains(t, rec.Body.String(), `"deleted_at"`)

		assert.Equal(t, http.StatusOK, req(http.MethodGet, "/tasks/1", user1).Code)
		rec = req(http.MethodGet, "/tasks/trash", user1)
		assert.Equal(t, []int64{51}, trashItemIDs(t, rec))
	})

	t.Run("Restore errors", func(t *testing.T) {
		assert.Equal(t, http.StatusNotFound, req(http.MethodPost, "/tasks/trash/2/restore", user1).Code, "not trashed")
		assert.Equal(t, http.StatusNotFound, req(http.MethodPost, "/tasks/trash/99999/restore", user1).Code)
		assert.Equal(t, http.StatusForbidden, req(http.MethodPost, "/tasks/trash/51/restore", user2).Code)
	})

	t.Run("Delete forever", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, req(http.MethodDelete, "/tasks/trash/51", user2).Code)
		assert.Equal(t, http.StatusNotFound, req(http.MethodDelete, "/tasks/trash/2", user1).Code, "not trashed")

		rec := req(http.MethodDelete, "/tasks/trash/51", user1)
		require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
		assert.Equal(t, http.StatusNotFound, req(http.MethodPost, "/tasks/trash/51/restore", user1).Code)
		assert.Empty(t, trashItemIDs(t, req(http.MethodGet, "/tasks/trash", user1)))
	})

	t.Run("Empty", func(t *testing.T) {
		require.Equal(t, http.StatusNoContent, req(http.MethodDelete, "/tasks/1", user1).Code)
		require.Equal(t, http.StatusNoContent, req(http.MethodDelete, "/tasks/2", user1).Code)

		rec := req(http.MethodPost, "/tasks/trash/empty", user1)
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		var result struct {
			Deleted int `json:"deleted"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
		assert.Equal(t, 2, result.Deleted)
		assert.Empty(t, trashItemIDs(t, req(http.MethodGet, "/tasks/trash", user1)))
		assert.Equal(t, http.StatusNotFound, req(http.MethodPost, "/tasks/trash/1/restore", user1).Code)
	})
}
