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

package inboundmail

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// isNone reports whether a value asks to clear a field.
func isNone(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "none", "no", "never", "clear", "remove", "-":
		return true
	}
	return false
}

// when is a parsed date: a whole day, or a moment when timed is set.
type when struct {
	at    time.Time
	timed bool
}

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "sun": time.Sunday,
	"monday": time.Monday, "mon": time.Monday,
	"tuesday": time.Tuesday, "tue": time.Tuesday, "tues": time.Tuesday,
	"wednesday": time.Wednesday, "wed": time.Wednesday,
	"thursday": time.Thursday, "thu": time.Thursday, "thurs": time.Thursday,
	"friday": time.Friday, "fri": time.Friday,
	"saturday": time.Saturday, "sat": time.Saturday,
}

var (
	clockRe = regexp.MustCompile(`^(\d{1,2})(?::(\d{2}))?\s*(am|pm)?$`)
	dateRe  = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})$`)
)

const dateHelp = "use 2026-11-20, 2026-11-20 17:00, today, tomorrow 9am or a weekday like friday 14:30"

// parseWhen reads a date or date and time in loc. Accepted: an ISO date, an ISO
// date-time (with or without offset), today, tomorrow or a weekday name, each
// optionally followed by a time like 17:00, 5pm or 9:30am.
func parseWhen(raw string, now time.Time, loc *time.Location) (when, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	v = strings.TrimSuffix(strings.TrimPrefix(v, "on "), ".")
	v = strings.TrimPrefix(strings.TrimPrefix(v, "next "), "this ")
	if v == "" {
		return when{}, fmt.Errorf("missing date: %s", dateHelp)
	}
	upper := strings.ToUpper(strings.TrimSpace(raw))
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04Z07:00", "2006-01-02 15:04:05Z07:00", "2006-01-02 15:04Z07:00"} {
		if t, err := time.Parse(layout, upper); err == nil {
			return when{at: t.In(loc), timed: true}, nil
		}
	}

	// Split into a day part and an optional time part.
	dayPart, clockPart := v, ""
	if i := strings.IndexAny(v, " t"); i > 0 && dateRe.MatchString(v[:i]) {
		dayPart, clockPart = v[:i], strings.TrimSpace(v[i+1:])
	} else if i := strings.Index(v, " "); i > 0 {
		dayPart, clockPart = v[:i], strings.TrimSpace(v[i+1:])
	}
	clockPart = strings.TrimSpace(strings.TrimPrefix(clockPart, "at "))

	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	var day time.Time
	switch {
	case dayPart == "today":
		day = today
	case dayPart == "tomorrow":
		day = today.AddDate(0, 0, 1)
	case dateRe.MatchString(dayPart):
		t, err := time.ParseInLocation("2006-01-02", dayPart, loc)
		if err != nil {
			return when{}, fmt.Errorf("%q is not a valid date: %s", raw, dateHelp)
		}
		day = t
	default:
		wd, ok := weekdays[dayPart]
		if !ok {
			return when{}, fmt.Errorf("%q is not a date I understand: %s", raw, dateHelp)
		}
		diff := (int(wd) - int(today.Weekday()) + 7) % 7
		day = today.AddDate(0, 0, diff)
	}
	if clockPart == "" {
		return when{at: day}, nil
	}
	m := clockRe.FindStringSubmatch(strings.ReplaceAll(clockPart, ".", ":"))
	if m == nil {
		return when{}, fmt.Errorf("%q is not a time I understand: use 17:00, 5pm or 9:30am", clockPart)
	}
	hour, _ := strconv.Atoi(m[1])
	minute := 0
	if m[2] != "" {
		minute, _ = strconv.Atoi(m[2])
	}
	switch m[3] {
	case "am":
		if hour == 12 {
			hour = 0
		}
	case "pm":
		if hour < 12 {
			hour += 12
		}
	}
	if hour > 23 || minute > 59 {
		return when{}, fmt.Errorf("%q is not a valid time", clockPart)
	}
	return when{at: time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, loc), timed: true}, nil
}

var durationPartRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(days?|d|hours?|hrs?|h|minutes?|mins?|m)`)

// parseDuration reads lengths like 45m, 1h30m, 1.5h, 2 hours, 90 minutes or 3d.
func parseDuration(raw string) (time.Duration, error) {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" {
		return 0, errors.New("missing duration: use 45m, 1h30m or 2d")
	}
	if n, err := strconv.Atoi(v); err == nil {
		// A bare number is minutes.
		v = strconv.Itoa(n) + "m"
	}
	matches := durationPartRe.FindAllStringSubmatchIndex(v, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("%q is not a duration I understand: use 45m, 1h30m or 2d", raw)
	}
	var total time.Duration
	rest := v
	for _, m := range matches {
		n, _ := strconv.ParseFloat(v[m[2]:m[3]], 64)
		unit := time.Minute
		switch v[m[4]] {
		case 'd':
			unit = 24 * time.Hour
		case 'h':
			unit = time.Hour
		}
		total += time.Duration(n * float64(unit))
		rest = strings.Replace(rest, v[m[0]:m[1]], "", 1)
	}
	if strings.Trim(rest, " ,and") != "" {
		return 0, fmt.Errorf("%q is not a duration I understand: use 45m, 1h30m or 2d", raw)
	}
	if total <= 0 {
		return 0, errors.New("the duration must be longer than zero")
	}
	return total, nil
}

var rruleDays = map[time.Weekday]string{
	time.Monday: "MO", time.Tuesday: "TU", time.Wednesday: "WE", time.Thursday: "TH",
	time.Friday: "FR", time.Saturday: "SA", time.Sunday: "SU",
}

var (
	everyNRe   = regexp.MustCompile(`^every\s+(\d+)\s+(day|week|month|year)s?(?:\s+on\s+(.+))?$`)
	untilRe    = regexp.MustCompile(`\s+until\s+(\d{4}-\d{2}-\d{2})$`)
	countRe    = regexp.MustCompile(`\s+(?:for\s+)?(\d+)\s+times$`)
	repeatHelp = "use daily, weekly, monthly, yearly, weekdays, every 2 weeks, every monday and thursday, or an RRULE like FREQ=MONTHLY;BYMONTHDAY=1 (optionally followed by until 2026-12-31 or 10 times)"
)

// parseRepeat turns a repeat description into an RFC 5545 RRULE.
func parseRepeat(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", fmt.Errorf("missing repeat: %s", repeatHelp)
	}
	if up := strings.ToUpper(v); strings.Contains(up, "FREQ=") {
		return strings.TrimPrefix(strings.ReplaceAll(up, " ", ""), "RRULE:"), nil
	}
	v = strings.ToLower(strings.TrimSuffix(v, "."))

	suffix := ""
	if m := untilRe.FindStringSubmatch(v); m != nil {
		suffix = ";UNTIL=" + strings.ReplaceAll(m[1], "-", "") + "T235959Z"
		v = strings.TrimSpace(v[:len(v)-len(m[0])])
	} else if m := countRe.FindStringSubmatch(v); m != nil {
		suffix = ";COUNT=" + m[1]
		v = strings.TrimSpace(v[:len(v)-len(m[0])])
	}

	base := ""
	switch v {
	case "daily", "every day", "each day":
		base = "FREQ=DAILY"
	case "weekly", "every week", "each week":
		base = "FREQ=WEEKLY"
	case "biweekly", "fortnightly", "every other week":
		base = "FREQ=WEEKLY;INTERVAL=2"
	case "monthly", "every month", "each month":
		base = "FREQ=MONTHLY"
	case "yearly", "annually", "every year", "each year":
		base = "FREQ=YEARLY"
	case "weekdays", "every weekday", "on weekdays", "workdays", "every workday":
		base = "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR"
	case "weekends", "every weekend", "on weekends":
		base = "FREQ=WEEKLY;BYDAY=SA,SU"
	}
	if base == "" {
		if m := everyNRe.FindStringSubmatch(v); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n < 1 {
				return "", fmt.Errorf("%q: the interval must be at least 1", raw)
			}
			base = "FREQ=" + strings.ToUpper(map[string]string{"day": "daily", "week": "weekly", "month": "monthly", "year": "yearly"}[m[2]])
			if n > 1 {
				base += ";INTERVAL=" + m[1]
			}
			if m[3] != "" {
				if m[2] != "week" {
					return "", fmt.Errorf("%q: weekdays only go with weeks; for anything else use an RRULE", raw)
				}
				days, err := parseWeekdayList(m[3])
				if err != nil {
					return "", fmt.Errorf("%q: %w", raw, err)
				}
				base += ";BYDAY=" + days
			}
		} else if rest, ok := strings.CutPrefix(v, "every "); ok {
			days, err := parseWeekdayList(rest)
			if err != nil {
				return "", fmt.Errorf("%q is not a repeat I understand: %s", raw, repeatHelp)
			}
			base = "FREQ=WEEKLY;BYDAY=" + days
		}
	}
	if base == "" {
		return "", fmt.Errorf("%q is not a repeat I understand: %s", raw, repeatHelp)
	}
	return base + suffix, nil
}

func parseWeekdayList(raw string) (string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == '/' || r == '&' })
	var days []string
	seen := map[string]bool{}
	for _, p := range parts {
		if p == "and" || p == "on" {
			continue
		}
		wd, ok := weekdays[strings.TrimSuffix(p, "s")]
		if !ok {
			wd, ok = weekdays[p]
		}
		if !ok {
			return "", fmt.Errorf("%q is not a weekday", p)
		}
		if d := rruleDays[wd]; !seen[d] {
			seen[d] = true
			days = append(days, d)
		}
	}
	if len(days) == 0 {
		return "", errors.New("no weekdays given")
	}
	return strings.Join(days, ","), nil
}

// parsePriority maps none, low, medium and high (or 0-3) onto the stored priority.
func parsePriority(raw string) (int64, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none", "no", "0", "":
		return 0, nil
	case "low", "1":
		return 1, nil
	case "medium", "normal", "2":
		return 2, nil
	case "high", "urgent", "important", "3":
		return 3, nil
	}
	return 0, fmt.Errorf("%q is not a priority: use none, low, medium or high", raw)
}
