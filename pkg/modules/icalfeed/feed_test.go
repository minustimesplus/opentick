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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func calendar(events ...string) []byte {
	lines := []string{"BEGIN:VCALENDAR", "VERSION:2.0", "PRODID:-//test//EN", "X-WR-CALNAME:Work"}
	for _, e := range events {
		lines = append(lines, "BEGIN:VEVENT")
		lines = append(lines, strings.Split(strings.TrimSpace(e), "\n")...)
		lines = append(lines, "END:VEVENT")
	}
	lines = append(lines, "END:VCALENDAR")
	return []byte(strings.Join(lines, "\r\n") + "\r\n")
}

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

func expand(t *testing.T, data []byte, from, to time.Time, loc *time.Location) []*Event {
	t.Helper()
	f, err := Parse(data)
	require.NoError(t, err)
	return f.Expand(from, to, loc)
}

func TestParse(t *testing.T) {
	t.Run("calendar name", func(t *testing.T) {
		f, err := Parse(calendar())
		require.NoError(t, err)
		assert.Equal(t, "Work", f.Name)
	})
	t.Run("not a calendar", func(t *testing.T) {
		_, err := Parse([]byte("<html>nope</html>"))
		require.ErrorIs(t, err, ErrNotACalendar)
	})
	t.Run("skips malformed lines", func(t *testing.T) {
		data := calendar(`UID:a
DTSTART:20261016T090000Z
DTEND:20261016T100000Z
X-BROKEN;PARAM_WITH_UNDERSCORE=1:value
SUMMARY:Still here`)
		events := expand(t, data, time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC), time.UTC)
		require.Len(t, events, 1)
		assert.Equal(t, "Still here", events[0].Title)
	})
}

func TestExpand(t *testing.T) {
	berlin := mustLoc(t, "Europe/Berlin")
	newYork := mustLoc(t, "America/New_York")
	octFrom := time.Date(2026, 10, 1, 0, 0, 0, 0, berlin)
	octTo := time.Date(2026, 11, 1, 0, 0, 0, 0, berlin)

	t.Run("single utc event with escaped text", func(t *testing.T) {
		events := expand(t, calendar(`UID:one
DTSTART:20261016T090000Z
DTEND:20261016T103000Z
SUMMARY:Standup\, daily
DESCRIPTION:Line one\nLine two
LOCATION:Room 1`), octFrom, octTo, berlin)
		require.Len(t, events, 1)
		e := events[0]
		assert.Equal(t, "Standup, daily", e.Title)
		assert.Equal(t, "Line one\nLine two", e.Description)
		assert.Equal(t, "Room 1", e.Location)
		assert.True(t, e.Start.Equal(time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)))
		assert.True(t, e.End.Equal(time.Date(2026, 10, 16, 10, 30, 0, 0, time.UTC)))
		assert.False(t, e.AllDay)
		assert.False(t, e.Recurring)
	})

	t.Run("event outside the range is dropped", func(t *testing.T) {
		events := expand(t, calendar(`UID:one
DTSTART:20261116T090000Z
DTEND:20261116T100000Z`), octFrom, octTo, berlin)
		assert.Empty(t, events)
	})

	t.Run("event overlapping the range start is kept", func(t *testing.T) {
		events := expand(t, calendar(`UID:one
DTSTART;VALUE=DATE:20260928
DTEND;VALUE=DATE:20261003`), octFrom, octTo, berlin)
		require.Len(t, events, 1)
	})

	t.Run("tzid event", func(t *testing.T) {
		events := expand(t, calendar(`UID:tz
DTSTART;TZID=America/New_York:20261016T090000
DTEND;TZID=America/New_York:20261016T100000`), octFrom, octTo, berlin)
		require.Len(t, events, 1)
		assert.True(t, events[0].Start.Equal(time.Date(2026, 10, 16, 9, 0, 0, 0, newYork)))
	})

	t.Run("windows tzid", func(t *testing.T) {
		events := expand(t, calendar(`UID:win
DTSTART;TZID=W. Europe Standard Time:20261016T090000
DTEND;TZID=W. Europe Standard Time:20261016T100000`), octFrom, octTo, time.UTC)
		require.Len(t, events, 1)
		assert.True(t, events[0].Start.Equal(time.Date(2026, 10, 16, 9, 0, 0, 0, berlin)))
	})

	t.Run("vendor prefixed tzid", func(t *testing.T) {
		events := expand(t, calendar(`UID:pfx
DTSTART;TZID=/freeassociation.sourceforge.net/Tzfile/Europe/Berlin:20261016T090000
DURATION:PT1H`), octFrom, octTo, time.UTC)
		require.Len(t, events, 1)
		assert.True(t, events[0].Start.Equal(time.Date(2026, 10, 16, 9, 0, 0, 0, berlin)))
		assert.Equal(t, time.Hour, events[0].End.Sub(events[0].Start))
	})

	t.Run("floating and unknown tzid use the viewer timezone", func(t *testing.T) {
		events := expand(t, calendar(`UID:float
DTSTART:20261016T090000
DTEND:20261016T100000`, `UID:custom
DTSTART;TZID=Customized Time Zone:20261017T090000
DTEND;TZID=Customized Time Zone:20261017T100000`), octFrom, octTo, newYork)
		require.Len(t, events, 2)
		assert.True(t, events[0].Start.Equal(time.Date(2026, 10, 16, 9, 0, 0, 0, newYork)))
		assert.True(t, events[1].Start.Equal(time.Date(2026, 10, 17, 9, 0, 0, 0, newYork)))
	})

	t.Run("all-day event pinned to viewer midnight", func(t *testing.T) {
		events := expand(t, calendar(`UID:allday
DTSTART;VALUE=DATE:20261016
DTEND;VALUE=DATE:20261018`), octFrom, octTo, berlin)
		require.Len(t, events, 1)
		e := events[0]
		assert.True(t, e.AllDay)
		assert.True(t, e.Start.Equal(time.Date(2026, 10, 16, 0, 0, 0, 0, berlin)))
		assert.True(t, e.End.Equal(time.Date(2026, 10, 18, 0, 0, 0, 0, berlin)))
	})

	t.Run("all-day event without end lasts one day", func(t *testing.T) {
		events := expand(t, calendar(`UID:allday
DTSTART;VALUE=DATE:20261016`), octFrom, octTo, berlin)
		require.Len(t, events, 1)
		assert.True(t, events[0].End.Equal(time.Date(2026, 10, 17, 0, 0, 0, 0, berlin)))
	})

	t.Run("weekly recurrence with exdate and count", func(t *testing.T) {
		events := expand(t, calendar(`UID:weekly
DTSTART;TZID=Europe/Berlin:20261005T090000
DTEND;TZID=Europe/Berlin:20261005T093000
RRULE:FREQ=WEEKLY;BYDAY=MO;COUNT=10
EXDATE;TZID=Europe/Berlin:20261012T090000`), octFrom, octTo, berlin)
		require.Len(t, events, 3) // 5th, (12th excluded), 19th, 26th
		assert.Equal(t, 5, events[0].Start.In(berlin).Day())
		assert.Equal(t, 19, events[1].Start.In(berlin).Day())
		assert.Equal(t, 26, events[2].Start.In(berlin).Day())
		for _, e := range events {
			assert.True(t, e.Recurring)
			assert.Equal(t, 9, e.Start.In(berlin).Hour(), "keeps local time across DST")
		}
		assert.NotEqual(t, events[0].ID, events[1].ID)
	})

	t.Run("recurrence started long ago", func(t *testing.T) {
		events := expand(t, calendar(`UID:daily
DTSTART:20100101T080000Z
DURATION:PT1H
RRULE:FREQ=DAILY`), time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC), time.UTC)
		require.Len(t, events, 3)
		assert.True(t, events[0].Start.Equal(time.Date(2026, 10, 16, 8, 0, 0, 0, time.UTC)))
	})

	t.Run("recurrence with until", func(t *testing.T) {
		events := expand(t, calendar(`UID:until
DTSTART:20261001T080000
RRULE:FREQ=DAILY;UNTIL=20261003T235959`), octFrom, octTo, berlin)
		require.Len(t, events, 3)
	})

	t.Run("all-day yearly recurrence", func(t *testing.T) {
		events := expand(t, calendar(`UID:bday
DTSTART;VALUE=DATE:20001016
RRULE:FREQ=YEARLY`), octFrom, octTo, newYork)
		require.Len(t, events, 1)
		assert.True(t, events[0].Start.Equal(time.Date(2026, 10, 16, 0, 0, 0, 0, newYork)))
		assert.True(t, events[0].AllDay)
	})

	t.Run("overridden and cancelled instances", func(t *testing.T) {
		events := expand(t, calendar(`UID:series
DTSTART:20261005T090000Z
DTEND:20261005T100000Z
SUMMARY:Series
RRULE:FREQ=DAILY;COUNT=3`, `UID:series
RECURRENCE-ID:20261006T090000Z
DTSTART:20261006T140000Z
DTEND:20261006T150000Z
SUMMARY:Moved`, `UID:series
RECURRENCE-ID:20261007T090000Z
DTSTART:20261007T090000Z
DTEND:20261007T100000Z
STATUS:CANCELLED`), octFrom, octTo, berlin)
		require.Len(t, events, 2)
		assert.Equal(t, "Series", events[0].Title)
		assert.Equal(t, "Moved", events[1].Title)
		assert.Equal(t, 14, events[1].Start.UTC().Hour())
	})

	t.Run("cancelled event is hidden", func(t *testing.T) {
		events := expand(t, calendar(`UID:c
DTSTART:20261016T090000Z
STATUS:CANCELLED`), octFrom, octTo, berlin)
		assert.Empty(t, events)
	})

	t.Run("rdate adds occurrences", func(t *testing.T) {
		events := expand(t, calendar(`UID:r
DTSTART:20261005T090000Z
DURATION:PT1H
RDATE:20261008T090000Z,20261009T090000Z`), octFrom, octTo, berlin)
		require.Len(t, events, 3)
	})

	t.Run("unsupported rrule falls back to the first occurrence", func(t *testing.T) {
		events := expand(t, calendar(`UID:bad
DTSTART:20261005T090000Z
RRULE:FREQ=DAILY;RSCALE=HEBREW`), octFrom, octTo, berlin)
		require.Len(t, events, 1)
	})

	t.Run("runaway recurrence is capped", func(t *testing.T) {
		events := expand(t, calendar(`UID:minutely
DTSTART:20261001T000000Z
RRULE:FREQ=MINUTELY`), octFrom, octTo, berlin)
		assert.Len(t, events, maxOccurrencesPerEvent)
	})

	t.Run("sorted by start", func(t *testing.T) {
		events := expand(t, calendar(`UID:b
DTSTART:20261020T090000Z`, `UID:a
DTSTART:20261010T090000Z`), octFrom, octTo, berlin)
		require.Len(t, events, 2)
		assert.Equal(t, "a", events[0].UID)
	})
}

func TestParseDuration(t *testing.T) {
	cases := map[string]time.Duration{
		"PT1H":       time.Hour,
		"PT1H30M":    90 * time.Minute,
		"P1D":        24 * time.Hour,
		"P1W":        7 * 24 * time.Hour,
		"P1DT2H3M4S": 26*time.Hour + 3*time.Minute + 4*time.Second,
		"-PT15M":     -15 * time.Minute,
	}
	for in, want := range cases {
		got, err := parseDuration(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	_, err := parseDuration("1 hour")
	require.Error(t, err)
}

func TestNormalizeURL(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"https://example.com/cal.ics", "https://example.com/cal.ics"},
		{"  webcal://example.com/cal.ics\n", "https://example.com/cal.ics"},
		{"WEBCALS://example.com/a.ics?token=1", "https://example.com/a.ics?token=1"},
		{"HTTP://example.com/a.ics", "http://example.com/a.ics"},
	} {
		got, err := NormalizeURL(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got)
	}
	for _, in := range []string{"", "example.com/cal.ics", "ftp://example.com/cal.ics", "file:///etc/passwd", "https://"} {
		_, err := NormalizeURL(in)
		require.ErrorIs(t, err, ErrInvalidURL, in)
	}
}
