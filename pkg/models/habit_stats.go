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
	"sort"
	"time"

	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// HabitStats summarizes a habit's history up to a day.
type HabitStats struct {
	HabitID        int64            `json:"habit_id" doc:"The habit these stats belong to."`
	Today          string           `json:"today" doc:"The day the stats are relative to, YYYY-MM-DD."`
	CurrentStreak  int              `json:"current_streak" doc:"Consecutive due days the goal was reached, up to today. Days the habit is not due are skipped; today only breaks the streak once it is over."`
	LongestStreak  int              `json:"longest_streak" doc:"The longest run of consecutive due days the goal was reached."`
	CompletedDays  int              `json:"completed_days" doc:"Due days since the habit started on which the goal was reached."`
	DueDays        int              `json:"due_days" doc:"Days since the habit started on which it was due. Today counts only once it is completed."`
	CompletionRate float64          `json:"completion_rate" doc:"completed_days / due_days, between 0 and 1."`
	TotalCheckins  int              `json:"total_checkins" doc:"All days with a check-in, due or not."`
	TotalValue     int              `json:"total_value" doc:"The sum of all checked-in values."`
	Month          string           `json:"month" doc:"The month of days, YYYY-MM."`
	Days           []*HabitStatsDay `json:"days" doc:"One entry per day of the month."`
}

// HabitStatsDay is one day of a habit's monthly calendar.
type HabitStatsDay struct {
	Date      string `json:"date" doc:"The day, YYYY-MM-DD."`
	Value     int    `json:"value" doc:"How much was checked in that day."`
	Due       bool   `json:"due" doc:"Whether the habit is due that day."`
	Completed bool   `json:"completed" doc:"Whether the goal was reached that day."`
}

// habitHistory answers schedule and completion questions about a habit.
type habitHistory struct {
	habit  *Habit
	values map[string]int
	// The first day the habit counts from: the day it was created, or its
	// first check-in if that is earlier.
	start time.Time
}

func newHabitHistory(h *Habit, checkins []*HabitCheckin) *habitHistory {
	hist := &habitHistory{habit: h, values: make(map[string]int, len(checkins))}
	created := h.Created.UTC()
	hist.start = time.Date(created.Year(), created.Month(), created.Day(), 0, 0, 0, 0, time.UTC)
	for _, c := range checkins {
		hist.values[habitDateKey(c.Day)] = c.Value
		if c.Day.Before(hist.start) {
			hist.start = c.Day
		}
	}
	return hist
}

func (hist *habitHistory) due(day time.Time) bool {
	return hist.habit.Weekdays&(1<<uint(day.Weekday())) != 0
}

func (hist *habitHistory) completed(day time.Time) bool {
	return hist.values[habitDateKey(day)] >= hist.habit.Goal
}

// pending reports whether day is today and still has a chance to be completed.
func (hist *habitHistory) pending(day, today time.Time) bool {
	return day.Equal(today) && !hist.completed(day)
}

func (hist *habitHistory) currentStreak(today time.Time) int {
	streak := 0
	for day := today; !day.Before(hist.start); day = day.AddDate(0, 0, -1) {
		if !hist.due(day) || hist.pending(day, today) {
			continue
		}
		if !hist.completed(day) {
			break
		}
		streak++
	}
	return streak
}

func (hist *habitHistory) stats(today time.Time) *HabitStats {
	st := &HabitStats{HabitID: hist.habit.ID, Today: habitDateKey(today)}
	run := 0
	for day := hist.start; !day.After(today); day = day.AddDate(0, 0, 1) {
		if !hist.due(day) || hist.pending(day, today) {
			continue
		}
		st.DueDays++
		if hist.completed(day) {
			st.CompletedDays++
			run++
			st.LongestStreak = max(st.LongestStreak, run)
		} else {
			run = 0
		}
	}
	st.CurrentStreak = hist.currentStreak(today)
	if st.DueDays > 0 {
		st.CompletionRate = float64(st.CompletedDays) / float64(st.DueDays)
	}
	for _, v := range hist.values {
		st.TotalCheckins++
		st.TotalValue += v
	}
	return st
}

func (hist *habitHistory) month(first time.Time) []*HabitStatsDay {
	days := []*HabitStatsDay{}
	for day := first; day.Month() == first.Month(); day = day.AddDate(0, 0, 1) {
		days = append(days, &HabitStatsDay{
			Date:      habitDateKey(day),
			Value:     hist.values[habitDateKey(day)],
			Due:       hist.due(day),
			Completed: hist.completed(day),
		})
	}
	return days
}

// GetStats returns the habit's streaks, completion rate and the calendar of
// month (YYYY-MM, defaults to the month of today). today defaults to today in
// the user's timezone. Callers must have checked CanRead.
func (h *Habit) GetStats(s *xorm.Session, a web.Auth, today, month string) (*HabitStats, error) {
	h.Today = today
	day, err := h.resolveToday(s, a)
	if err != nil {
		return nil, err
	}

	first := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, time.UTC)
	if month != "" {
		first, err = time.ParseInLocation(habitMonthFormat, month, time.UTC)
		if err != nil {
			return nil, ErrHabitInvalidDate{Date: month}
		}
	}

	checkins, err := h.allCheckins(s)
	if err != nil {
		return nil, err
	}
	hist := newHabitHistory(h, checkins)
	st := hist.stats(day)
	st.Month = first.Format(habitMonthFormat)
	st.Days = hist.month(first)
	return st, nil
}

func sortCheckins(checkins []*HabitCheckin) {
	sort.Slice(checkins, func(i, j int) bool { return checkins[i].Day.Before(checkins[j].Day) })
}
