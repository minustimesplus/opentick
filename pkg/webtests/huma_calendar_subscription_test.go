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
	"net/url"
	"strconv"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures (pkg/db/fixtures/calendar_subscriptions.yml): #1 and #2 belong to
// user1, #3 to user2.
func TestHumaCalendarSubscription(t *testing.T) {
	testHandler := webHandlerTestV2{
		user:     &testuser1,
		basePath: "/api/v2/calendar-subscriptions",
		idParam:  "calendarsubscription",
		t:        t,
	}

	t.Run("ReadAll", func(t *testing.T) {
		rec, err := testHandler.testReadAllWithUser(nil, nil)
		require.NoError(t, err)
		var body struct {
			Items []*models.CalendarSubscription `json:"items"`
			Total int64                          `json:"total"`
		}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		require.Len(t, body.Items, 2)
		assert.Equal(t, int64(1), body.Items[0].ID)
		assert.Equal(t, int64(2), body.Items[1].ID)
		assert.EqualValues(t, 2, body.Total)
		assert.NotContains(t, rec.Body.String(), "secret-token", "another user's subscription must not be listed")
	})

	t.Run("ReadOne", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"calendarsubscription": "1"})
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"title":"Work"`)
			assert.Contains(t, rec.Body.String(), `"url":"https://calendar.example.com/work.ics"`)
			assert.Contains(t, rec.Body.String(), `"max_permission":2`)
			assert.NotEmpty(t, rec.Result().Header.Get("ETag"))
		})
		t.Run("Nonexisting", func(t *testing.T) {
			_, err := testHandler.testReadOneWithUser(nil, map[string]string{"calendarsubscription": "9999"})
			require.Error(t, err)
			assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
		})
		t.Run("Forbidden - other user's subscription", func(t *testing.T) {
			rec, err := testHandler.testReadOneWithUser(nil, map[string]string{"calendarsubscription": "3"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			assert.NotContains(t, rec.Body.String(), "secret-token")
		})
	})

	t.Run("Create", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testCreateWithUser(nil, nil, `{"title":"School","url":"webcal://example.com/school.ics","hex_color":"ff8800"}`)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code)
			assert.Contains(t, rec.Body.String(), `"url":"https://example.com/school.ics"`)
			db.AssertExists(t, "calendar_subscriptions", map[string]any{"title": "School", "user_id": 1}, false)
		})
		t.Run("Invalid url", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"title":"Bad","url":"file:///etc/passwd"}`)
			require.Error(t, err)
			assertHandlerErrorCode(t, err, models.ErrCodeCalendarSubscriptionInvalidURL)
		})
		t.Run("Missing title", func(t *testing.T) {
			_, err := testHandler.testCreateWithUser(nil, nil, `{"title":"","url":"https://example.com/a.ics"}`)
			require.Error(t, err)
		})
	})

	t.Run("Update", func(t *testing.T) {
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testUpdateWithUser(nil, map[string]string{"calendarsubscription": "1"}, `{"title":"Office","url":"https://example.com/office.ics","hex_color":"00aa00"}`)
			require.NoError(t, err)
			assert.Contains(t, rec.Body.String(), `"title":"Office"`)
			db.AssertExists(t, "calendar_subscriptions", map[string]any{"id": 1, "title": "Office", "url": "https://example.com/office.ics"}, false)
		})
		t.Run("Forbidden - other user's subscription", func(t *testing.T) {
			_, err := testHandler.testUpdateWithUser(nil, map[string]string{"calendarsubscription": "3"}, `{"title":"Mine now","url":"https://example.com/a.ics"}`)
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "calendar_subscriptions", map[string]any{"id": 3, "title": "Someone else's calendar"}, false)
		})
	})

	t.Run("Delete", func(t *testing.T) {
		t.Run("Forbidden - other user's subscription", func(t *testing.T) {
			_, err := testHandler.testDeleteWithUser(nil, map[string]string{"calendarsubscription": "3"})
			require.Error(t, err)
			assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
			db.AssertExists(t, "calendar_subscriptions", map[string]any{"id": 3}, false)
		})
		t.Run("Normal", func(t *testing.T) {
			rec, err := testHandler.testDeleteWithUser(nil, map[string]string{"calendarsubscription": "2"})
			require.NoError(t, err)
			assert.Equal(t, http.StatusNoContent, rec.Code)
			db.AssertMissing(t, "calendar_subscriptions", map[string]any{"id": 2})
		})
	})
}

func TestHumaCalendarSubscription_Events(t *testing.T) {
	feed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/team.ics" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = w.Write([]byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nX-WR-CALNAME:Team\r\n" +
			"BEGIN:VEVENT\r\nUID:standup\r\nDTSTART;TZID=Europe/Berlin:20261012T093000\r\nDURATION:PT15M\r\nRRULE:FREQ=WEEKLY;BYDAY=MO,WE\r\nSUMMARY:Standup\r\nEND:VEVENT\r\n" +
			"BEGIN:VEVENT\r\nUID:offsite\r\nDTSTART;VALUE=DATE:20261015\r\nSUMMARY:Offsite\r\nEND:VEVENT\r\n" +
			"END:VCALENDAR\r\n"))
	}))
	defer feed.Close()

	previous := config.OutgoingRequestsAllowNonRoutableIPs.GetBool()
	config.OutgoingRequestsAllowNonRoutableIPs.Set(true)
	t.Cleanup(func() { config.OutgoingRequestsAllowNonRoutableIPs.Set(previous) })

	testHandler := webHandlerTestV2{user: &testuser1, basePath: "/api/v2/calendar-subscriptions", t: t}
	rec, err := testHandler.testCreateWithUser(nil, nil, `{"title":"Team","url":"`+feed.URL+`/team.ics"}`)
	require.NoError(t, err)
	var created models.CalendarSubscription
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))

	eventsPath := func(id int64, q url.Values) string {
		return "/api/v2/calendar-subscriptions/" + strconv.FormatInt(id, 10) + "/events?" + q.Encode()
	}
	week := url.Values{
		"from":     {"2026-10-12T00:00:00+02:00"},
		"to":       {"2026-10-19T00:00:00+02:00"},
		"timezone": {"Europe/Berlin"},
	}

	t.Run("Normal", func(t *testing.T) {
		rec, err := testHandler.serve(http.MethodGet, eventsPath(created.ID, week), "")
		require.NoError(t, err)
		var body models.CalendarSubscriptionEvents
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
		assert.Equal(t, "Team", body.CalendarName)
		require.Len(t, body.Events, 3, rec.Body.String())
		assert.Equal(t, "Standup", body.Events[0].Title)
		assert.Equal(t, "2026-10-12T09:30:00+02:00", body.Events[0].Start.Format("2006-01-02T15:04:05-07:00"))
		assert.True(t, body.Events[0].Recurring)
		assert.Equal(t, "Offsite", body.Events[2].Title)
		assert.True(t, body.Events[2].AllDay)
	})
	t.Run("Forbidden - other user's subscription", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, eventsPath(3, week), "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("Missing range", func(t *testing.T) {
		_, err := testHandler.serve(http.MethodGet, eventsPath(created.ID, url.Values{}), "")
		require.Error(t, err)
		assert.Equal(t, http.StatusUnprocessableEntity, getHTTPErrorCode(err))
	})
	t.Run("Range too long", func(t *testing.T) {
		q := url.Values{"from": {"2026-01-01T00:00:00Z"}, "to": {"2028-01-01T00:00:00Z"}}
		_, err := testHandler.serve(http.MethodGet, eventsPath(created.ID, q), "")
		require.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeCalendarSubscriptionInvalidRange)
	})
	t.Run("Unknown timezone", func(t *testing.T) {
		q := url.Values{"from": week["from"], "to": week["to"], "timezone": {"Mars/Olympus"}}
		_, err := testHandler.serve(http.MethodGet, eventsPath(created.ID, q), "")
		require.Error(t, err)
		assert.Equal(t, http.StatusBadRequest, getHTTPErrorCode(err))
	})
	t.Run("Unreachable feed", func(t *testing.T) {
		rec, err := testHandler.testCreateWithUser(nil, nil, `{"title":"Gone","url":"`+feed.URL+`/gone.ics"}`)
		require.NoError(t, err)
		var gone models.CalendarSubscription
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &gone))

		_, err = testHandler.serve(http.MethodGet, eventsPath(gone.ID, week), "")
		require.Error(t, err)
		assert.Equal(t, http.StatusBadGateway, getHTTPErrorCode(err))
		assertHandlerErrorCode(t, err, models.ErrCodeCalendarSubscriptionFetchFailed)
	})
}
