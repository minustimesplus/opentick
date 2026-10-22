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

// Fixtures (pkg/db/fixtures/habits.yml, habit_checkins.yml): habit #1 is daily
// with a goal of 8, done Oct 1-3 and 5-7 2026 with only 3 on Oct 4. Habit #2 is
// due Mon/Wed/Fri and done Wed Oct 7, Fri Oct 9 and Sat Oct 10. Both belong to
// user 1; #3 belongs to user 2.

func habitDay(t *testing.T, date string) time.Time {
	d, err := ParseHabitDate(date)
	require.NoError(t, err)
	return d
}

func TestHabit_Create(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		h := &Habit{Title: "Read", Icon: "📚", HexColor: "#ff0000", Frequency: HabitFrequencyDaily, Weekdays: 3, UserID: 99}
		can, err := h.CanCreate(s, u)
		require.NoError(t, err)
		assert.True(t, can)
		require.NoError(t, h.Create(s, u))
		require.NoError(t, s.Commit())

		assert.Equal(t, int64(1), h.UserID, "the owner comes from the auth, not the body")
		assert.Equal(t, HabitAllWeekdays, h.Weekdays, "daily habits are due every day")
		assert.Equal(t, 1, h.Goal, "the goal defaults to 1")
		assert.NotNil(t, h.RecentCheckins)
		db.AssertExists(t, "habits", map[string]any{
			"id":        h.ID,
			"user_id":   1,
			"title":     "Read",
			"hex_color": "ff0000",
			"frequency": "daily",
			"weekdays":  127,
			"goal":      1,
		}, false)
	})
	t.Run("weekdays", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		h := &Habit{Title: "Weekend run", Frequency: HabitFrequencyWeekdays, Weekdays: 0b1000001 | 0b10000000, Goal: 5}
		require.NoError(t, h.Create(s, u))
		assert.Equal(t, 0b1000001, h.Weekdays, "bits beyond Saturday are dropped")
		assert.Equal(t, 5, h.Goal)
	})
	t.Run("weekdays without a day", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Habit{Title: "Never", Frequency: HabitFrequencyWeekdays}).Create(s, u)
		require.Error(t, err)
		assert.True(t, IsErrHabitInvalidWeekdays(err))
	})
	t.Run("unknown frequency", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Habit{Title: "Hourly", Frequency: "hourly"}).Create(s, u)
		require.Error(t, err)
		assert.True(t, IsErrHabitInvalidFrequency(err))
	})
	t.Run("link share cannot create", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Habit{}).CanCreate(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
}

func TestHabit_ReadAll(t *testing.T) {
	t.Run("only own habits, with progress", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, count, total, err := (&Habit{Today: "2026-10-07"}).ReadAll(s, &user.User{ID: 1}, "", 0, 50)
		require.NoError(t, err)
		habits := result.([]*Habit)
		assert.Equal(t, 2, count)
		assert.Equal(t, int64(2), total)
		require.Len(t, habits, 2)

		water := habits[0]
		assert.Equal(t, int64(1), water.ID)
		assert.Equal(t, 8, water.TodayValue)
		assert.Equal(t, 3, water.CurrentStreak)
		require.Len(t, water.RecentCheckins, 7)
		assert.Equal(t, "2026-10-01", water.RecentCheckins[0].Date)
		assert.Equal(t, "2026-10-07", water.RecentCheckins[6].Date)

		gym := habits[1]
		assert.Equal(t, int64(2), gym.ID)
		assert.Equal(t, 1, gym.TodayValue)
		assert.Equal(t, 1, gym.CurrentStreak)
		require.Len(t, gym.RecentCheckins, 1)
	})
	t.Run("search", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Habit{}).ReadAll(s, &user.User{ID: 1}, "gym", 0, 50)
		require.NoError(t, err)
		habits := result.([]*Habit)
		require.Len(t, habits, 1)
		assert.Equal(t, "Gym", habits[0].Title)
	})
	t.Run("user without habits", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Habit{}).ReadAll(s, &user.User{ID: 3}, "", 0, 50)
		require.NoError(t, err)
		assert.Empty(t, result.([]*Habit))
	})
	t.Run("invalid today", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, _, err := (&Habit{Today: "yesterday"}).ReadAll(s, &user.User{ID: 1}, "", 0, 50)
		require.Error(t, err)
		assert.True(t, IsErrHabitInvalidDate(err))
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, _, err := (&Habit{}).ReadAll(s, &LinkSharing{ID: 1}, "", 0, 50)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestHabit_Permissions(t *testing.T) {
	owner := &user.User{ID: 1}
	other := &user.User{ID: 2}

	t.Run("owner can read and it loads the habit", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		h := &Habit{ID: 1}
		can, perm, err := h.CanRead(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
		assert.Equal(t, int(PermissionAdmin), perm)
		assert.Equal(t, "Drink water", h.Title)
		assert.Equal(t, 8, h.Goal)
	})
	t.Run("owner can update and delete", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Habit{ID: 1}).CanUpdate(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
		can, err = (&Habit{ID: 1}).CanDelete(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
	})
	t.Run("other user cannot read, update or delete", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		h := &Habit{ID: 1}
		can, _, err := h.CanRead(s, other)
		require.NoError(t, err)
		assert.False(t, can)
		assert.Empty(t, h.Title, "must not leak the habit to a non-owner")

		can, err = (&Habit{ID: 1, Title: "x"}).CanUpdate(s, other)
		require.NoError(t, err)
		assert.False(t, can)

		can, err = (&Habit{ID: 1}).CanDelete(s, other)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("link share has no access", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		share := &LinkSharing{ID: 1}
		can, _, err := (&Habit{ID: 1}).CanRead(s, share)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&Habit{ID: 1}).CanUpdate(s, share)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&Habit{ID: 1}).CanDelete(s, share)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("nonexistent", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, err := (&Habit{ID: 9999}).CanRead(s, owner)
		require.Error(t, err)
		assert.True(t, IsErrHabitDoesNotExist(err))
		_, err = (&Habit{ID: 9999}).CanUpdate(s, owner)
		assert.True(t, IsErrHabitDoesNotExist(err))
		_, err = (&Habit{ID: 9999}).CanDelete(s, owner)
		assert.True(t, IsErrHabitDoesNotExist(err))
	})
}

func TestHabit_Update(t *testing.T) {
	owner := &user.User{ID: 1}

	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	h := &Habit{ID: 1, Title: "Water", Frequency: HabitFrequencyWeekdays, Weekdays: 0b0111110, Goal: 3, UserID: 2, Today: "2026-10-07"}
	can, err := h.CanUpdate(s, owner)
	require.NoError(t, err)
	require.True(t, can)
	require.NoError(t, h.Update(s, owner))
	require.NoError(t, s.Commit())

	assert.Equal(t, int64(1), h.UserID, "the owner can never change")
	// With a goal of 3, Oct 4 counts too: Oct 1-7 is a streak of five weekdays.
	assert.Equal(t, 5, h.CurrentStreak)
	db.AssertExists(t, "habits", map[string]any{"id": 1, "user_id": 1, "title": "Water", "frequency": "weekdays", "weekdays": 62, "goal": 3}, false)
}

func TestHabit_Delete(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	require.NoError(t, (&Habit{ID: 1}).Delete(s, &user.User{ID: 1}))
	require.NoError(t, s.Commit())

	db.AssertMissing(t, "habits", map[string]any{"id": 1})
	db.AssertMissing(t, "habit_checkins", map[string]any{"habit_id": 1})
	db.AssertExists(t, "habit_checkins", map[string]any{"habit_id": 2}, false)
}

func TestHabit_DeleteHabitsOfUser(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	require.NoError(t, deleteHabitsOfUser(s, 1))
	require.NoError(t, s.Commit())

	db.AssertMissing(t, "habits", map[string]any{"user_id": 1})
	db.AssertMissing(t, "habit_checkins", map[string]any{"habit_id": 1})
	db.AssertMissing(t, "habit_checkins", map[string]any{"habit_id": 2})
	db.AssertExists(t, "habits", map[string]any{"id": 3}, false)
	db.AssertExists(t, "habit_checkins", map[string]any{"habit_id": 3}, false)
}

func TestHabit_Checkins(t *testing.T) {
	owner := &user.User{ID: 1}

	t.Run("list a range", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		checkins, err := (&Habit{ID: 1}).GetCheckins(s, habitDay(t, "2026-10-03"), habitDay(t, "2026-10-05"))
		require.NoError(t, err)
		require.Len(t, checkins, 3)
		assert.Equal(t, "2026-10-03", checkins[0].Date)
		assert.Equal(t, 9, checkins[0].Value)
		assert.Equal(t, "2026-10-04", checkins[1].Date)
		assert.Equal(t, "2026-10-05", checkins[2].Date)
	})
	t.Run("invalid range", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := (&Habit{ID: 1}).GetCheckins(s, habitDay(t, "2026-10-05"), habitDay(t, "2026-10-03"))
		assert.True(t, IsErrHabitInvalidRange(err))
		_, err = (&Habit{ID: 1}).GetCheckins(s, habitDay(t, "2025-01-01"), habitDay(t, "2026-10-03"))
		assert.True(t, IsErrHabitInvalidRange(err))
	})
	t.Run("check in extends the streak", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		h := &Habit{ID: 1, Today: "2026-10-08"}
		require.NoError(t, h.SetCheckin(s, owner, habitDay(t, "2026-10-08"), 8))
		require.NoError(t, s.Commit())

		assert.Equal(t, "Drink water", h.Title, "returns the reloaded habit")
		assert.Equal(t, 8, h.TodayValue)
		assert.Equal(t, 4, h.CurrentStreak)
		assert.True(t, h.Updated.After(time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)), "a check-in bumps the habit's updated timestamp")
		db.AssertExists(t, "habit_checkins", map[string]any{"habit_id": 1, "value": 8, "id": 12}, false)
	})
	t.Run("check in replaces the day's value", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		h := &Habit{ID: 1, Today: "2026-10-07"}
		require.NoError(t, h.SetCheckin(s, owner, habitDay(t, "2026-10-04"), 8))
		require.NoError(t, s.Commit())

		assert.Equal(t, 7, h.CurrentStreak)
		db.AssertExists(t, "habit_checkins", map[string]any{"id": 4, "value": 8}, false)
	})
	t.Run("value 0 removes the check-in", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		h := &Habit{ID: 1, Today: "2026-10-07"}
		require.NoError(t, h.SetCheckin(s, owner, habitDay(t, "2026-10-07"), 0))
		require.NoError(t, s.Commit())

		assert.Equal(t, 0, h.TodayValue)
		assert.Equal(t, 2, h.CurrentStreak, "today is still pending")
		db.AssertMissing(t, "habit_checkins", map[string]any{"id": 7})
	})
	t.Run("future days are refused", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		err := (&Habit{ID: 1}).SetCheckin(s, owner, time.Now().UTC().AddDate(0, 0, 3), 1)
		require.Error(t, err)
		assert.True(t, IsErrHabitCheckinInFuture(err))
	})
}

func TestHabit_Stats(t *testing.T) {
	owner := &user.User{ID: 1}

	stats := func(t *testing.T, id int64, today, month string) *HabitStats {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		h := &Habit{ID: id}
		can, _, err := h.CanRead(s, owner)
		require.NoError(t, err)
		require.True(t, can)
		st, err := h.GetStats(s, owner, today, month)
		require.NoError(t, err)
		return st
	}

	t.Run("daily habit, today done", func(t *testing.T) {
		st := stats(t, 1, "2026-10-07", "")
		assert.Equal(t, 3, st.CurrentStreak)
		assert.Equal(t, 3, st.LongestStreak)
		assert.Equal(t, 6, st.CompletedDays)
		assert.Equal(t, 7, st.DueDays)
		assert.InDelta(t, 6.0/7, st.CompletionRate, 0.0001)
		assert.Equal(t, 7, st.TotalCheckins)
		assert.Equal(t, 52, st.TotalValue)

		assert.Equal(t, "2026-10", st.Month)
		require.Len(t, st.Days, 31)
		assert.Equal(t, &HabitStatsDay{Date: "2026-10-04", Value: 3, Due: true, Completed: false}, st.Days[3])
		assert.Equal(t, &HabitStatsDay{Date: "2026-10-05", Value: 8, Due: true, Completed: true}, st.Days[4])
	})
	t.Run("today pending keeps the streak", func(t *testing.T) {
		st := stats(t, 1, "2026-10-08", "")
		assert.Equal(t, 3, st.CurrentStreak)
		assert.Equal(t, 7, st.DueDays, "a pending today is not counted")
	})
	t.Run("a missed day breaks the streak", func(t *testing.T) {
		st := stats(t, 1, "2026-10-09", "")
		assert.Equal(t, 0, st.CurrentStreak)
		assert.Equal(t, 3, st.LongestStreak)
		assert.Equal(t, 8, st.DueDays)
	})
	t.Run("weekdays habit skips days it is not due", func(t *testing.T) {
		st := stats(t, 2, "2026-10-10", "")
		assert.Equal(t, 2, st.CurrentStreak)
		assert.Equal(t, 2, st.LongestStreak)
		assert.Equal(t, 2, st.CompletedDays)
		assert.Equal(t, 4, st.DueDays, "Fri 2, Mon 5, Wed 7 and Fri 9")
		assert.InDelta(t, 0.5, st.CompletionRate, 0.0001)
		assert.Equal(t, 3, st.TotalCheckins, "the extra Saturday counts as a check-in")
		assert.False(t, st.Days[9].Due, "Oct 10 is a Saturday")
		assert.True(t, st.Days[9].Completed)
	})
	t.Run("other month", func(t *testing.T) {
		st := stats(t, 1, "2026-10-07", "2026-09")
		assert.Equal(t, "2026-09", st.Month)
		require.Len(t, st.Days, 30)
		assert.Equal(t, "2026-09-01", st.Days[0].Date)
		assert.Zero(t, st.Days[0].Value)
	})
	t.Run("invalid month", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := (&Habit{ID: 1}).GetStats(s, owner, "2026-10-07", "October")
		assert.True(t, IsErrHabitInvalidDate(err))
	})
}
