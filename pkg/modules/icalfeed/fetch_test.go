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

package icalfeed

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"code.vikunja.io/api/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetch(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch r.URL.Path {
		case "/cal.ics":
			w.Header().Set("Content-Type", "text/calendar")
			_, _ = w.Write(calendar(`UID:a
DTSTART:20261016T090000Z`))
		case "/html":
			_, _ = w.Write([]byte("<html></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	t.Run("blocks private addresses by default", func(t *testing.T) {
		config.OutgoingRequestsAllowNonRoutableIPs.Set(false)
		_, err := Fetch(context.Background(), srv.URL+"/blocked.ics")
		var fe *FetchError
		require.ErrorAs(t, err, &fe)
		assert.Equal(t, int32(0), hits.Load())
	})

	config.OutgoingRequestsAllowNonRoutableIPs.Set(true)
	defer config.OutgoingRequestsAllowNonRoutableIPs.Set(false)

	t.Run("fetches, parses and caches", func(t *testing.T) {
		before := hits.Load()
		f, err := Fetch(context.Background(), srv.URL+"/cal.ics")
		require.NoError(t, err)
		assert.Equal(t, "Work", f.Name)
		_, err = Fetch(context.Background(), srv.URL+"/cal.ics")
		require.NoError(t, err)
		assert.Equal(t, before+1, hits.Load())

		Invalidate(srv.URL + "/cal.ics")
		_, err = Fetch(context.Background(), srv.URL+"/cal.ics")
		require.NoError(t, err)
		assert.Equal(t, before+2, hits.Load())
	})

	t.Run("http error", func(t *testing.T) {
		_, err := Fetch(context.Background(), srv.URL+"/missing.ics")
		var fe *FetchError
		require.ErrorAs(t, err, &fe)
		assert.Contains(t, fe.Reason, "404")
	})

	t.Run("not a calendar", func(t *testing.T) {
		_, err := Fetch(context.Background(), srv.URL+"/html")
		var fe *FetchError
		require.ErrorAs(t, err, &fe)
		assert.Contains(t, fe.Reason, "iCalendar")
	})

	t.Run("invalid url", func(t *testing.T) {
		_, err := Fetch(context.Background(), "ftp://example.com/a.ics")
		require.ErrorIs(t, err, ErrInvalidURL)
	})
}
