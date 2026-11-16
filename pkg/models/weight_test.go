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

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"xorm.io/builder"
)

// Fixtures (pkg/db/fixtures/weights.yml): user 1 weighed in Oct 1-3 2026, user 2 on Oct 2.

func TestGetWeights(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("range", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		weights, err := GetWeights(s, u, habitDay(t, "2026-10-02"), habitDay(t, "2026-10-31"))
		require.NoError(t, err)
		require.Len(t, weights, 2, "only the user's own weights, from the first day on")
		assert.Equal(t, "2026-10-02", weights[0].Date)
		assert.InDelta(t, 80.1, weights[0].Weight, 0.001)
		assert.Equal(t, "2026-10-03", weights[1].Date)
	})
	t.Run("reversed range", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := GetWeights(s, u, habitDay(t, "2026-10-05"), habitDay(t, "2026-10-01"))
		assert.True(t, IsErrWeightInvalidRange(err))
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := GetWeights(s, &LinkSharing{ID: 1}, habitDay(t, "2026-10-01"), habitDay(t, "2026-10-31"))
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestSetWeight(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("new day", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		w, err := SetWeight(s, u, habitDay(t, "2026-10-04"), 79.66, "2026-10-04")
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		assert.Equal(t, "2026-10-04", w.Date)
		assert.InDelta(t, 79.7, w.Weight, 0.001, "rounded to 0.1 kg")
		db.AssertExists(t, "weights", map[string]any{"user_id": 1, "weight": 79.7}, false)
	})
	t.Run("replaces the day's weight", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := SetWeight(s, u, habitDay(t, "2026-10-03"), 79.5, "")
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "weights", map[string]any{"id": 3, "weight": 79.5}, false)
		db.AssertCount(t, "weights", builder.Eq{"user_id": 1}, 3)
	})
	t.Run("checks off weigh-in habits", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		// Habit 2 (Gym) stands in for a weigh-in habit; habit 1 doesn't track weight.
		_, err := s.ID(2).Cols("tracks_weight").Update(&Habit{TracksWeight: true})
		require.NoError(t, err)
		// Habit 3 is user 2's and must not be touched.
		_, err = s.ID(3).Cols("tracks_weight").Update(&Habit{TracksWeight: true})
		require.NoError(t, err)

		_, err = SetWeight(s, u, habitDay(t, "2026-10-12"), 79, "2026-10-12")
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "habit_checkins", map[string]any{"habit_id": 2, "value": 1}, false)
		db.AssertCount(t, "habit_checkins", builder.Eq{"habit_id": 1}, 7)
		db.AssertCount(t, "habit_checkins", builder.Eq{"habit_id": 3}, 1)
	})
	t.Run("out of range", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := SetWeight(s, u, habitDay(t, "2026-10-04"), 0.2, "")
		assert.True(t, IsErrWeightInvalid(err))
	})
	t.Run("future day", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := SetWeight(s, u, habitDay(t, "2999-01-01"), 80, "")
		assert.True(t, IsErrHabitCheckinInFuture(err))
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, err := SetWeight(s, &LinkSharing{ID: 1}, habitDay(t, "2026-10-04"), 80, "")
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestDeleteWeight(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	require.NoError(t, DeleteWeight(s, &user.User{ID: 1}, habitDay(t, "2026-10-02")))
	require.NoError(t, s.Commit())
	db.AssertMissing(t, "weights", map[string]any{"id": 2})
	db.AssertExists(t, "weights", map[string]any{"id": 4}, false)
}

func TestDeleteWeightsOfUser(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	require.NoError(t, deleteWeightsOfUser(s, 1))
	require.NoError(t, s.Commit())
	db.AssertMissing(t, "weights", map[string]any{"user_id": 1})
	db.AssertExists(t, "weights", map[string]any{"id": 4}, false)
}
