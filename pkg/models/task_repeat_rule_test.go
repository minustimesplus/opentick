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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mustLoc(t *testing.T, name string) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation(name)
	require.NoError(t, err)
	return loc
}

func TestNormalizeTaskRepeatRule(t *testing.T) {
	valid := map[string]string{
		"":                                       "",
		"FREQ=DAILY":                             "FREQ=DAILY",
		"RRULE:FREQ=DAILY;INTERVAL=1":            "FREQ=DAILY",
		"freq=weekly;interval=2;byday=fr,mo":     "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,FR",
		"FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR;":      "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR",
		"FREQ=MONTHLY;BYDAY=-1FR":                "FREQ=MONTHLY;BYDAY=-1FR",
		"FREQ=MONTHLY;BYDAY=+2TU":                "FREQ=MONTHLY;BYDAY=2TU",
		"FREQ=MONTHLY;BYMONTHDAY=05,-1":          "FREQ=MONTHLY;BYMONTHDAY=-1,5",
		"FREQ=YEARLY;BYMONTH=3,1;BYMONTHDAY=15":  "FREQ=YEARLY;BYMONTHDAY=15;BYMONTH=1,3",
		"FREQ=YEARLY;BYMONTH=11;BYDAY=4TH":       "FREQ=YEARLY;BYDAY=4TH;BYMONTH=11",
		"FREQ=DAILY;COUNT=10":                    "FREQ=DAILY;COUNT=10",
		"FREQ=DAILY;UNTIL=20261231":              "FREQ=DAILY;UNTIL=20261231",
		"FREQ=DAILY;UNTIL=20261231T120000Z":      "FREQ=DAILY;UNTIL=20261231T120000Z",
		"FREQ=WEEKLY;WKST=SU;BYDAY=SU,SA":        "FREQ=WEEKLY;BYDAY=SU,SA;WKST=SU",
		"FREQ=WEEKLY;WKST=MO":                    "FREQ=WEEKLY",
		"FREQ=DAILY;BYDAY=MO,MO":                 "FREQ=DAILY;BYDAY=MO",
		"  FREQ=DAILY;INTERVAL=999  ":            "FREQ=DAILY;INTERVAL=999",
		"FREQ=MONTHLY;BYDAY=MO,TU;BYMONTHDAY=13": "FREQ=MONTHLY;BYDAY=MO,TU;BYMONTHDAY=13",
	}
	for in, want := range valid {
		t.Run("valid "+in, func(t *testing.T) {
			got, err := NormalizeTaskRepeatRule(in)
			require.NoError(t, err)
			assert.Equal(t, want, got)
			// Canonical output must be a fixed point.
			again, err := NormalizeTaskRepeatRule(got)
			require.NoError(t, err)
			assert.Equal(t, got, again)
		})
	}

	invalid := []string{
		"RRULE:",
		"INTERVAL=2",
		"FREQ=HOURLY",
		"FREQ=SECONDLY;INTERVAL=5",
		"FREQ=DAILY;INTERVAL=0",
		"FREQ=DAILY;INTERVAL=1000",
		"FREQ=DAILY;INTERVAL=abc",
		"FREQ=DAILY;FREQ=WEEKLY",
		"FREQ=DAILY;COUNT=0",
		"FREQ=DAILY;COUNT=3;UNTIL=20261231",
		"FREQ=DAILY;UNTIL=2026-12-31",
		"FREQ=DAILY;BYDAY=1MO",
		"FREQ=WEEKLY;BYDAY=-1FR",
		"FREQ=WEEKLY;BYMONTHDAY=3",
		"FREQ=MONTHLY;BYDAY=6MO",
		"FREQ=MONTHLY;BYDAY=0MO",
		"FREQ=MONTHLY;BYDAY=XX",
		"FREQ=MONTHLY;BYMONTHDAY=32",
		"FREQ=MONTHLY;BYMONTHDAY=0",
		"FREQ=YEARLY;BYMONTH=13",
		"FREQ=YEARLY;BYMONTH=-1",
		"FREQ=YEARLY;BYDAY=MO",
		"FREQ=DAILY;BYSETPOS=1",
		"FREQ=DAILY;BYHOUR=9",
		"FREQ=DAILY;WKST=XX",
		"FREQ=DAILY;garbage",
		"FREQ=DAILY;INTERVAL=",
		"FREQ=DAILY;" + string(make([]byte, 600)),
	}
	for _, in := range invalid {
		t.Run("invalid "+in, func(t *testing.T) {
			_, err := NormalizeTaskRepeatRule(in)
			require.Error(t, err)
			assert.True(t, IsErrInvalidTaskRepeatRule(err))
		})
	}
}

func TestRepeatRuleNextOccurrence(t *testing.T) {
	berlin := mustLoc(t, "Europe/Berlin")
	d := func(y int, m time.Month, day, h, min int) time.Time {
		return time.Date(y, m, day, h, min, 0, 0, berlin)
	}

	tests := []struct {
		name   string
		rule   string
		anchor time.Time
		after  time.Time // zero means the anchor itself
		want   time.Time // zero means no occurrence
	}{
		// Every N days.
		{"daily", "FREQ=DAILY", d(2026, 10, 22, 9, 0), time.Time{}, d(2026, 10, 23, 9, 0)},
		{"every 3 days", "FREQ=DAILY;INTERVAL=3", d(2026, 10, 22, 9, 0), time.Time{}, d(2026, 10, 25, 9, 0)},
		{"every 3 days stays on grid when catching up", "FREQ=DAILY;INTERVAL=3", d(2026, 10, 1, 9, 0), d(2026, 10, 22, 12, 0), d(2026, 10, 25, 9, 0)},
		{"catch up on the reference day itself", "FREQ=DAILY", d(2026, 10, 1, 18, 0), d(2026, 10, 22, 12, 0), d(2026, 10, 22, 18, 0)},
		{"daily across month end", "FREQ=DAILY;INTERVAL=2", d(2026, 1, 31, 8, 0), time.Time{}, d(2026, 2, 2, 8, 0)},
		{"daily across leap day", "FREQ=DAILY", d(2028, 2, 28, 8, 0), time.Time{}, d(2028, 2, 29, 8, 0)},
		{"daily keeps wall clock across DST start", "FREQ=DAILY", d(2026, 3, 28, 9, 30), time.Time{}, d(2026, 3, 29, 9, 30)},
		{"daily keeps wall clock across DST end", "FREQ=DAILY", d(2026, 10, 24, 9, 30), time.Time{}, d(2026, 10, 25, 9, 30)},
		{"daily filtered to weekdays skips the weekend", "FREQ=DAILY;BYDAY=MO,TU,WE,TH,FR", d(2026, 10, 23, 9, 0), time.Time{}, d(2026, 10, 26, 9, 0)},
		{"daily filtered by month", "FREQ=DAILY;BYMONTH=12", d(2026, 10, 22, 9, 0), time.Time{}, d(2026, 12, 1, 9, 0)},
		{"daily filtered by month day", "FREQ=DAILY;BYMONTHDAY=1,15", d(2026, 10, 2, 9, 0), time.Time{}, d(2026, 10, 15, 9, 0)},
		{"daily filter that never matches", "FREQ=DAILY;INTERVAL=7;BYDAY=TU", d(2026, 10, 19, 9, 0), time.Time{}, time.Time{}},

		// Weekly and specific weekdays. 2026-10-22 is a Thursday.
		{"weekly on the anchor weekday", "FREQ=WEEKLY", d(2026, 10, 22, 9, 0), time.Time{}, d(2026, 10, 29, 9, 0)},
		{"every 2 weeks", "FREQ=WEEKLY;INTERVAL=2", d(2026, 10, 22, 9, 0), time.Time{}, d(2026, 11, 5, 9, 0)},
		{"weekly mon+fri from thu", "FREQ=WEEKLY;BYDAY=MO,FR", d(2026, 10, 22, 9, 0), time.Time{}, d(2026, 10, 23, 9, 0)},
		{"weekly mon+fri from fri", "FREQ=WEEKLY;BYDAY=MO,FR", d(2026, 10, 23, 9, 0), time.Time{}, d(2026, 10, 26, 9, 0)},
		{"biweekly mon+fri from fri skips a week", "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,FR", d(2026, 10, 23, 9, 0), time.Time{}, d(2026, 11, 2, 9, 0)},
		{"biweekly mon+fri catching up stays on the anchor's weeks", "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO,FR", d(2026, 10, 23, 9, 0), d(2026, 11, 3, 12, 0), d(2026, 11, 6, 9, 0)},
		{"biweekly catching up into an off week", "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO", d(2026, 10, 19, 9, 0), d(2026, 11, 10, 12, 0), d(2026, 11, 16, 9, 0)},
		{"workdays from fri", "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR", d(2026, 10, 23, 17, 0), time.Time{}, d(2026, 10, 26, 17, 0)},
		{"workdays from sat (anchor off the rule)", "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR", d(2026, 10, 24, 17, 0), time.Time{}, d(2026, 10, 26, 17, 0)},
		{"weekends", "FREQ=WEEKLY;BYDAY=SA,SU", d(2026, 10, 22, 10, 0), time.Time{}, d(2026, 10, 24, 10, 0)},
		{"week start sunday changes biweekly grouping", "FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,MO;WKST=SU", d(2026, 10, 25, 9, 0), time.Time{}, d(2026, 10, 26, 9, 0)},
		{"week start monday biweekly grouping", "FREQ=WEEKLY;INTERVAL=2;BYDAY=SU,MO", d(2026, 10, 25, 9, 0), time.Time{}, d(2026, 11, 2, 9, 0)},
		{"weekly limited to a month", "FREQ=WEEKLY;BYMONTH=1", d(2026, 10, 22, 9, 0), time.Time{}, d(2027, 1, 7, 9, 0)},
		{"weekly across year end", "FREQ=WEEKLY", d(2026, 12, 29, 9, 0), time.Time{}, d(2027, 1, 5, 9, 0)},

		// Monthly.
		{"monthly on the anchor day", "FREQ=MONTHLY", d(2026, 10, 22, 9, 0), time.Time{}, d(2026, 11, 22, 9, 0)},
		{"monthly on the 31st skips short months", "FREQ=MONTHLY", d(2026, 1, 31, 9, 0), time.Time{}, d(2026, 3, 31, 9, 0)},
		{"monthly on the 30th skips february", "FREQ=MONTHLY", d(2026, 1, 30, 9, 0), time.Time{}, d(2026, 3, 30, 9, 0)},
		{"every 3 months", "FREQ=MONTHLY;INTERVAL=3", d(2026, 11, 15, 9, 0), time.Time{}, d(2027, 2, 15, 9, 0)},
		{"every 3 months catching up", "FREQ=MONTHLY;INTERVAL=3", d(2026, 1, 15, 9, 0), d(2026, 10, 22, 0, 0), d(2027, 1, 15, 9, 0)},
		{"every 3 months catching up within the period", "FREQ=MONTHLY;INTERVAL=3", d(2026, 1, 15, 9, 0), d(2026, 10, 1, 0, 0), d(2026, 10, 15, 9, 0)},
		{"last day of month", "FREQ=MONTHLY;BYMONTHDAY=-1", d(2026, 1, 31, 9, 0), time.Time{}, d(2026, 2, 28, 9, 0)},
		{"last day of month in a leap year", "FREQ=MONTHLY;BYMONTHDAY=-1", d(2028, 1, 31, 9, 0), time.Time{}, d(2028, 2, 29, 9, 0)},
		{"second to last day", "FREQ=MONTHLY;BYMONTHDAY=-2", d(2026, 4, 29, 9, 0), time.Time{}, d(2026, 5, 30, 9, 0)},
		{"1st and 15th", "FREQ=MONTHLY;BYMONTHDAY=1,15", d(2026, 10, 1, 9, 0), time.Time{}, d(2026, 10, 15, 9, 0)},
		{"1st and 15th wraps to next month", "FREQ=MONTHLY;BYMONTHDAY=1,15", d(2026, 10, 15, 9, 0), time.Time{}, d(2026, 11, 1, 9, 0)},
		{"second tuesday", "FREQ=MONTHLY;BYDAY=2TU", d(2026, 10, 13, 9, 0), time.Time{}, d(2026, 11, 10, 9, 0)},
		{"last friday", "FREQ=MONTHLY;BYDAY=-1FR", d(2026, 10, 30, 9, 0), time.Time{}, d(2026, 11, 27, 9, 0)},
		{"fifth monday skips months without one", "FREQ=MONTHLY;BYDAY=5MO", d(2026, 8, 31, 9, 0), time.Time{}, d(2026, 11, 30, 9, 0)},
		{"every monday of the month", "FREQ=MONTHLY;BYDAY=MO", d(2026, 10, 26, 9, 0), time.Time{}, d(2026, 11, 2, 9, 0)},
		{"friday the 13th", "FREQ=MONTHLY;BYDAY=FR;BYMONTHDAY=13", d(2026, 2, 13, 9, 0), time.Time{}, d(2026, 3, 13, 9, 0)},
		{"friday the 13th after march", "FREQ=MONTHLY;BYDAY=FR;BYMONTHDAY=13", d(2026, 3, 13, 9, 0), time.Time{}, d(2026, 11, 13, 9, 0)},
		{"monthly limited to months", "FREQ=MONTHLY;BYMONTH=3,9", d(2026, 3, 5, 9, 0), time.Time{}, d(2026, 9, 5, 9, 0)},
		{"monthly across DST end", "FREQ=MONTHLY", d(2026, 10, 1, 9, 0), time.Time{}, d(2026, 11, 1, 9, 0)},

		// Yearly.
		{"yearly", "FREQ=YEARLY", d(2026, 10, 22, 9, 0), time.Time{}, d(2027, 10, 22, 9, 0)},
		{"every 2 years", "FREQ=YEARLY;INTERVAL=2", d(2026, 10, 22, 9, 0), time.Time{}, d(2028, 10, 22, 9, 0)},
		{"yearly on feb 29 waits for leap year", "FREQ=YEARLY", d(2028, 2, 29, 9, 0), time.Time{}, d(2032, 2, 29, 9, 0)},
		{"yearly in several months", "FREQ=YEARLY;BYMONTH=1,7", d(2026, 1, 10, 9, 0), time.Time{}, d(2026, 7, 10, 9, 0)},
		{"yearly in several months wraps", "FREQ=YEARLY;BYMONTH=1,7", d(2026, 7, 10, 9, 0), time.Time{}, d(2027, 1, 10, 9, 0)},
		{"thanksgiving", "FREQ=YEARLY;BYMONTH=11;BYDAY=4TH", d(2026, 11, 26, 9, 0), time.Time{}, d(2027, 11, 25, 9, 0)},
		{"yearly catching up", "FREQ=YEARLY;INTERVAL=4", d(2000, 5, 1, 9, 0), d(2026, 10, 22, 0, 0), d(2028, 5, 1, 9, 0)},
		{"impossible date", "FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30", d(2026, 1, 1, 9, 0), time.Time{}, time.Time{}},

		// Pathologically old anchors resolve without scanning every period.
		{"ancient daily anchor", "FREQ=DAILY", d(1, 1, 1, 9, 0), d(2026, 10, 22, 12, 0), d(2026, 10, 23, 9, 0)},
		{"ancient weekly anchor", "FREQ=WEEKLY;BYDAY=WE", d(1, 1, 3, 9, 0), d(2026, 10, 22, 12, 0), d(2026, 10, 28, 9, 0)},
		{"ancient monthly anchor", "FREQ=MONTHLY;BYMONTHDAY=-1", d(1, 1, 31, 9, 0), d(2026, 10, 22, 12, 0), d(2026, 10, 31, 9, 0)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r, err := parseRepeatRule(tc.rule)
			require.NoError(t, err)
			after := tc.after
			if after.IsZero() {
				after = tc.anchor
			}
			got, ok := r.nextOccurrence(tc.anchor, after, berlin)
			if tc.want.IsZero() {
				assert.False(t, ok, "expected no occurrence, got %s", got)
				return
			}
			require.True(t, ok)
			assert.True(t, tc.want.Equal(got), "want %s, got %s", tc.want, got.In(berlin))
		})
	}
}

func TestRepeatRuleNextOccurrenceUsesZone(t *testing.T) {
	// 23:30 on a Sunday in Berlin is still Sunday there 22:30 UTC; in
	// Auckland it is already Monday. The weekday the rule sees follows the zone.
	berlin := mustLoc(t, "Europe/Berlin")
	auckland := mustLoc(t, "Pacific/Auckland")
	anchor := time.Date(2026, 10, 25, 23, 30, 0, 0, berlin)
	r, err := parseRepeatRule("FREQ=WEEKLY;BYDAY=MO")
	require.NoError(t, err)

	got, ok := r.nextOccurrence(anchor, anchor, berlin)
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 10, 26, 23, 30, 0, 0, berlin), got)

	got, ok = r.nextOccurrence(anchor, anchor, auckland)
	require.True(t, ok)
	a := anchor.In(auckland)
	assert.Equal(t, time.Date(2026, 11, 2, a.Hour(), a.Minute(), 0, 0, auckland), got)
}

func TestRepeatRuleAllowsOccurrence(t *testing.T) {
	berlin := mustLoc(t, "Europe/Berlin")

	r, err := parseRepeatRule("FREQ=DAILY;UNTIL=20261231")
	require.NoError(t, err)
	assert.True(t, r.allowsOccurrence(time.Date(2026, 12, 31, 23, 59, 0, 0, berlin), berlin), "a date UNTIL includes the whole day")
	assert.False(t, r.allowsOccurrence(time.Date(2027, 1, 1, 0, 0, 0, 0, berlin), berlin))

	r, err = parseRepeatRule("FREQ=DAILY;UNTIL=20261231T120000Z")
	require.NoError(t, err)
	assert.True(t, r.allowsOccurrence(time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC), berlin))
	assert.False(t, r.allowsOccurrence(time.Date(2026, 12, 31, 12, 0, 1, 0, time.UTC), berlin))

	r, err = parseRepeatRule("FREQ=DAILY")
	require.NoError(t, err)
	assert.True(t, r.allowsOccurrence(time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC), berlin))
}

func TestSetTaskDatesRepeatRule(t *testing.T) {
	berlin := mustLoc(t, "Europe/Berlin")
	d := func(y int, m time.Month, day, h, min int) time.Time {
		return time.Date(y, m, day, h, min, 0, 0, berlin)
	}
	// Thursday afternoon.
	now := d(2026, 10, 22, 15, 0)

	t.Run("moves every date by whole days and reopens", func(t *testing.T) {
		old := &Task{
			Done:       false,
			RepeatRule: "FREQ=WEEKLY;BYDAY=MO,TH",
			DueDate:    d(2026, 10, 22, 18, 0),
			StartDate:  d(2026, 10, 22, 17, 0),
			EndDate:    d(2026, 10, 22, 19, 0),
			Reminders:  []*TaskReminder{{Reminder: d(2026, 10, 22, 17, 45)}},
		}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.False(t, next.Done)
		assert.Equal(t, d(2026, 10, 26, 18, 0), next.DueDate)
		assert.Equal(t, d(2026, 10, 26, 17, 0), next.StartDate)
		assert.Equal(t, d(2026, 10, 26, 19, 0), next.EndDate)
		assert.Equal(t, d(2026, 10, 26, 17, 45), next.Reminders[0].Reminder)
		assert.Equal(t, "FREQ=WEEKLY;BYDAY=MO,TH", next.RepeatRule)
	})

	t.Run("completing early moves to the occurrence after the due date", func(t *testing.T) {
		old := &Task{RepeatRule: "FREQ=DAILY", DueDate: d(2026, 10, 24, 9, 0)}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.Equal(t, d(2026, 10, 25, 9, 0), next.DueDate)
	})

	t.Run("overdue tasks skip occurrences in the past", func(t *testing.T) {
		old := &Task{RepeatRule: "FREQ=WEEKLY;INTERVAL=2;BYDAY=MO", DueDate: d(2026, 9, 7, 9, 0)}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.Equal(t, d(2026, 11, 2, 9, 0), next.DueDate)
	})

	t.Run("dates keep their wall clock across DST", func(t *testing.T) {
		old := &Task{
			RepeatRule: "FREQ=DAILY",
			DueDate:    d(2026, 10, 24, 9, 0),
			StartDate:  d(2026, 10, 24, 8, 0),
		}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, d(2026, 10, 24, 8, 30), berlin))
		assert.Equal(t, d(2026, 10, 25, 9, 0), next.DueDate)
		assert.Equal(t, d(2026, 10, 25, 8, 0), next.StartDate)
	})

	t.Run("after completion counts from the completion day", func(t *testing.T) {
		old := &Task{
			RepeatRule: "FREQ=DAILY;INTERVAL=3",
			RepeatMode: TaskRepeatModeFromCurrentDate,
			DueDate:    d(2026, 10, 10, 9, 0),
			StartDate:  d(2026, 10, 10, 8, 0),
		}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.Equal(t, d(2026, 10, 25, 9, 0), next.DueDate)
		assert.Equal(t, d(2026, 10, 25, 8, 0), next.StartDate)
	})

	t.Run("after completion when completed early", func(t *testing.T) {
		old := &Task{
			RepeatRule: "FREQ=WEEKLY",
			RepeatMode: TaskRepeatModeFromCurrentDate,
			DueDate:    d(2026, 10, 30, 9, 0),
		}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.Equal(t, d(2026, 10, 29, 9, 0), next.DueDate)
	})

	t.Run("after completion monthly", func(t *testing.T) {
		old := &Task{
			RepeatRule: "FREQ=MONTHLY;INTERVAL=2",
			RepeatMode: TaskRepeatModeFromCurrentDate,
			DueDate:    d(2026, 1, 3, 0, 0),
		}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.Equal(t, d(2026, 12, 22, 0, 0), next.DueDate)
	})

	t.Run("count counts down and ends the series", func(t *testing.T) {
		task := &Task{RepeatRule: "FREQ=DAILY;COUNT=3", DueDate: d(2026, 10, 22, 18, 0)}
		var dues []time.Time
		for range 5 {
			next := &Task{Done: true}
			if !setTaskDatesRepeatRule(task, next, now, berlin) {
				break
			}
			dues = append(dues, next.DueDate)
			task = &Task{RepeatRule: next.RepeatRule, DueDate: next.DueDate}
		}
		assert.Equal(t, []time.Time{d(2026, 10, 23, 18, 0), d(2026, 10, 24, 18, 0)}, dues)
		assert.Equal(t, "FREQ=DAILY;COUNT=1", task.RepeatRule)
		assert.True(t, repeatSeriesEndsOnCompletion(task, now, berlin))
	})

	t.Run("until allows occurrences up to it", func(t *testing.T) {
		old := &Task{RepeatRule: "FREQ=WEEKLY;UNTIL=20261029", DueDate: d(2026, 10, 22, 9, 0)}
		assert.False(t, repeatSeriesEndsOnCompletion(old, now, berlin))
	})

	t.Run("until stops before the next occurrence", func(t *testing.T) {
		old := &Task{RepeatRule: "FREQ=WEEKLY;UNTIL=20261028", DueDate: d(2026, 10, 22, 9, 0)}
		assert.True(t, repeatSeriesEndsOnCompletion(old, now, berlin))
	})

	t.Run("until includes its own day", func(t *testing.T) {
		old := &Task{RepeatRule: "FREQ=WEEKLY;UNTIL=20261029", DueDate: d(2026, 10, 22, 21, 0)}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.Equal(t, d(2026, 10, 29, 21, 0), next.DueDate)
	})

	t.Run("without dates the task just reopens", func(t *testing.T) {
		old := &Task{RepeatRule: "FREQ=DAILY;COUNT=2"}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.False(t, next.Done)
		assert.True(t, next.DueDate.IsZero())
		assert.Equal(t, "FREQ=DAILY;COUNT=1", next.RepeatRule)
	})

	t.Run("without dates past until stays done", func(t *testing.T) {
		old := &Task{RepeatRule: "FREQ=DAILY;UNTIL=20261001"}
		next := &Task{Done: true}
		assert.False(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.True(t, next.Done)
	})

	t.Run("reminders anchor a task without dates", func(t *testing.T) {
		old := &Task{
			RepeatRule: "FREQ=MONTHLY;BYMONTHDAY=-1",
			Reminders:  []*TaskReminder{{Reminder: d(2026, 10, 31, 9, 0)}, {Reminder: d(2026, 10, 30, 9, 0)}},
		}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		// Anchored on the earliest reminder (Oct 30), next last-day is Oct 31: one day later.
		assert.Equal(t, d(2026, 11, 1, 9, 0), next.Reminders[0].Reminder)
		assert.Equal(t, d(2026, 10, 31, 9, 0), next.Reminders[1].Reminder)
	})

	t.Run("start date anchors a task without a due date", func(t *testing.T) {
		old := &Task{
			RepeatRule: "FREQ=WEEKLY",
			StartDate:  d(2026, 10, 22, 9, 0),
			EndDate:    d(2026, 10, 23, 9, 0),
		}
		next := &Task{Done: true}
		require.True(t, setTaskDatesRepeatRule(old, next, now, berlin))
		assert.Equal(t, d(2026, 10, 29, 9, 0), next.StartDate)
		assert.Equal(t, d(2026, 10, 30, 9, 0), next.EndDate)
		assert.True(t, next.DueDate.IsZero())
	})
}

func TestUpdateDoneWithRepeatRule(t *testing.T) {
	berlin := mustLoc(t, "Europe/Berlin")

	t.Run("rule takes precedence over repeat_after", func(t *testing.T) {
		due := time.Now().In(berlin).AddDate(0, 0, 1).Round(0)
		old := &Task{RepeatRule: "FREQ=WEEKLY", RepeatAfter: 3600, DueDate: due}
		next := &Task{Done: true}
		updateDoneIn(old, next, berlin)
		assert.False(t, next.Done)
		assert.Equal(t, due.AddDate(0, 0, 7), next.DueDate)
		assert.False(t, next.DoneAt.IsZero())
	})

	t.Run("finished series stays done", func(t *testing.T) {
		old := &Task{RepeatRule: "FREQ=DAILY;COUNT=1", DueDate: time.Now()}
		next := &Task{Done: true, Description: `<ul data-type="taskList"><li data-checked="true">x</li></ul>`}
		assert.True(t, updateDoneIn(old, next, berlin))
		assert.True(t, next.Done)
		assert.Contains(t, next.Description, `data-checked="true"`, "a finished series keeps its checklist")
	})
}
