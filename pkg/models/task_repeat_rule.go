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
	"slices"
	"strconv"
	"strings"
	"time"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// Repeat rules are a subset of RFC 5545 RRULE: everything TickTick-style repeats
// need (every N days/weeks/months/years, weekdays, nth weekday of the month, last
// day of the month, until/count), and nothing that needs open-ended expansion.

const (
	maxRepeatRuleLength = 500
	maxRepeatInterval   = 999
	maxRepeatCount      = 99999
	// Upper bound on periods scanned for the next occurrence once fast-forwarded
	// to the reference date. Rules whose next match lies further out (e.g.
	// BYMONTH=2;BYMONTHDAY=30) are treated as finished.
	maxRepeatScanPeriods = 1000
)

type repeatFreq int

const (
	repeatFreqDaily repeatFreq = iota
	repeatFreqWeekly
	repeatFreqMonthly
	repeatFreqYearly
)

var repeatFreqNames = map[string]repeatFreq{
	"DAILY":   repeatFreqDaily,
	"WEEKLY":  repeatFreqWeekly,
	"MONTHLY": repeatFreqMonthly,
	"YEARLY":  repeatFreqYearly,
}

var repeatWeekdayNames = map[string]time.Weekday{
	"SU": time.Sunday,
	"MO": time.Monday,
	"TU": time.Tuesday,
	"WE": time.Wednesday,
	"TH": time.Thursday,
	"FR": time.Friday,
	"SA": time.Saturday,
}

func repeatWeekdayName(d time.Weekday) string {
	return strings.ToUpper(d.String()[:2])
}

type repeatWeekday struct {
	// N selects the nth such weekday of the month (negative counts from the end); 0 means every one.
	N   int
	Day time.Weekday
}

type repeatRule struct {
	Freq       repeatFreq
	Interval   int
	ByDay      []repeatWeekday
	ByMonthDay []int
	ByMonth    []time.Month
	WeekStart  time.Weekday
	// Count is the number of occurrences left, including the current one. 0 means unlimited.
	Count int
	// Until is the last moment an occurrence may fall on. When UntilIsDate is set, only its
	// calendar date matters and the whole day is included.
	Until       time.Time
	UntilIsDate bool
}

func invalidRepeatRule(rule, reason string) error {
	return ErrInvalidTaskRepeatRule{Rule: rule, Reason: reason}
}

func parseRepeatInts(raw, rule, key string, minVal, maxVal int, allowNegative bool) ([]int, error) {
	parts := strings.Split(raw, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, invalidRepeatRule(rule, key+" must be a list of numbers")
		}
		abs := n
		if n < 0 && allowNegative {
			abs = -n
		}
		if abs < minVal || abs > maxVal {
			return nil, invalidRepeatRule(rule, key+" value "+p+" is out of range")
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out, nil
}

func parseRepeatUntil(raw, rule string) (time.Time, bool, error) {
	if t, err := time.Parse("20060102", raw); err == nil {
		return t, true, nil
	}
	// A floating date-time has no zone to anchor it to; read it as UTC like the Z form.
	for _, layout := range []string{"20060102T150405Z", "20060102T150405"} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t, false, nil
		}
	}
	return time.Time{}, false, invalidRepeatRule(rule, "UNTIL must be a date (YYYYMMDD) or UTC date-time (YYYYMMDDTHHMMSSZ)")
}

// parseRepeatRule parses an RRULE value (with or without the "RRULE:" prefix).
//
//nolint:gocyclo
func parseRepeatRule(raw string) (*repeatRule, error) {
	s := strings.TrimSpace(raw)
	if len(s) > maxRepeatRuleLength {
		return nil, invalidRepeatRule(raw, "the rule is too long")
	}
	if len(s) >= 6 && strings.EqualFold(s[:6], "RRULE:") {
		s = s[6:]
	}
	s = strings.ToUpper(strings.TrimSuffix(s, ";"))
	if s == "" {
		return nil, invalidRepeatRule(raw, "the rule is empty")
	}

	r := &repeatRule{Interval: 1, WeekStart: time.Monday}
	seen := map[string]bool{}
	hasFreq := false
	for _, part := range strings.Split(s, ";") {
		key, value, ok := strings.Cut(part, "=")
		if !ok || value == "" {
			return nil, invalidRepeatRule(raw, "expected KEY=VALUE, got "+strconv.Quote(part))
		}
		if seen[key] {
			return nil, invalidRepeatRule(raw, key+" is given more than once")
		}
		seen[key] = true

		switch key {
		case "FREQ":
			f, ok := repeatFreqNames[value]
			if !ok {
				return nil, invalidRepeatRule(raw, "FREQ must be DAILY, WEEKLY, MONTHLY or YEARLY")
			}
			r.Freq = f
			hasFreq = true
		case "INTERVAL":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > maxRepeatInterval {
				return nil, invalidRepeatRule(raw, "INTERVAL must be between 1 and "+strconv.Itoa(maxRepeatInterval))
			}
			r.Interval = n
		case "COUNT":
			n, err := strconv.Atoi(value)
			if err != nil || n < 1 || n > maxRepeatCount {
				return nil, invalidRepeatRule(raw, "COUNT must be between 1 and "+strconv.Itoa(maxRepeatCount))
			}
			r.Count = n
		case "UNTIL":
			until, isDate, err := parseRepeatUntil(value, raw)
			if err != nil {
				return nil, err
			}
			r.Until, r.UntilIsDate = until, isDate
		case "WKST":
			d, ok := repeatWeekdayNames[value]
			if !ok {
				return nil, invalidRepeatRule(raw, "WKST must be a weekday (MO, TU, ...)")
			}
			r.WeekStart = d
		case "BYDAY":
			for _, item := range strings.Split(value, ",") {
				if len(item) < 2 {
					return nil, invalidRepeatRule(raw, "invalid BYDAY value "+strconv.Quote(item))
				}
				d, ok := repeatWeekdayNames[item[len(item)-2:]]
				if !ok {
					return nil, invalidRepeatRule(raw, "invalid BYDAY value "+strconv.Quote(item))
				}
				wd := repeatWeekday{Day: d}
				if prefix := item[:len(item)-2]; prefix != "" {
					n, err := strconv.Atoi(prefix)
					if err != nil || n == 0 || n < -5 || n > 5 {
						return nil, invalidRepeatRule(raw, "BYDAY ordinals must be between -5 and 5")
					}
					wd.N = n
				}
				if !slices.Contains(r.ByDay, wd) {
					r.ByDay = append(r.ByDay, wd)
				}
			}
		case "BYMONTHDAY":
			days, err := parseRepeatInts(value, raw, key, 1, 31, true)
			if err != nil {
				return nil, err
			}
			r.ByMonthDay = days
		case "BYMONTH":
			months, err := parseRepeatInts(value, raw, key, 1, 12, false)
			if err != nil {
				return nil, err
			}
			for _, m := range months {
				r.ByMonth = append(r.ByMonth, time.Month(m))
			}
		default:
			return nil, invalidRepeatRule(raw, key+" is not supported")
		}
	}

	if !hasFreq {
		return nil, invalidRepeatRule(raw, "FREQ is required")
	}
	if r.Count > 0 && !r.Until.IsZero() {
		return nil, invalidRepeatRule(raw, "COUNT and UNTIL cannot be combined")
	}
	hasOrdinal := slices.ContainsFunc(r.ByDay, func(d repeatWeekday) bool { return d.N != 0 })
	switch r.Freq {
	case repeatFreqDaily, repeatFreqWeekly:
		if hasOrdinal {
			return nil, invalidRepeatRule(raw, "BYDAY ordinals need FREQ=MONTHLY or YEARLY")
		}
		if r.Freq == repeatFreqWeekly && len(r.ByMonthDay) > 0 {
			return nil, invalidRepeatRule(raw, "BYMONTHDAY cannot be used with FREQ=WEEKLY")
		}
	case repeatFreqMonthly:
		// Every BYDAY/BYMONTHDAY/BYMONTH combination applies.
	case repeatFreqYearly:
		if len(r.ByDay) > 0 && len(r.ByMonth) == 0 {
			return nil, invalidRepeatRule(raw, "BYDAY with FREQ=YEARLY needs BYMONTH")
		}
	}

	slices.Sort(r.ByMonthDay)
	slices.Sort(r.ByMonth)
	slices.SortFunc(r.ByDay, func(a, b repeatWeekday) int {
		if a.N != b.N {
			return a.N - b.N
		}
		return int(a.Day) - int(b.Day)
	})
	return r, nil
}

// String renders the rule in canonical form, without the "RRULE:" prefix.
func (r *repeatRule) String() string {
	var b strings.Builder
	for name, f := range repeatFreqNames {
		if f == r.Freq {
			b.WriteString("FREQ=" + name)
		}
	}
	if r.Interval > 1 {
		b.WriteString(";INTERVAL=" + strconv.Itoa(r.Interval))
	}
	if len(r.ByDay) > 0 {
		days := make([]string, len(r.ByDay))
		for i, d := range r.ByDay {
			days[i] = repeatWeekdayName(d.Day)
			if d.N != 0 {
				days[i] = strconv.Itoa(d.N) + days[i]
			}
		}
		b.WriteString(";BYDAY=" + strings.Join(days, ","))
	}
	if len(r.ByMonthDay) > 0 {
		days := make([]string, len(r.ByMonthDay))
		for i, d := range r.ByMonthDay {
			days[i] = strconv.Itoa(d)
		}
		b.WriteString(";BYMONTHDAY=" + strings.Join(days, ","))
	}
	if len(r.ByMonth) > 0 {
		months := make([]string, len(r.ByMonth))
		for i, m := range r.ByMonth {
			months[i] = strconv.Itoa(int(m))
		}
		b.WriteString(";BYMONTH=" + strings.Join(months, ","))
	}
	if r.WeekStart != time.Monday {
		b.WriteString(";WKST=" + repeatWeekdayName(r.WeekStart))
	}
	if r.Count > 0 {
		b.WriteString(";COUNT=" + strconv.Itoa(r.Count))
	}
	if !r.Until.IsZero() {
		if r.UntilIsDate {
			b.WriteString(";UNTIL=" + r.Until.Format("20060102"))
		} else {
			b.WriteString(";UNTIL=" + r.Until.UTC().Format("20060102T150405Z"))
		}
	}
	return b.String()
}

// NormalizeTaskRepeatRule validates a repeat rule and returns its canonical form.
// An empty rule stays empty.
func NormalizeTaskRepeatRule(rule string) (string, error) {
	if strings.TrimSpace(rule) == "" {
		return "", nil
	}
	r, err := parseRepeatRule(rule)
	if err != nil {
		return "", err
	}
	return r.String(), nil
}

// Civil-day arithmetic: days since the Unix epoch for a calendar date, independent of zones and DST.
func repeatDayNumber(y int, m time.Month, d int) int64 {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Unix() / 86400
}

func repeatDateOf(day int64) (int, time.Month, int) {
	return time.Unix(day*86400, 0).UTC().Date()
}

func repeatWeekdayOf(day int64) time.Weekday {
	// 1970-01-01 was a Thursday.
	return time.Weekday((day%7 + 7 + int64(time.Thursday)) % 7)
}

func daysInMonth(y int, m time.Month) int {
	return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}
	return q
}

func (r *repeatRule) matchesMonthDay(d, n int) bool {
	for _, md := range r.ByMonthDay {
		if md == d || (md < 0 && n+md+1 == d) {
			return true
		}
	}
	return false
}

func (r *repeatRule) matchesWeekday(y int, m time.Month, d, n int) bool {
	wd := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).Weekday()
	for _, bd := range r.ByDay {
		if bd.Day != wd {
			continue
		}
		switch {
		case bd.N == 0,
			bd.N > 0 && (d-1)/7+1 == bd.N,
			bd.N < 0 && (n-d)/7+1 == -bd.N:
			return true
		}
	}
	return false
}

// monthDays lists, in order, the days of a month that a MONTHLY or YEARLY rule hits.
// Without BYDAY/BYMONTHDAY a rule repeats on defaultDay; months too short for it are skipped.
func (r *repeatRule) monthDays(y int, m time.Month, defaultDay int) []int {
	n := daysInMonth(y, m)
	if len(r.ByDay) == 0 && len(r.ByMonthDay) == 0 {
		if defaultDay > n {
			return nil
		}
		return []int{defaultDay}
	}
	var days []int
	for d := 1; d <= n; d++ {
		if len(r.ByMonthDay) > 0 && !r.matchesMonthDay(d, n) {
			continue
		}
		if len(r.ByDay) > 0 && !r.matchesWeekday(y, m, d, n) {
			continue
		}
		days = append(days, d)
	}
	return days
}

// nextOccurrence returns the first occurrence of the series starting at anchor that
// lies strictly after the reference time. Occurrences keep the anchor's wall-clock
// time in loc; Count and Until are not applied here. ok is false when no occurrence
// exists within the scan bound.
//
//nolint:gocyclo
func (r *repeatRule) nextOccurrence(anchor, after time.Time, loc *time.Location) (next time.Time, ok bool) {
	if after.Before(anchor) {
		after = anchor
	}
	a := anchor.In(loc)
	at := func(y int, m time.Month, d int) time.Time {
		return time.Date(y, m, d, a.Hour(), a.Minute(), a.Second(), a.Nanosecond(), loc)
	}
	ay, am, ad := a.Date()
	ry, rm, rd := after.In(loc).Date()
	anchorDay := repeatDayNumber(ay, am, ad)
	refDay := repeatDayNumber(ry, rm, rd)
	interval := int64(r.Interval)

	// Skip whole intervals up to the period holding the reference date, so the
	// scan below stays short no matter how old the anchor is.
	skip := func(periods int64) int64 {
		if periods <= 0 {
			return 0
		}
		return floorDiv(periods, interval) * interval
	}

	switch r.Freq {
	case repeatFreqDaily:
		start := skip(refDay - anchorDay)
		for i := int64(0); i < maxRepeatScanPeriods; i++ {
			y, m, d := repeatDateOf(anchorDay + start + i*interval)
			if len(r.ByMonth) > 0 && !slices.Contains(r.ByMonth, m) ||
				len(r.ByMonthDay) > 0 && !r.matchesMonthDay(d, daysInMonth(y, m)) ||
				len(r.ByDay) > 0 && !r.matchesWeekday(y, m, d, daysInMonth(y, m)) {
				continue
			}
			if t := at(y, m, d); t.After(after) {
				return t, true
			}
		}

	case repeatFreqWeekly:
		weekOf := func(day int64) int64 {
			return day - int64((int(repeatWeekdayOf(day))-int(r.WeekStart)+7)%7)
		}
		anchorWeek := weekOf(anchorDay)
		start := skip((weekOf(refDay) - anchorWeek) / 7)
		for i := int64(0); i < maxRepeatScanPeriods; i++ {
			weekStart := anchorWeek + (start+i*interval)*7
			for off := int64(0); off < 7; off++ {
				day := weekStart + off
				wd := repeatWeekdayOf(day)
				if len(r.ByDay) == 0 && wd != a.Weekday() ||
					len(r.ByDay) > 0 && !slices.ContainsFunc(r.ByDay, func(bd repeatWeekday) bool { return bd.Day == wd }) {
					continue
				}
				y, m, d := repeatDateOf(day)
				if len(r.ByMonth) > 0 && !slices.Contains(r.ByMonth, m) {
					continue
				}
				if t := at(y, m, d); t.After(after) {
					return t, true
				}
			}
		}

	case repeatFreqMonthly:
		anchorMonth := int64(ay)*12 + int64(am) - 1
		start := skip(int64(ry)*12 + int64(rm) - 1 - anchorMonth)
		for i := int64(0); i < maxRepeatScanPeriods; i++ {
			idx := anchorMonth + start + i*interval
			y, m := int(idx/12), time.Month(idx%12+1)
			if len(r.ByMonth) > 0 && !slices.Contains(r.ByMonth, m) {
				continue
			}
			for _, d := range r.monthDays(y, m, ad) {
				if t := at(y, m, d); t.After(after) {
					return t, true
				}
			}
		}

	case repeatFreqYearly:
		months := r.ByMonth
		if len(months) == 0 {
			months = []time.Month{am}
		}
		start := skip(int64(ry - ay))
		for i := int64(0); i < maxRepeatScanPeriods; i++ {
			y := ay + int(start+i*interval)
			for _, m := range months {
				for _, d := range r.monthDays(y, m, ad) {
					if t := at(y, m, d); t.After(after) {
						return t, true
					}
				}
			}
		}
	}

	return time.Time{}, false
}

// allowsOccurrence reports whether t is on or before the rule's UNTIL.
func (r *repeatRule) allowsOccurrence(t time.Time, loc *time.Location) bool {
	if r.Until.IsZero() {
		return true
	}
	if r.UntilIsDate {
		y, m, d := t.In(loc).Date()
		uy, um, ud := r.Until.Date()
		return repeatDayNumber(y, m, d) <= repeatDayNumber(uy, um, ud)
	}
	return !t.After(r.Until)
}

// repeatAnchor is the date a repeat rule is evaluated against: the due date, else the
// start date, else the end date, else the earliest reminder.
func repeatAnchor(t *Task) time.Time {
	for _, d := range []time.Time{t.DueDate, t.StartDate, t.EndDate} {
		if !d.IsZero() {
			return d
		}
	}
	var first time.Time
	for _, r := range t.Reminders {
		if !r.Reminder.IsZero() && (first.IsZero() || r.Reminder.Before(first)) {
			first = r.Reminder
		}
	}
	return first
}

// shiftDays moves t by a number of calendar days in loc, keeping its wall-clock time across DST changes.
func shiftDays(t time.Time, days int, loc *time.Location) time.Time {
	if t.IsZero() {
		return t
	}
	return t.In(loc).AddDate(0, 0, days)
}

// planRepeatRule works out where completing oldTask moves its rule-based series:
// the number of calendar days its dates shift by (0 when it has no dates). With
// TaskRepeatModeFromCurrentDate the series restarts on the completion day ("repeat
// after completion"); otherwise it continues from the old dates, skipping
// occurrences that are already past. ok is false once COUNT or UNTIL ends the series.
func planRepeatRule(oldTask *Task, now time.Time, loc *time.Location) (rule *repeatRule, days int, ok bool) {
	rule, err := parseRepeatRule(oldTask.RepeatRule)
	if err != nil || rule.Count == 1 {
		return nil, 0, false
	}

	anchor := repeatAnchor(oldTask)
	if anchor.IsZero() {
		// Nothing to reschedule; the task just reopens while the series lasts.
		return rule, 0, rule.allowsOccurrence(now, loc)
	}

	from := anchor
	if oldTask.RepeatMode == TaskRepeatModeFromCurrentDate {
		a, n := anchor.In(loc), now.In(loc)
		from = time.Date(n.Year(), n.Month(), n.Day(), a.Hour(), a.Minute(), a.Second(), a.Nanosecond(), loc)
	}
	after := from
	if oldTask.RepeatMode != TaskRepeatModeFromCurrentDate && now.After(after) {
		after = now
	}
	next, found := rule.nextOccurrence(from, after, loc)
	if !found || !rule.allowsOccurrence(next, loc) {
		return nil, 0, false
	}

	ay, am, ad := anchor.In(loc).Date()
	ny, nm, nd := next.Date()
	return rule, int(repeatDayNumber(ny, nm, nd) - repeatDayNumber(ay, am, ad)), true
}

// repeatSeriesEndsOnCompletion reports whether completing t now finishes its repeat rule,
// so the task stays done instead of reopening.
func repeatSeriesEndsOnCompletion(t *Task, now time.Time, loc *time.Location) bool {
	if t.RepeatRule == "" {
		return false
	}
	_, _, ok := planRepeatRule(t, now, loc)
	return !ok
}

// setTaskDatesRepeatRule moves newTask to the next occurrence of oldTask's repeat rule
// and reopens it, counting down COUNT. It returns false, leaving newTask done, once
// the series has ended.
func setTaskDatesRepeatRule(oldTask, newTask *Task, now time.Time, loc *time.Location) bool {
	rule, days, ok := planRepeatRule(oldTask, now, loc)
	if !ok {
		return false
	}

	if !repeatAnchor(oldTask).IsZero() {
		newTask.DueDate = shiftDays(oldTask.DueDate, days, loc)
		newTask.StartDate = shiftDays(oldTask.StartDate, days, loc)
		newTask.EndDate = shiftDays(oldTask.EndDate, days, loc)
		newTask.Reminders = oldTask.Reminders
		for _, r := range newTask.Reminders {
			r.Reminder = shiftDays(r.Reminder, days, loc)
		}
	}

	if rule.Count > 1 {
		rule.Count--
	}
	newTask.RepeatRule = rule.String()
	newTask.Done = false
	return true
}

// repeatLocation is the time zone repeat rules are evaluated in: the acting user's,
// falling back to the server's for link shares and users without one.
func repeatLocation(s *xorm.Session, a web.Auth) *time.Location {
	if u, err := user.GetFromAuth(a); err == nil && u != nil && u.ID > 0 {
		if full, err := user.GetUserByID(s, u.ID); err == nil && full.Timezone != "" {
			if loc, err := time.LoadLocation(full.Timezone); err == nil {
				return loc
			}
		}
	}
	return config.GetTimeZone()
}
