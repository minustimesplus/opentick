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
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures (pkg/db/fixtures/habits.yml, habit_checkins.yml): #1 and #2 belong
// to user1, #3 to user2. Habit #1 is daily with a goal of 8 and was done
// Oct 1-3 and 5-7 2026.
func TestHumaHabit(t *testing.T) {
	testHandler := webHandlerTestV2{
		user:     &testuser1,
		basePath: "/api/v2/habits",
		idParam:  "habit",
		t:        t,
	}

	t.Run("ReadAll", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/habits?today=2026-10-07", "")
		require.NoError(t, err)
		var body struct {
			Items []*models.Habit `json:"items"`
			Total int64           `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Items, 2)
		assert.EqualValues(t, 2, body.Total)
		assert.Equal(t, int64(1), body.Items[0].ID)
		assert.Equal(t, 8, body.Items[0].TodayValue)
		assert.Equal(t, 3, body.Items[0].CurrentStreak)
		assert.Len(t, body.Items[0].RecentCheckins, 7)
		assert.NotContains(t, rec.Body.String(), "Someone else's habit")
	})

	t.Run("ReadOne", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"habit": "1"})
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"title":"Drink water"`)
			assert.Contains(t, rec.Body.String(), `"goal":8`)
			assert.Contains(t, rec.Body.String(), `"max_permission":2`)
			assert.NotEmpty(t, rec.Result().Header.Get("ETag"))
		})
		t.Run("Nonexisting", func(t *testing.T) {
			_, err := testHandler.testReadOneWithUser(nil, map[string]string{"habit": "9999"})
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
		})
		t.Run("Forbidden - other user's habit", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"habit": "3"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			assert.NotContains(t, rec.Body.String(), "Someone else's habit")
		})
	})

	t.Run("Create", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testCreateWithUser(nil, nil, `{"title":"Meditate","icon":"🧘","frequency":"weekdays","weekdays":62,"goal":1}`)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.Contains(t, rec.Body.String(), `"weekdays":62`)
			db.AssertExists(t, "habits", map[string]any{"title": "Meditate", "user_id": 1, "frequency": "weekdays"}, false)
		})
		t.Run("Weekdays without a day", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"title":"Never","frequency":"weekdays","weekdays":0}`)
			require.Error(t, err)
			assertHandlerErrorCode(t, err, models.ErrCodeHabitInvalidWeekdays)
		})
		t.Run("Unknown frequency", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"title":"Hourly","frequency":"hourly"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusUnprocessableEntity, getHTTPErrorCode(err))
		})
		t.Run("Missing title", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"title":""}`)
			require.Error(t, err)
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testUpdateWithUser(nil, map[string]string{"habit": "1"}, `{"title":"Water","frequency":"daily","goal":6,"archived":true}`)
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"title":"Water"`)
			db.AssertExists(t, "habits", map[string]any{"id": 1, "title": "Water", "goal": 6, "archived": true}, false)
		})
		t.Run("Forbidden - other user's habit", func(t *testing.T) {
			_, err := testHandler.testUpdateWithUser(nil, map[string]string{"habit": "3"}, `{"title":"Mine now"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "habits", map[string]any{"id": 3, "title": "Someone else's habit"}, false)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("Forbidden - other user's habit", func(t *testing.T) {
			_, err := testHandler.testDeleteWithUser(nil, map[string]string{"habit": "3"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "habits", map[string]any{"id": 3}, false)
		})
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testDeleteWithUser(nil, map[string]string{"habit": "2"})
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, rec.Code)
			db.AssertMissing(t, "habits", map[string]any{"id": 2})
			db.AssertMissing(t, "habit_checkins", map[string]any{"habit_id": 2})
		})
	})
}

func TestHumaHabit_Checkins(t *testing.T) {
	testHandler := webHandlerTestV2{user: &testuser1, basePath: "/api/v2/habits", t: t}

	t.Run("List", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/habits/1/checkins?from=2026-10-03&to=2026-10-05", "")
		require.NoError(t, err)
		var checkins []*models.HabitCheckin
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &checkins))
		require.Len(t, checkins, 3)
		assert.Equal(t, "2026-10-03", checkins[0].Date)
		assert.Equal(t, 9, checkins[0].Value)
	})
	t.Run("List - invalid date", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, "/api/v2/habits/1/checkins?from=03.10.2026&to=2026-10-05", "")
		require.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeHabitInvalidDate)
	})
	t.Run("List - forbidden", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, "/api/v2/habits/3/checkins?from=2026-10-01&to=2026-10-31", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("Set", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodPut, "/api/v2/habits/1/checkins/2026-10-08?today=2026-10-08", `{"value":8}`)
		require.NoError(t, err)
		var h models.Habit
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &h))
		assert.Equal(t, 8, h.TodayValue)
		assert.Equal(t, 4, h.CurrentStreak)
		assert.Equal(t, []int{8}, habitCheckinValues(t, 1, "2026-10-08"))
	})
	t.Run("Set - forbidden", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodPut, "/api/v2/habits/3/checkins/2026-10-08", `{"value":1}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		assert.Empty(t, habitCheckinValues(t, 3, "2026-10-08"))
	})
	t.Run("Set - future day", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodPut, "/api/v2/habits/1/checkins/2999-01-01", `{"value":1}`)
		require.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeHabitCheckinInFuture)
	})
	t.Run("Delete", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodDelete, "/api/v2/habits/1/checkins/2026-10-07", "")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		db.AssertMissing(t, "habit_checkins", map[string]any{"id": 7})
	})
	t.Run("Delete - forbidden", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodDelete, "/api/v2/habits/3/checkins/2026-10-07", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
		db.AssertExists(t, "habit_checkins", map[string]any{"id": 11}, false)
	})
}

// habitCheckinValues returns the values checked in to a habit on a day. Rows
// inserted by a test get database-specific ids, so tests look them up by day.
func habitCheckinValues(t *testing.T, habitID int64, date string) []int {
	t.Helper()
	day, err := models.ParseHabitDate(date)
	require.NoError(t, err)
	s := db.NewSession()
	defer s.Close()
	checkins, err := (&models.Habit{ID: habitID}).GetCheckins(s, day, day)
	require.NoError(t, err)
	values := []int{}
	for _, c := range checkins {
		values = append(values, c.Value)
	}
	return values
}

func TestHumaHabit_Stats(t *testing.T) {
	testHandler := webHandlerTestV2{user: &testuser1, basePath: "/api/v2/habits", t: t}

	t.Run("Normal", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, "/api/v2/habits/1/stats?today=2026-10-07&month=2026-10", "")
		require.NoError(t, err)
		var st models.HabitStats
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &st))
		assert.Equal(t, 3, st.CurrentStreak)
		assert.Equal(t, 3, st.LongestStreak)
		assert.InDelta(t, 6.0/7, st.CompletionRate, 0.0001)
		assert.Len(t, st.Days, 31)
	})
	t.Run("Forbidden", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, "/api/v2/habits/3/stats", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
}

func TestHumaHabit_CheckinChangesETag(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	rec := humaRequest(t, e, http.MethodGet, "/api/v2/habits/1", "", token, "")
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	etag := rec.Header().Get("ETag")
	require.NotEmpty(t, etag)

	rec = humaRequest(t, e, http.MethodPut, "/api/v2/habits/1/checkins/2026-10-09", `{"value":2}`, token, "")
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	req := httptest.NewRequest(http.MethodGet, "/api/v2/habits/1", strings.NewReader(""))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("If-None-Match", etag)
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code, "a check-in must invalidate the habit's ETag")
}
