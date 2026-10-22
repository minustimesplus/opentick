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
package models

import (
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures (pkg/db/fixtures/focus_sessions.yml): #1 (pomo, task 1) and #2
// (stopwatch) belong to user1, #3 to user2. Task 13 is in project 2, which
// user1 cannot access.

func TestFocusSession_Create(t *testing.T) {
	u := &user.User{ID: 1}
	start := time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC)

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		fs := &FocusSession{TaskID: 1, Type: FocusSessionTypePomo, StartTime: start, EndTime: start.Add(30 * time.Minute), Duration: 1500, UserID: 99}
		can, err := fs.CanCreate(s, u)
		require.NoError(t, err)
		assert.True(t, can)
		require.NoError(t, fs.Create(s, u))
		require.NoError(t, s.Commit())

		assert.Equal(t, int64(1), fs.UserID, "the owner comes from the auth, not the body")
		db.AssertExists(t, "focus_sessions", map[string]any{
			"id":       fs.ID,
			"user_id":  1,
			"task_id":  1,
			"type":     "pomo",
			"duration": 1500,
		}, false)
	})
	t.Run("duration defaults to the span", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		fs := &FocusSession{Type: FocusSessionTypeStopwatch, StartTime: start, EndTime: start.Add(42 * time.Minute)}
		require.NoError(t, fs.Create(s, u))
		assert.Equal(t, int64(42*60), fs.Duration)
	})
	t.Run("end defaults to now", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		fs := &FocusSession{Type: FocusSessionTypeStopwatch, StartTime: time.Now().Add(-time.Minute)}
		require.NoError(t, fs.Create(s, u))
		assert.False(t, fs.EndTime.IsZero())
		assert.InDelta(t, 60, fs.Duration, 2)
	})
	t.Run("invalid type", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&FocusSession{Type: "nap", StartTime: start, EndTime: start}).Create(s, u)
		require.Error(t, err)
		assert.True(t, IsErrFocusSessionInvalidType(err))
	})
	t.Run("invalid times", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		for name, fs := range map[string]*FocusSession{
			"no start":              {Type: FocusSessionTypePomo},
			"end before start":      {Type: FocusSessionTypePomo, StartTime: start, EndTime: start.Add(-time.Minute)},
			"duration exceeds span": {Type: FocusSessionTypePomo, StartTime: start, EndTime: start.Add(time.Minute), Duration: 61},
			"negative duration":     {Type: FocusSessionTypePomo, StartTime: start, EndTime: start.Add(time.Minute), Duration: -1},
		} {
			err := fs.Create(s, u)
			require.Error(t, err, name)
			assert.True(t, IsErrFocusSessionInvalidTimes(err), name)
		}
	})
	t.Run("task without access", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&FocusSession{TaskID: 13}).CanCreate(s, u)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("nonexistent task", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := (&FocusSession{TaskID: 99999}).CanCreate(s, u)
		require.Error(t, err)
		assert.True(t, IsErrTaskDoesNotExist(err))
	})
	t.Run("link share cannot create", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&FocusSession{}).CanCreate(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
}

func TestFocusSession_ReadAll(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("only own sessions, newest first", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, count, total, err := (&FocusSession{}).ReadAll(s, u, "", 0, 50)
		require.NoError(t, err)
		sessions := result.([]*FocusSession)
		assert.Equal(t, 2, count)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, int64(2), sessions[0].ID)
		assert.Equal(t, int64(1), sessions[1].ID)
	})
	t.Run("range and task filter", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		from := time.Date(2018, 12, 1, 9, 30, 0, 0, time.UTC)
		result, _, _, err := (&FocusSession{From: &from}).ReadAll(s, u, "", 0, 50)
		require.NoError(t, err)
		sessions := result.([]*FocusSession)
		require.Len(t, sessions, 1)
		assert.Equal(t, int64(2), sessions[0].ID)

		result, _, _, err = (&FocusSession{To: &from}).ReadAll(s, u, "", 0, 50)
		require.NoError(t, err)
		require.Len(t, result.([]*FocusSession), 1)
		assert.Equal(t, int64(1), result.([]*FocusSession)[0].ID)

		result, _, _, err = (&FocusSession{TaskID: 1}).ReadAll(s, u, "", 0, 50)
		require.NoError(t, err)
		require.Len(t, result.([]*FocusSession), 1)
	})
	t.Run("search", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&FocusSession{}).ReadAll(s, u, "outline", 0, 50)
		require.NoError(t, err)
		require.Len(t, result.([]*FocusSession), 1)
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, _, err := (&FocusSession{}).ReadAll(s, &LinkSharing{ID: 1}, "", 0, 50)
		require.Error(t, err)
	})
}

func TestFocusSession_Permissions(t *testing.T) {
	owner := &user.User{ID: 1}
	other := &user.User{ID: 2}

	t.Run("owner can read, update and delete", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		fs := &FocusSession{ID: 1}
		can, maxPerm, err := fs.CanRead(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
		assert.Equal(t, int(PermissionAdmin), maxPerm)
		assert.Equal(t, "Wrote the outline", fs.Note, "CanRead loads the session")

		can, err = (&FocusSession{ID: 1, TaskID: 1}).CanUpdate(s, owner)
		require.NoError(t, err)
		assert.True(t, can)

		can, err = (&FocusSession{ID: 1}).CanDelete(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
	})
	t.Run("other user is denied", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, _, err := (&FocusSession{ID: 1}).CanRead(s, other)
		require.NoError(t, err)
		assert.False(t, can)

		can, err = (&FocusSession{ID: 1}).CanUpdate(s, other)
		require.NoError(t, err)
		assert.False(t, can)

		can, err = (&FocusSession{ID: 1}).CanDelete(s, other)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("link share is denied", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, _, err := (&FocusSession{ID: 1}).CanRead(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("relinking to an inaccessible task is denied", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&FocusSession{ID: 1, TaskID: 13}).CanUpdate(s, owner)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("nonexistent", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, err := (&FocusSession{ID: 9999}).CanRead(s, owner)
		require.Error(t, err)
		assert.True(t, IsErrFocusSessionDoesNotExist(err))
	})
}

func TestFocusSession_Update(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	u := &user.User{ID: 1}
	fs := &FocusSession{
		ID:        2,
		Type:      FocusSessionTypeStopwatch,
		StartTime: time.Date(2018, 12, 1, 10, 0, 0, 0, time.UTC),
		EndTime:   time.Date(2018, 12, 1, 11, 0, 0, 0, time.UTC),
		Duration:  3000,
		Note:      "edited",
	}
	can, err := fs.CanUpdate(s, u)
	require.NoError(t, err)
	require.True(t, can)
	require.NoError(t, fs.Update(s, u))
	require.NoError(t, s.Commit())

	assert.Equal(t, "edited", fs.Note)
	assert.False(t, fs.Created.IsZero(), "update reloads the stored row")
	db.AssertExists(t, "focus_sessions", map[string]any{"id": 2, "duration": 3000, "note": "edited", "user_id": 1}, false)
}

func TestFocusSession_Delete(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	require.NoError(t, (&FocusSession{ID: 1}).Delete(s, &user.User{ID: 1}))
	require.NoError(t, s.Commit())
	db.AssertMissing(t, "focus_sessions", map[string]any{"id": 1})
}

func TestGetFocusStats(t *testing.T) {
	u := &user.User{ID: 1}
	now := time.Date(2018, 12, 1, 23, 0, 0, 0, time.UTC)

	t.Run("totals and history", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		stats, err := GetFocusStats(s, u, time.UTC, 3, now)
		require.NoError(t, err)
		assert.Equal(t, int64(1), stats.TotalPomos)
		assert.Equal(t, int64(70), stats.TotalFocusMinutes)
		assert.Equal(t, int64(1), stats.TodayPomos)
		assert.Equal(t, int64(70), stats.TodayFocusMinutes)
		require.Len(t, stats.History, 3)
		assert.Equal(t, "2018-11-29", stats.History[0].Date)
		assert.Equal(t, int64(0), stats.History[0].FocusMinutes)
		assert.Equal(t, "2018-12-01", stats.History[2].Date)
		assert.Equal(t, int64(70), stats.History[2].FocusMinutes)
	})
	t.Run("days are cut in the requested timezone", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// In Kiritimati (UTC+14) the 09:00 UTC pomo is still 2018-12-01, but the
		// 10:00 UTC stopwatch session falls on 2018-12-02, which is "today".
		loc, err := time.LoadLocation("Pacific/Kiritimati")
		require.NoError(t, err)
		stats, err := GetFocusStats(s, u, loc, 2, now)
		require.NoError(t, err)
		assert.Equal(t, "2018-12-01", stats.History[0].Date)
		assert.Equal(t, int64(1), stats.History[0].Pomos)
		assert.Equal(t, int64(25), stats.History[0].FocusMinutes)
		assert.Equal(t, "2018-12-02", stats.History[1].Date)
		assert.Equal(t, int64(0), stats.TodayPomos)
		assert.Equal(t, int64(45), stats.TodayFocusMinutes)
	})
	t.Run("other user's sessions are not counted", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		stats, err := GetFocusStats(s, &user.User{ID: 3}, time.UTC, 1, now)
		require.NoError(t, err)
		assert.Equal(t, int64(0), stats.TotalPomos)
		assert.Equal(t, int64(0), stats.TotalFocusMinutes)
		require.Len(t, stats.History, 1)
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := GetFocusStats(s, &LinkSharing{ID: 1}, time.UTC, 1, now)
		require.Error(t, err)
	})
}
