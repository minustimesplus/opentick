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

// Package icalfeed parses read-only iCalendar (ICS) feeds and expands their
// events, including recurring ones, into concrete occurrences.
package icalfeed

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	ics "github.com/arran4/golang-ical"
	"github.com/teambition/rrule-go"
)

const (
	// maxOccurrencesPerEvent caps how many instances a single recurring
	// event may contribute to one expansion.
	maxOccurrencesPerEvent = 1000
	// maxIterationsPerEvent caps the work spent walking a recurrence rule
	// from its DTSTART up to the requested range (e.g. FREQ=MINUTELY since 2005).
	maxIterationsPerEvent = 200000
	// MaxEvents caps the number of occurrences returned from one expansion.
	MaxEvents = 5000
)

// Event is one concrete occurrence of a calendar event.
type Event struct {
	ID          string    `json:"id" doc:"Identifier of this occurrence, unique within the feed: the event UID plus the occurrence start."`
	UID         string    `json:"uid" doc:"The UID of the event in the feed. Shared by all occurrences of a recurring event."`
	Title       string    `json:"title" doc:"The event summary."`
	Description string    `json:"description" doc:"The event description as plain text."`
	Location    string    `json:"location" doc:"The event location as plain text."`
	URL         string    `json:"url" doc:"The URL attached to the event, if any."`
	Start       time.Time `json:"start" doc:"Start of the occurrence. For all-day events this is midnight in the requested timezone."`
	End         time.Time `json:"end" doc:"End of the occurrence (exclusive). For all-day events this is midnight after the last day in the requested timezone."`
	AllDay      bool      `json:"all_day" doc:"Whether this is an all-day event."`
	Recurring   bool      `json:"recurring" doc:"Whether this occurrence belongs to a recurring event."`
}

// stamp is a parsed iCalendar date or date-time. All-day and floating values
// have no fixed instant: t then holds their wall-clock value in UTC and is
// pinned to a timezone only when expanded for a viewer.
type stamp struct {
	t        time.Time
	allDay   bool
	floating bool
}

func (s stamp) wall() bool { return s.allDay || s.floating }

// in resolves the stamp to an instant for a viewer in loc.
func (s stamp) in(loc *time.Location) time.Time {
	if !s.wall() {
		return s.t
	}
	return wallIn(s.t, loc)
}

// relativeTo returns s in the time base of master, so the two can be compared
// (EXDATE and RECURRENCE-ID against generated occurrences).
func (s stamp) relativeTo(master stamp, loc *time.Location) time.Time {
	switch {
	case master.wall() && s.wall():
		return s.t
	case master.wall():
		return toWall(s.t, loc)
	case s.wall():
		return wallIn(s.t, master.t.Location())
	default:
		return s.t
	}
}

// wallIn interprets the wall clock of t (stored in UTC) in loc.
func wallIn(t time.Time, loc *time.Location) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, loc)
}

// toWall converts an instant to its wall clock in loc, stored as UTC.
func toWall(t time.Time, loc *time.Location) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC)
}

type sourceEvent struct {
	uid         string
	summary     string
	description string
	location    string
	url         string

	start    stamp
	duration time.Duration
	// endDays is set instead of duration for all-day events, so that DST
	// changes do not shift the end off midnight.
	endDays int

	rrules  []string
	rdates  []stamp
	exdates []stamp

	recurrenceID *stamp
	cancelled    bool
}

// Feed is a parsed calendar feed, ready to be expanded for any time range.
type Feed struct {
	// Name is the calendar's display name (X-WR-CALNAME or NAME), if the feed sets one.
	Name   string
	events []*sourceEvent
}

// ErrNotACalendar is returned when the data does not contain a VCALENDAR.
var ErrNotACalendar = errors.New("the data is not an iCalendar feed")

// Parse reads an iCalendar feed. Malformed lines are skipped rather than
// failing the whole feed, as real-world feeds are often not RFC-compliant.
// Individual events that cannot be understood are dropped.
func Parse(data []byte) (*Feed, error) {
	if !bytes.Contains(data, []byte("BEGIN:VCALENDAR")) {
		return nil, ErrNotACalendar
	}
	cal, err := ics.ParseCalendarWithOptions(
		bytes.NewReader(data),
		ics.WithPropertyParser(ics.FallbackParser(ics.LooseParser, ics.SkipPropertyParser)),
	)
	if err != nil {
		return nil, fmt.Errorf("could not parse calendar: %w", err)
	}

	f := &Feed{}
	for _, p := range cal.CalendarProperties {
		switch p.IANAToken {
		case string(ics.PropertyXWRCalName):
			f.Name = ics.FromText(p.Value)
		case string(ics.PropertyName):
			if f.Name == "" {
				f.Name = ics.FromText(p.Value)
			}
		}
	}

	for _, ev := range cal.Events() {
		se, err := parseEvent(ev)
		if err != nil {
			continue
		}
		f.events = append(f.events, se)
	}
	return f, nil
}

func textProp(ev *ics.VEvent, p ics.ComponentProperty) string {
	prop := ev.GetProperty(p)
	if prop == nil {
		return ""
	}
	return strings.TrimSpace(ics.FromText(prop.Value))
}

func parseEvent(ev *ics.VEvent) (*sourceEvent, error) {
	startProp := ev.GetProperty(ics.ComponentPropertyDtStart)
	if startProp == nil {
		return nil, errors.New("event has no DTSTART")
	}
	start, err := parseStamp(startProp.Value, startProp.ICalParameters)
	if err != nil {
		return nil, err
	}

	se := &sourceEvent{
		uid:         textProp(ev, ics.ComponentPropertyUniqueId),
		summary:     textProp(ev, ics.ComponentPropertySummary),
		description: textProp(ev, ics.ComponentPropertyDescription),
		location:    textProp(ev, ics.ComponentPropertyLocation),
		url:         textProp(ev, ics.ComponentPropertyUrl),
		start:       start,
		cancelled:   strings.EqualFold(textProp(ev, ics.ComponentPropertyStatus), string(ics.ObjectStatusCancelled)),
	}

	switch {
	case ev.GetProperty(ics.ComponentPropertyDtEnd) != nil:
		endProp := ev.GetProperty(ics.ComponentPropertyDtEnd)
		end, err := parseStamp(endProp.Value, endProp.ICalParameters)
		if err != nil {
			return nil, err
		}
		if start.allDay {
			se.endDays = int(end.t.Sub(start.t).Hours() / 24)
		} else {
			se.duration = end.relativeTo(start, time.UTC).Sub(start.t)
		}
	case ev.GetProperty(ics.ComponentPropertyDuration) != nil:
		d, err := parseDuration(ev.GetProperty(ics.ComponentPropertyDuration).Value)
		if err != nil {
			return nil, err
		}
		if start.allDay {
			se.endDays = int(d.Hours() / 24)
		} else {
			se.duration = d
		}
	case start.allDay:
		se.endDays = 1
	}
	if start.allDay && se.endDays < 1 {
		se.endDays = 1
	}
	if se.duration < 0 {
		se.duration = 0
	}

	for _, p := range ev.GetProperties(ics.ComponentPropertyRrule) {
		se.rrules = append(se.rrules, strings.TrimSpace(p.Value))
	}
	se.rdates = parseStampList(ev.GetProperties(ics.ComponentPropertyRdate))
	se.exdates = parseStampList(ev.GetProperties(ics.ComponentPropertyExdate))

	if p := ev.GetProperty(ics.ComponentPropertyRecurrenceId); p != nil {
		rid, err := parseStamp(p.Value, p.ICalParameters)
		if err == nil {
			se.recurrenceID = &rid
		}
	}

	return se, nil
}

func parseStampList(props []*ics.IANAProperty) []stamp {
	var out []stamp
	for _, p := range props {
		for _, v := range strings.Split(p.Value, ",") {
			v = strings.TrimSpace(v)
			if v == "" {
				continue
			}
			s, err := parseStamp(v, p.ICalParameters)
			if err != nil {
				// RDATE may also hold a PERIOD; only its start matters here.
				if i := strings.Index(v, "/"); i > 0 {
					s, err = parseStamp(v[:i], p.ICalParameters)
				}
				if err != nil {
					continue
				}
			}
			out = append(out, s)
		}
	}
	return out
}

func firstParam(params map[string][]string, name string) string {
	for k, v := range params {
		if strings.EqualFold(k, name) && len(v) > 0 {
			return strings.Trim(v[0], `"`)
		}
	}
	return ""
}

func parseStamp(value string, params map[string][]string) (stamp, error) {
	value = strings.TrimSpace(value)
	if strings.EqualFold(firstParam(params, "VALUE"), "DATE") || len(value) == 8 {
		if len(value) < 8 {
			return stamp{}, fmt.Errorf("invalid date %q", value)
		}
		t, err := time.ParseInLocation("20060102", value[:8], time.UTC)
		if err != nil {
			return stamp{}, err
		}
		return stamp{t: t, allDay: true}, nil
	}

	if strings.HasSuffix(value, "Z") {
		t, err := time.ParseInLocation("20060102T150405Z", value, time.UTC)
		if err != nil {
			return stamp{}, err
		}
		return stamp{t: t}, nil
	}

	t, err := time.ParseInLocation("20060102T150405", value, time.UTC)
	if err != nil {
		return stamp{}, err
	}
	if tzid := firstParam(params, "TZID"); tzid != "" {
		if loc := resolveTimezone(tzid); loc != nil {
			return stamp{t: wallIn(t, loc)}, nil
		}
	}
	// No TZID, or one we cannot resolve (custom VTIMEZONE names): treat the
	// time as floating, i.e. in the viewer's own timezone.
	return stamp{t: t, floating: true}, nil
}

func resolveTimezone(tzid string) *time.Location {
	if loc := ics.WindowsTimezoneToIANA(tzid); loc != nil {
		return loc
	}
	if loc, err := time.LoadLocation(tzid); err == nil {
		return loc
	}
	// Some producers prefix the IANA name with a vendor path, e.g.
	// "/freeassociation.sourceforge.net/Tzfile/Europe/Berlin".
	parts := strings.Split(strings.Trim(tzid, "/"), "/")
	for i := 1; i < len(parts); i++ {
		if loc, err := time.LoadLocation(strings.Join(parts[i:], "/")); err == nil {
			return loc
		}
	}
	return nil
}

var durationRegex = regexp.MustCompile(`^([+-])?P(?:(\d+)W)?(?:(\d+)D)?(?:T(?:(\d+)H)?(?:(\d+)M)?(?:(\d+)S)?)?$`)

func parseDuration(value string) (time.Duration, error) {
	m := durationRegex.FindStringSubmatch(strings.TrimSpace(value))
	if m == nil {
		return 0, fmt.Errorf("invalid duration %q", value)
	}
	units := []time.Duration{7 * 24 * time.Hour, 24 * time.Hour, time.Hour, time.Minute, time.Second}
	var d time.Duration
	for i, unit := range units {
		if m[i+2] == "" {
			continue
		}
		n, err := strconv.Atoi(m[i+2])
		if err != nil {
			return 0, err
		}
		d += time.Duration(n) * unit
	}
	if m[1] == "-" {
		d = -d
	}
	return d, nil
}

// Expand returns all event occurrences overlapping [from, to), sorted by
// start. All-day and floating events are pinned to loc. At most MaxEvents
// occurrences are returned.
func (f *Feed) Expand(from, to time.Time, loc *time.Location) []*Event {
	if loc == nil {
		loc = time.UTC
	}

	// Instances replaced or cancelled by a RECURRENCE-ID override, per UID.
	overridden := map[string][]stamp{}
	for _, se := range f.events {
		if se.recurrenceID != nil {
			overridden[se.uid] = append(overridden[se.uid], *se.recurrenceID)
		}
	}

	out := []*Event{}
	for _, se := range f.events {
		if se.recurrenceID != nil {
			if !se.cancelled {
				out = appendIfOverlaps(out, se, se.start.t, from, to, loc, true)
			}
			continue
		}
		if se.cancelled {
			continue
		}
		if len(se.rrules) == 0 && len(se.rdates) == 0 {
			out = appendIfOverlaps(out, se, se.start.t, from, to, loc, false)
			continue
		}
		out = append(out, se.expandRecurring(from, to, loc, overridden[se.uid])...)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Start.Equal(out[j].Start) {
			return out[i].Start.Before(out[j].Start)
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > MaxEvents {
		out = out[:MaxEvents]
	}
	return out
}

// occurrence builds the event for an occurrence starting at start, given in
// the event's own time base (see stamp).
func (se *sourceEvent) occurrence(start time.Time, loc *time.Location, recurring bool) *Event {
	s := stamp{t: start, allDay: se.start.allDay, floating: se.start.floating}
	st := s.in(loc)
	var end time.Time
	if se.start.allDay {
		end = stamp{t: start.AddDate(0, 0, se.endDays), allDay: true}.in(loc)
	} else {
		end = stamp{t: start.Add(se.duration), floating: se.start.floating}.in(loc)
	}
	return &Event{
		ID:          fmt.Sprintf("%s#%d", se.uid, st.Unix()),
		UID:         se.uid,
		Title:       se.summary,
		Description: se.description,
		Location:    se.location,
		URL:         se.url,
		Start:       st,
		End:         end,
		AllDay:      se.start.allDay,
		Recurring:   recurring,
	}
}

func overlaps(e *Event, from, to time.Time) bool {
	if e.End.After(e.Start) {
		return e.Start.Before(to) && e.End.After(from)
	}
	// Zero-length events count when their start lies in the range.
	return !e.Start.Before(from) && e.Start.Before(to)
}

func appendIfOverlaps(out []*Event, se *sourceEvent, start, from, to time.Time, loc *time.Location, recurring bool) []*Event {
	e := se.occurrence(start, loc, recurring)
	if overlaps(e, from, to) {
		out = append(out, e)
	}
	return out
}

func (se *sourceEvent) expandRecurring(from, to time.Time, loc *time.Location, overrides []stamp) []*Event {
	// Work in the event's own time base: real instants for zoned events,
	// UTC-stored wall clocks for all-day and floating ones.
	base := se.start.t
	rangeFrom, rangeTo := from, to
	if se.start.wall() {
		rangeFrom, rangeTo = toWall(from, loc), toWall(to, loc)
	}
	// Occurrences starting this long before the range can still overlap it.
	span := se.duration
	if se.start.allDay {
		span = time.Duration(se.endDays) * 24 * time.Hour
	}
	// Leave a day of slack for DST shifts between the time bases.
	earliest := rangeFrom.Add(-span - 24*time.Hour)
	latest := rangeTo.Add(24 * time.Hour)

	set := &rrule.Set{}
	set.DTStart(base)
	for _, r := range se.rrules {
		opt, err := rrule.StrToROptionInLocation(r, base.Location())
		if err != nil {
			continue
		}
		opt.Dtstart = base
		// A wall-time event's UNTIL is parsed as a wall time already; a UTC
		// UNTIL has to be moved into the viewer's wall clock to compare.
		if !opt.Until.IsZero() && se.start.wall() && strings.HasSuffix(untilValue(r), "Z") {
			opt.Until = toWall(opt.Until, loc)
		}
		rule, err := rrule.NewRRule(*opt)
		if err != nil {
			continue
		}
		set.RRule(rule)
		break // Multiple RRULEs are deprecated (RFC 5545) and practically unused.
	}
	if set.GetRRule() == nil {
		// A broken rule still has its first occurrence.
		set.RDate(base)
	}
	for _, rd := range se.rdates {
		set.RDate(rd.relativeTo(se.start, loc))
	}
	skip := map[int64]bool{}
	for _, ex := range se.exdates {
		skip[ex.relativeTo(se.start, loc).Unix()] = true
	}
	for _, o := range overrides {
		skip[o.relativeTo(se.start, loc).Unix()] = true
	}

	var out []*Event
	next := set.Iterator()
	for i := 0; i < maxIterationsPerEvent && len(out) < maxOccurrencesPerEvent; i++ {
		occ, ok := next()
		if !ok || occ.After(latest) {
			break
		}
		if occ.Before(earliest) || skip[occ.Unix()] {
			continue
		}
		out = appendIfOverlaps(out, se, occ, from, to, loc, true)
	}
	return out
}

func untilValue(rule string) string {
	for _, part := range strings.Split(rule, ";") {
		if v, ok := strings.CutPrefix(strings.TrimPrefix(part, "RRULE:"), "UNTIL="); ok {
			return v
		}
	}
	return ""
}
