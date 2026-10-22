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
	"time"

	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHumaUserStatistics(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	t.Run("Defaults", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/user/statistics", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		var stats models.UserStatistics
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
		assert.Len(t, stats.Daily, 30)
		assert.Len(t, stats.Weekly, 12)
		assert.Len(t, stats.Monthly, 12)
		assert.Positive(t, stats.TotalCompleted)
		assert.NotEmpty(t, stats.Projects)
		assert.GreaterOrEqual(t, stats.Score.Level, int64(1))
		assert.Greater(t, stats.Score.NextLevelScore, stats.Score.Score)
	})
	t.Run("Custom periods", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/user/statistics?days=7&weeks=4&months=3", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		var stats models.UserStatistics
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
		assert.Len(t, stats.Daily, 7)
		assert.Len(t, stats.Weekly, 4)
		assert.Len(t, stats.Monthly, 3)
	})
	t.Run("Time zone and week start", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/user/statistics?timezone=Asia/Tokyo&week_start=0", "", token, "")
		require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

		var stats models.UserStatistics
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &stats))
		assert.Equal(t, "Asia/Tokyo", stats.Timezone)
		weekStart, err := time.Parse(time.DateOnly, stats.Weekly[len(stats.Weekly)-1].Date)
		require.NoError(t, err)
		assert.Equal(t, time.Sunday, weekStart.Weekday())
	})
	t.Run("Unknown time zone", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/user/statistics?timezone=Mars/Olympus", "", token, "")
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
	t.Run("Out of range", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/user/statistics?days=1000", "", token, "")
		assert.Equal(t, http.StatusUnprocessableEntity, rec.Code)
	})
	t.Run("Unauthenticated", func(t *testing.T) {
		rec := humaRequest(t, e, http.MethodGet, "/api/v2/user/statistics", "", "", "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
}
