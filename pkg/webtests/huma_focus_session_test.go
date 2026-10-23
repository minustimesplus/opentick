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

// Fixtures (pkg/db/fixtures/focus_sessions.yml): #1 (pomo on task 1) and #2
// (stopwatch) belong to user1, #3 to user2. Task 13 is in a project user1
// cannot access.
func TestHumaFocusSession(t *testing.T) {
	testHandler := webHandlerTestV2{
		user:     &testuser1,
		basePath: "/api/v2/focus-sessions",
		idParam:  "focussession",
		t:        t,
	}

	t.Run("ReadAll", func(t *testing.T) {
		rec, err := testHandler.testReadAllWithUser(nil, nil)
		require.NoError(t, err)
		var body struct {
			Items []*models.FocusSession `json:"items"`
			Total int64                  `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Items, 2)
		assert.Equal(t, int64(2), body.Items[0].ID, "newest first")
		assert.Equal(t, int64(1), body.Items[1].ID)
		assert.EqualValues(t, 2, body.Total)
		assert.NotContains(t, rec.Body.String(), "secret focus", "another user's session must not be listed")
	})
	t.Run("ReadAll with range", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/focus-sessions?from=2018-12-01T09:30:00Z", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"total":1`)
		assert.Contains(t, rec.Body.String(), `"type":"stopwatch"`)
	})

	t.Run("ReadOne", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"focussession": "1"})
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"type":"pomo"`)
			assert.Contains(t, rec.Body.String(), `"duration":1500`)
			assert.Contains(t, rec.Body.String(), `"max_permission":2`)
			assert.NotEmpty(t, rec.Result().Header.Get("ETag"))
		})
		t.Run("Nonexisting", func(t *testing.T) {
			_, err := testHandler.testReadOneWithUser(nil, map[string]string{"focussession": "9999"})
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
		})
		t.Run("Forbidden - other user's session", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"focussession": "3"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			assert.NotContains(t, rec.Body.String(), "secret focus")
		})
	})

	t.Run("Create", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testCreateWithUser(nil, nil, `{"type":"pomo","task_id":1,"start_time":"2026-10-22T09:00:00Z","end_time":"2026-10-22T09:25:00Z"}`)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.Contains(t, rec.Body.String(), `"duration":1500`)
			db.AssertExists(t, "focus_sessions", map[string]any{"task_id": 1, "user_id": 1, "duration": 1500}, false)
		})
		t.Run("Invalid type", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"type":"nap","start_time":"2026-10-22T09:00:00Z"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusUnprocessableEntity, getHTTPErrorCode(err))
		})
		t.Run("End before start", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"type":"pomo","start_time":"2026-10-22T09:00:00Z","end_time":"2026-10-22T08:00:00Z"}`)
			require.Error(t, err)
			assertHandlerErrorCode(t, err, models.ErrCodeFocusSessionInvalidTimes)
		})
		t.Run("Forbidden - task without access", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"type":"pomo","task_id":13,"start_time":"2026-10-22T09:00:00Z","end_time":"2026-10-22T09:25:00Z"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testUpdateWithUser(nil, map[string]string{"focussession": "2"}, `{"type":"stopwatch","start_time":"2018-12-01T10:00:00Z","end_time":"2018-12-01T11:00:00Z","note":"edited"}`)
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"note":"edited"`)
			db.AssertExists(t, "focus_sessions", map[string]any{"id": 2, "note": "edited", "duration": 3600}, false)
		})
		t.Run("Forbidden - other user's session", func(t *testing.T) {
			_, err := testHandler.testUpdateWithUser(nil, map[string]string{"focussession": "3"}, `{"type":"pomo","start_time":"2018-12-01T09:00:00Z","note":"mine now"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "focus_sessions", map[string]any{"id": 3, "note": "Someone else's secret focus"}, false)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("Forbidden - other user's session", func(t *testing.T) {
			_, err := testHandler.testDeleteWithUser(nil, map[string]string{"focussession": "3"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "focus_sessions", map[string]any{"id": 3}, false)
		})
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testDeleteWithUser(nil, map[string]string{"focussession": "1"})
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, rec.Code)
			db.AssertMissing(t, "focus_sessions", map[string]any{"id": 1})
		})
	})
}

func TestHumaFocusSession_Stats(t *testing.T) {
	testHandler := webHandlerTestV2{user: &testuser1, basePath: "/api/v2/focus-sessions", t: t}

	t.Run("Normal", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/focus-sessions/stats?days=7&timezone=Europe/Berlin", "")
		require.NoError(t, err)
		var stats models.FocusStats
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
		assert.Equal(t, int64(1), stats.TotalPomos)
		assert.Equal(t, int64(70), stats.TotalFocusMinutes)
		assert.Len(t, stats.History, 7)
	})
	t.Run("Default days", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/focus-sessions/stats", "")
		require.NoError(t, err)
		var stats models.FocusStats
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
		assert.Len(t, stats.History, 30)
	})
	t.Run("Unknown timezone", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, "/api/v2/focus-sessions/stats?timezone=Mars/Olympus", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, getHTTPErrorCode(err))
	})
	t.Run("Too many days", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, "/api/v2/focus-sessions/stats?days=1000", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusUnprocessableEntity, getHTTPErrorCode(err))
	})
}
