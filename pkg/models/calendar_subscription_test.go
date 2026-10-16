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
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCalendarSubscription_Create(t *testing.T) {
	u := &user.User{ID: 1}

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		cs := &CalendarSubscription{Title: "School", URL: "webcal://example.com/school.ics", HexColor: "#ff0000", UserID: 99}
		can, err := cs.CanCreate(s, u)
		require.NoError(t, err)
		assert.True(t, can)
		require.NoError(t, cs.Create(s, u))
		require.NoError(t, s.Commit())

		assert.Equal(t, int64(1), cs.UserID, "the owner comes from the auth, not the body")
		assert.Equal(t, "https://example.com/school.ics", cs.URL)
		db.AssertExists(t, "calendar_subscriptions", map[string]any{
			"id":        cs.ID,
			"user_id":   1,
			"title":     "School",
			"url":       "https://example.com/school.ics",
			"hex_color": "ff0000",
		}, false)
	})
	t.Run("invalid url", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		for _, url := range []string{"file:///etc/passwd", "not a url", "ftp://example.com/a.ics"} {
			cs := &CalendarSubscription{Title: "Bad", URL: url}
			err := cs.Create(s, u)
			require.Error(t, err, url)
			assert.True(t, IsErrCalendarSubscriptionInvalidURL(err), url)
		}
	})
	t.Run("link share cannot create", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		can, err := (&CalendarSubscription{}).CanCreate(s, &LinkSharing{ID: 1})
		require.NoError(t, err)
		assert.False(t, can)
	})
}

func TestCalendarSubscription_ReadAll(t *testing.T) {
	t.Run("only own subscriptions", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, count, total, err := (&CalendarSubscription{}).ReadAll(s, &user.User{ID: 1}, "", 0, 50)
		require.NoError(t, err)
		subs := result.([]*CalendarSubscription)
		assert.Equal(t, 2, count)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, int64(1), subs[0].ID)
		assert.Equal(t, int64(2), subs[1].ID)
	})
	t.Run("search", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&CalendarSubscription{}).ReadAll(s, &user.User{ID: 1}, "holi", 0, 50)
		require.NoError(t, err)
		subs := result.([]*CalendarSubscription)
		require.Len(t, subs, 1)
		assert.Equal(t, "Holidays", subs[0].Title)
	})
	t.Run("pagination", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, count, total, err := (&CalendarSubscription{}).ReadAll(s, &user.User{ID: 1}, "", 2, 1)
		require.NoError(t, err)
		subs := result.([]*CalendarSubscription)
		assert.Equal(t, 1, count)
		assert.Equal(t, int64(2), total)
		assert.Equal(t, int64(2), subs[0].ID)
	})
	t.Run("user without subscriptions", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		result, _, _, err := (&CalendarSubscription{}).ReadAll(s, &user.User{ID: 3}, "", 0, 50)
		require.NoError(t, err)
		assert.Empty(t, result.([]*CalendarSubscription))
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, _, err := (&CalendarSubscription{}).ReadAll(s, &LinkSharing{ID: 1}, "", 0, 50)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestCalendarSubscription_Permissions(t *testing.T) {
	owner := &user.User{ID: 1}
	other := &user.User{ID: 2}

	t.Run("owner can read and it loads the subscription", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		cs := &CalendarSubscription{ID: 1}
		can, perm, err := cs.CanRead(s, owner)
		require.NoError(t, err)
		assert.True(t, can)
		assert.Equal(t, int(PermissionAdmin), perm)
		assert.Equal(t, "Work", cs.Title)
		assert.Equal(t, "https://calendar.example.com/work.ics", cs.URL)
	})
	t.Run("other user cannot read, update or delete", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		cs := &CalendarSubscription{ID: 1}
		can, _, err := cs.CanRead(s, other)
		require.NoError(t, err)
		assert.False(t, can)
		assert.Empty(t, cs.URL, "must not leak the url to a non-owner")

		can, err = (&CalendarSubscription{ID: 1, Title: "x", URL: "https://example.com"}).CanUpdate(s, other)
		require.NoError(t, err)
		assert.False(t, can)

		can, err = (&CalendarSubscription{ID: 1}).CanDelete(s, other)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("link share has no access", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		share := &LinkSharing{ID: 1}
		can, _, err := (&CalendarSubscription{ID: 1}).CanRead(s, share)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&CalendarSubscription{ID: 1}).CanUpdate(s, share)
		require.NoError(t, err)
		assert.False(t, can)
		can, err = (&CalendarSubscription{ID: 1}).CanDelete(s, share)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("nonexistent", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		_, _, err := (&CalendarSubscription{ID: 9999}).CanRead(s, owner)
		require.Error(t, err)
		assert.True(t, IsErrCalendarSubscriptionDoesNotExist(err))
		_, err = (&CalendarSubscription{ID: 9999}).CanUpdate(s, owner)
		assert.True(t, IsErrCalendarSubscriptionDoesNotExist(err))
		_, err = (&CalendarSubscription{ID: 9999}).CanDelete(s, owner)
		assert.True(t, IsErrCalendarSubscriptionDoesNotExist(err))
	})
}

func TestCalendarSubscription_Update(t *testing.T) {
	owner := &user.User{ID: 1}

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		cs := &CalendarSubscription{ID: 1, Title: "Office", URL: "webcal://example.com/new.ics", HexColor: "00ff00", UserID: 2}
		can, err := cs.CanUpdate(s, owner)
		require.NoError(t, err)
		require.True(t, can)
		require.NoError(t, cs.Update(s, owner))
		require.NoError(t, s.Commit())

		assert.Equal(t, "Office", cs.Title)
		db.AssertExists(t, "calendar_subscriptions", map[string]any{
			"id":        1,
			"user_id":   1,
			"title":     "Office",
			"url":       "https://example.com/new.ics",
			"hex_color": "00ff00",
		}, false)
	})
	t.Run("invalid url", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()

		cs := &CalendarSubscription{ID: 1, Title: "Office", URL: "javascript:alert(1)"}
		can, err := cs.CanUpdate(s, owner)
		require.NoError(t, err)
		require.True(t, can)
		err = cs.Update(s, owner)
		require.Error(t, err)
		assert.True(t, IsErrCalendarSubscriptionInvalidURL(err))
	})
}

func TestCalendarSubscription_Delete(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	cs := &CalendarSubscription{ID: 1}
	can, err := cs.CanDelete(s, &user.User{ID: 1})
	require.NoError(t, err)
	require.True(t, can)
	require.NoError(t, cs.Delete(s, &user.User{ID: 1}))
	require.NoError(t, s.Commit())
	db.AssertMissing(t, "calendar_subscriptions", map[string]any{"id": 1})
	db.AssertExists(t, "calendar_subscriptions", map[string]any{"id": 2}, false)
}

func TestCalendarSubscription_GetEvents(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cal.ics" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-CALNAME:Team\r\n" +
			"BEGIN:VEVENT\r\nUID:standup\r\nDTSTART;TZID=Europe/Berlin:20261012T093000\r\nDURATION:PT15M\r\nRRULE:FREQ=DAILY;BYDAY=MO,TU,WE,TH,FR\r\nSUMMARY:Standup\r\nEND:VEVENT\r\n" +
			"END:VCALENDAR\r\n"))
	}))
	defer srv.Close()

	previous := config.OutgoingRequestsAllowNonRoutableIPs.GetBool()
	config.OutgoingRequestsAllowNonRoutableIPs.Set(true)
	t.Cleanup(func() { config.OutgoingRequestsAllowNonRoutableIPs.Set(previous) })

	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	from := time.Date(2026, 10, 12, 0, 0, 0, 0, berlin)
	to := from.AddDate(0, 0, 7)

	t.Run("expands the feed", func(t *testing.T) {
		cs := &CalendarSubscription{ID: 5, URL: srv.URL + "/cal.ics"}
		res, err := cs.GetEvents(context.Background(), from, to, berlin)
		require.NoError(t, err)
		assert.Equal(t, int64(5), res.SubscriptionID)
		assert.Equal(t, "Team", res.CalendarName)
		require.Len(t, res.Events, 5)
		assert.Equal(t, "Standup", res.Events[0].Title)
		assert.True(t, res.Events[0].Start.Equal(time.Date(2026, 10, 12, 9, 30, 0, 0, berlin)))
	})
	t.Run("unreachable feed", func(t *testing.T) {
		cs := &CalendarSubscription{ID: 5, URL: srv.URL + "/gone.ics"}
		_, err := cs.GetEvents(context.Background(), from, to, berlin)
		require.Error(t, err)
		assert.True(t, IsErrCalendarSubscriptionFetchFailed(err))
	})
	t.Run("invalid range", func(t *testing.T) {
		cs := &CalendarSubscription{ID: 5, URL: srv.URL + "/cal.ics"}
		_, err := cs.GetEvents(context.Background(), to, from, berlin)
		assert.True(t, IsErrCalendarSubscriptionInvalidRange(err))
		_, err = cs.GetEvents(context.Background(), from, from.AddDate(2, 0, 0), berlin)
		assert.True(t, IsErrCalendarSubscriptionInvalidRange(err))
	})
}
