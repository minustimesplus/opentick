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

// Fixtures (pkg/db/fixtures/weights.yml): user1 weighed in Oct 1-3 2026, user2 on Oct 2.
func TestHumaWeight(t *testing.T) {
	testHandler := webHandlerTestV2{
		user:     &testuser1,
		basePath: "/api/v2/weights",
		t:        t,
	}

	t.Run("List", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/weights?from=2026-10-01&to=2026-10-31", "")
		require.NoError(t, err)
		var body []*models.Weight
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body, 3, "only the user's own weights")
		assert.Equal(t, "2026-10-01", body[0].Date)
		assert.InDelta(t, 80.4, body[0].Weight, 0.001)
	})
	t.Run("List - invalid date", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, "/api/v2/weights?from=01.10.2026", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, getHTTPErrorCode(err))
	})
	t.Run("Set", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodPut, "/api/v2/weights/2026-10-04?today=2026-10-04", `{"weight":79.6}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"date":"2026-10-04"`)
		assert.Contains(t, rec.Body.String(), `"weight":79.6`)
		db.AssertExists(t, "weights", map[string]any{"user_id": 1, "weight": 79.6}, false)
	})
	t.Run("Set - out of range", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodPut, "/api/v2/weights/2026-10-04", `{"weight":5000}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusUnprocessableEntity, getHTTPErrorCode(err))
	})
	t.Run("Set - future", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodPut, "/api/v2/weights/2999-01-01", `{"weight":80}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, getHTTPErrorCode(err))
	})
	t.Run("Delete", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodDelete, "/api/v2/weights/2026-10-02", "")
		require.NoError(t, err)
		db.AssertMissing(t, "weights", map[string]any{"id": 2})
		db.AssertExists(t, "weights", map[string]any{"id": 4}, false)
	})
}
