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

func TestCountdown_Create(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		berlin, err := time.LoadLocation("Europe/Berlin")
		require.NoError(t, err)
		c := &Countdown{
			Title:        "Wedding day",
			Date:         time.Date(2020, time.May, 9, 23, 30, 0, 0, time.UTC).In(berlin),
			Type:         CountdownTypeAnniversary,
			RepeatYearly: true,
			Icon:         "💍",
			HexColor:     "#ff0000",
			UserID:       99,
		}
		can, err := c.CanCreate(s, u)
		require.NoError(t, err)
		assert.True(t, can)
		require.NoError(t, c.Create(s, u))
		require.NoError(t, s.Commit())

		assert.Equal(t, int64(1), c.UserID, "the owner comes from the auth, not the body")
		assert.Equal(t, time.Date(2020, time.May, 9, 0, 0, 0, 0, time.UTC), c.Date, "the time of day is dropped from the UTC date")
		db.AssertExists(t, "countdowns", map[string]any{
			"id":            c.ID,
			"user_id":       1,
			"title":         "Wedding day",
			"type":          "anniversary",
			"repeat_yearly": true,
			"icon":          "💍",
			"hex_color":     "ff0000",
		}, false)
	})
	t.Run("type defaults to countdown", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Countdown{Title: "Exam", Date: time.Date(2027, time.January, 10, 0, 0, 0, 0, time.UTC)}
		require.NoError(t, c.Create(s, u))
		assert.Equal(t, CountdownTypeCountdown, c.Type)
	})
	t.Run("invalid type", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Countdown{Title: "Bad", Type: "holiday"}
		err := c.Create(s, u)
		require.Error(t, err)
		assert.True(t, IsErrCountdownInvalidType(err))
	})
	t.Run("link share cannot create", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&Countdown{}).CanCreate(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
}

func TestCountdown_ReadAll(t *testing.T) {
	t.Run("only own countdowns, ordered by date", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, count, total, err := (&Countdown{}).ReadAll(s, &user.User{ID: 1}, "", 0, 50)
		require.NoError(t, err)
		countdowns := result.([]*Countdown)
		assert.Equal(t, 2, count)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, int64(2), countdowns[0].ID, "the 1990 birthday comes before the 2027 trip")
		assert.Equal(t, int64(1), countdowns[1].ID)
	})
	t.Run("search", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Countdown{}).ReadAll(s, &user.User{ID: 1}, "japan", 0, 50)
		require.NoError(t, err)
		countdowns := result.([]*Countdown)
		require.Len(t, countdowns, 1)
		assert.Equal(t, "Trip to Japan", countdowns[0].Title)
	})
	t.Run("pagination", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, count, total, err := (&Countdown{}).ReadAll(s, &user.User{ID: 1}, "", 2, 1)
		require.NoError(t, err)
		countdowns := result.([]*Countdown)
		assert.Equal(t, 1, count)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, int64(1), countdowns[0].ID)
	})
	t.Run("user without countdowns", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&Countdown{}).ReadAll(s, &user.User{ID: 3}, "", 0, 50)
		require.NoError(t, err)
		assert.Empty(t, result.([]*Countdown))
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, _, err := (&Countdown{}).ReadAll(s, &LinkSharing{ID: 1}, "", 0, 50)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestCountdown_Permissions(t *testing.T) {
	owner := &user.User{ID: 1}
	other := &user.User{ID: 2}

	t.Run("owner can read, update and delete", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Countdown{ID: 1}
		can, perm, err := c.CanRead(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
		assert.Equal(t, int(PermissionAdmin), perm)
		assert.Equal(t, "Trip to Japan", c.Title, "CanRead loads the countdown")

		can, err = (&Countdown{ID: 1, Title: "x"}).CanUpdate(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
		can, err = (&Countdown{ID: 1}).CanDelete(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
	})
	t.Run("other user cannot read, update or delete", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Countdown{ID: 1}
		can, _, err := c.CanRead(s, other)
		require.NoError(t, err)
		assert.False(t, can)
		assert.Empty(t, c.Title, "must not leak the countdown to a non-owner")

		can, err = (&Countdown{ID: 1, Title: "x"}).CanUpdate(s, other)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&Countdown{ID: 1}).CanDelete(s, other)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("link share has no access", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		share := &LinkSharing{ID: 1}
		can, _, err := (&Countdown{ID: 1}).CanRead(s, share)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&Countdown{ID: 1}).CanUpdate(s, share)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&Countdown{ID: 1}).CanDelete(s, share)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("nonexistent", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, err := (&Countdown{ID: 9999}).CanRead(s, owner)
		assert.True(t, IsErrCountdownDoesNotExist(err))
		_, err = (&Countdown{ID: 9999}).CanUpdate(s, owner)
		assert.True(t, IsErrCountdownDoesNotExist(err))
		_, err = (&Countdown{ID: 9999}).CanDelete(s, owner)
		assert.True(t, IsErrCountdownDoesNotExist(err))
	})
}

func TestCountdown_Update(t *testing.T) {
	owner := &user.User{ID: 1}

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Countdown{
			ID:           1,
			Title:        "Trip to Kyoto",
			Date:         time.Date(2027, time.May, 2, 0, 0, 0, 0, time.UTC),
			Type:         CountdownTypeCountdown,
			RepeatYearly: false,
			Icon:         "🗾",
			HexColor:     "00ff00",
			UserID:       2,
		}
		can, err := c.CanUpdate(s, owner)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, c.Update(s, owner))
		require.NoError(t, s.Commit())

		assert.Equal(t, "Trip to Kyoto", c.Title)
		db.AssertExists(t, "countdowns", map[string]any{
			"id":        1,
			"user_id":   1,
			"title":     "Trip to Kyoto",
			"icon":      "🗾",
			"hex_color": "00ff00",
		}, false)
	})
	t.Run("invalid type", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		c := &Countdown{ID: 1, Title: "Trip", Type: "nope"}
		can, err := c.CanUpdate(s, owner)
		require.NoError(t, err)
		require.True(t, can)
		err = c.Update(s, owner)
		require.Error(t, err)
		assert.True(t, IsErrCountdownInvalidType(err))
	})
}

func TestCountdown_Delete(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	c := &Countdown{ID: 1}
	can, err := c.CanDelete(s, &user.User{ID: 1})
	require.NoError(t, err)
	require.True(t, can)
	require.NoError(t, c.Delete(s, &user.User{ID: 1}))
	require.NoError(t, s.Commit())
	db.AssertMissing(t, "countdowns", map[string]any{"id": 1})
	db.AssertExists(t, "countdowns", map[string]any{"id": 3}, false)
}
