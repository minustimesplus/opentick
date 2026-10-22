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

	"code.vikunja.io/api/pkg/user"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// Achievement score formula. Every completed task earns StatsPointsPerCompletion;
// one completed on or before its due date earns StatsPointsOnTimeBonus on top,
// one completed after its due date loses StatsPointsLatePenalty, and every open
// task whose due date has passed loses StatsPointsOverduePenalty. The score never
// drops below zero.
const (
	StatsPointsPerCompletion  = 10
	StatsPointsOnTimeBonus    = 5
	StatsPointsLatePenalty    = 5
	StatsPointsOverduePenalty = 2
	// StatsLevelStep sets the level curve: reaching level n takes
	// StatsLevelStep * n * (n-1) / 2 points, so level 2 is at 50, level 3 at
	// 150, level 4 at 300 and each level needs StatsLevelStep more than the last.
	StatsLevelStep = 50

	statsMaxDays   = 366
	statsMaxWeeks  = 104
	statsMaxMonths = 60
)

// StatsBucket is the number of tasks completed in one day, week or month.
type StatsBucket struct {
	Date  string `json:"date" doc:"First day of the period as YYYY-MM-DD, in the user's time zone."`
	Count int64  `json:"count" doc:"Tasks completed in the period."`
}

// StatsProject is the number of completed tasks in one project.
type StatsProject struct {
	ProjectID int64  `json:"project_id"`
	Title     string `json:"title"`
	HexColor  string `json:"hex_color"`
	Count     int64  `json:"count" doc:"Tasks completed in this project, all time."`
}

// StatsCompletionRate splits completed tasks with a due date by punctuality.
type StatsCompletionRate struct {
	OnTime    int64   `json:"on_time" doc:"Completed tasks finished on or before their due date."`
	Late      int64   `json:"late" doc:"Completed tasks finished after their due date."`
	NoDueDate int64   `json:"no_due_date" doc:"Completed tasks without a due date; not counted in the rate."`
	Rate      float64 `json:"rate" doc:"on_time / (on_time + late), between 0 and 1. 0 when no completed task had a due date."`
}

// StatsScore is the achievement score and the level it reaches.
type StatsScore struct {
	Score            int64 `json:"score" doc:"Total achievement points, never below 0."`
	Level            int64 `json:"level" doc:"Current level, starting at 1."`
	LevelScore       int64 `json:"level_score" doc:"Points needed to reach the current level."`
	NextLevelScore   int64 `json:"next_level_score" doc:"Points needed to reach the next level."`
	CompletionPoints int64 `json:"completion_points" doc:"Points from completed tasks."`
	OnTimePoints     int64 `json:"on_time_points" doc:"Bonus points from tasks completed on time."`
	LatePenalty      int64 `json:"late_penalty" doc:"Points lost to tasks completed after their due date (positive number)."`
	OverduePenalty   int64 `json:"overdue_penalty" doc:"Points lost to open tasks past their due date (positive number)."`
}

// UserStatistics summarises the task completions of one user.
type UserStatistics struct {
	Timezone       string              `json:"timezone" doc:"The IANA time zone the periods are computed in."`
	TotalCompleted int64               `json:"total_completed"`
	CompletedToday int64               `json:"completed_today"`
	OpenOverdue    int64               `json:"open_overdue" doc:"Open tasks whose due date has passed."`
	Daily          []*StatsBucket      `json:"daily" doc:"Completions per day, oldest first, ending today."`
	Weekly         []*StatsBucket      `json:"weekly" doc:"Completions per week, oldest first, ending with the current week. Weeks start on the user's configured week start day."`
	Monthly        []*StatsBucket      `json:"monthly" doc:"Completions per calendar month, oldest first, ending with the current month."`
	CompletionRate StatsCompletionRate `json:"completion_rate"`
	Projects       []*StatsProject     `json:"projects" doc:"Completed tasks per project, most completions first. Projects without completions are left out."`
	Score          StatsScore          `json:"score"`
}

// UserStatisticsOptions sets how many periods each series covers and, when
// set, overrides the user's time zone and week start day.
type UserStatisticsOptions struct {
	Days      int
	Weeks     int
	Months    int
	Timezone  string
	WeekStart *time.Weekday
}

// statsTask holds the task columns the statistics need.
type statsTask struct {
	ID        int64     `xorm:"id"`
	ProjectID int64     `xorm:"project_id"`
	Done      bool      `xorm:"done"`
	DoneAt    time.Time `xorm:"done_at"`
	DueDate   time.Time `xorm:"due_date"`
}

// GetUserStatistics computes the statistics for u at the moment now. It counts
// tasks in projects u can access that u created or is assigned to, so tasks other
// people finish in a shared project do not inflate u's score.
func GetUserStatistics(s *xorm.Session, u *user.User, now time.Time, opts UserStatisticsOptions) (*UserStatistics, error) {
	fullUser, err := user.GetUserByID(s, u.ID)
	if err != nil {
		return nil, err
	}

	access, err := getProjectAccessForUser(s, fullUser.ID)
	if err != nil {
		return nil, err
	}

	tasks := []*statsTask{}
	if len(access.sortedIDs) > 0 {
		err = s.Table("tasks").
			Cols("id", "project_id", "done", "done_at", "due_date").
			Where(builder.And(
				access.cond("project_id"),
				taskNotDeletedCond("tasks"),
				builder.Or(
					builder.Eq{"created_by_id": fullUser.ID},
					builder.In("id", builder.Select("task_id").From("task_assignees").Where(builder.Eq{"user_id": fullUser.ID})),
				),
				builder.Or(
					builder.Eq{"done": true},
					builder.And(builder.NotNull{"due_date"}, builder.Lt{"due_date": now}),
				),
			)).
			Find(&tasks)
		if err != nil {
			return nil, err
		}
	}

	tz := opts.Timezone
	if tz == "" {
		tz = fullUser.Timezone
	}
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc = time.UTC
	}
	weekStart := time.Weekday(fullUser.WeekStart % 7)
	if opts.WeekStart != nil {
		weekStart = *opts.WeekStart
	}

	stats := computeUserStatistics(tasks, now.In(loc), weekStart, opts)
	stats.Timezone = loc.String()

	if err := addStatsProjectDetails(s, stats.Projects); err != nil {
		return nil, err
	}
	return stats, nil
}

func addStatsProjectDetails(s *xorm.Session, projects []*StatsProject) error {
	if len(projects) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(projects))
	for _, p := range projects {
		ids = append(ids, p.ProjectID)
	}
	rows := map[int64]*Project{}
	if err := s.In("id", ids).Find(&rows); err != nil {
		return err
	}
	for _, p := range projects {
		if row, has := rows[p.ProjectID]; has {
			p.Title = row.Title
			p.HexColor = row.HexColor
		}
	}
	return nil
}

func clampStatsPeriods(v, def, maxValue int) int {
	if v <= 0 {
		return def
	}
	if v > maxValue {
		return maxValue
	}
	return v
}

func statsDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

func statsWeek(t time.Time, weekStart time.Weekday) time.Time {
	d := statsDay(t)
	offset := (int(d.Weekday()) - int(weekStart) + 7) % 7
	return d.AddDate(0, 0, -offset)
}

func statsMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
}

// statsSeries builds count buckets keyed by period start, oldest first.
type statsSeries struct {
	buckets []*StatsBucket
	index   map[string]*StatsBucket
}

func newStatsSeries(last time.Time, n int, step func(time.Time, int) time.Time) *statsSeries {
	series := &statsSeries{buckets: make([]*StatsBucket, 0, n), index: make(map[string]*StatsBucket, n)}
	for i := n - 1; i >= 0; i-- {
		b := &StatsBucket{Date: step(last, -i).Format(time.DateOnly)}
		series.buckets = append(series.buckets, b)
		series.index[b.Date] = b
	}
	return series
}

func (s *statsSeries) add(start time.Time) {
	if b, has := s.index[start.Format(time.DateOnly)]; has {
		b.Count++
	}
}

// StatsLevelFor returns the level a score reaches and the scores at which that
// level and the next one start.
func StatsLevelFor(score int64) (level, levelScore, nextLevelScore int64) {
	threshold := func(n int64) int64 { return StatsLevelStep * n * (n - 1) / 2 }
	level = 1
	for threshold(level+1) <= score {
		level++
	}
	return level, threshold(level), threshold(level + 1)
}

// computeUserStatistics aggregates tasks in now's location. tasks must contain
// every completed task and every open task that is overdue at now.
func computeUserStatistics(tasks []*statsTask, now time.Time, weekStart time.Weekday, opts UserStatisticsOptions) *UserStatistics {
	loc := now.Location()
	days := clampStatsPeriods(opts.Days, 30, statsMaxDays)
	weeks := clampStatsPeriods(opts.Weeks, 12, statsMaxWeeks)
	months := clampStatsPeriods(opts.Months, 12, statsMaxMonths)

	daily := newStatsSeries(statsDay(now), days, func(t time.Time, i int) time.Time { return t.AddDate(0, 0, i) })
	weekly := newStatsSeries(statsWeek(now, weekStart), weeks, func(t time.Time, i int) time.Time { return t.AddDate(0, 0, 7*i) })
	monthly := newStatsSeries(statsMonth(now), months, func(t time.Time, i int) time.Time { return t.AddDate(0, i, 0) })

	stats := &UserStatistics{}
	perProject := map[int64]*StatsProject{}
	today := statsDay(now)

	for _, t := range tasks {
		if !t.Done {
			if !t.DueDate.IsZero() && t.DueDate.Before(now) {
				stats.OpenOverdue++
			}
			continue
		}

		stats.TotalCompleted++
		p, has := perProject[t.ProjectID]
		if !has {
			p = &StatsProject{ProjectID: t.ProjectID}
			perProject[t.ProjectID] = p
		}
		p.Count++

		switch {
		case t.DueDate.IsZero():
			stats.CompletionRate.NoDueDate++
		case !t.DoneAt.IsZero() && t.DoneAt.After(t.DueDate):
			stats.CompletionRate.Late++
		default:
			stats.CompletionRate.OnTime++
		}

		// Tasks marked done before done_at existed have no completion time;
		// they count towards totals and the score but not towards any period.
		if t.DoneAt.IsZero() {
			continue
		}
		doneAt := t.DoneAt.In(loc)
		if statsDay(doneAt).Equal(today) {
			stats.CompletedToday++
		}
		daily.add(statsDay(doneAt))
		weekly.add(statsWeek(doneAt, weekStart))
		monthly.add(statsMonth(doneAt))
	}

	stats.Daily = daily.buckets
	stats.Weekly = weekly.buckets
	stats.Monthly = monthly.buckets

	rated := stats.CompletionRate.OnTime + stats.CompletionRate.Late
	if rated > 0 {
		stats.CompletionRate.Rate = float64(stats.CompletionRate.OnTime) / float64(rated)
	}

	stats.Projects = make([]*StatsProject, 0, len(perProject))
	for _, p := range perProject {
		stats.Projects = append(stats.Projects, p)
	}
	sort.Slice(stats.Projects, func(i, j int) bool {
		if stats.Projects[i].Count != stats.Projects[j].Count {
			return stats.Projects[i].Count > stats.Projects[j].Count
		}
		return stats.Projects[i].ProjectID < stats.Projects[j].ProjectID
	})

	sc := &stats.Score
	sc.CompletionPoints = stats.TotalCompleted * StatsPointsPerCompletion
	sc.OnTimePoints = stats.CompletionRate.OnTime * StatsPointsOnTimeBonus
	sc.LatePenalty = stats.CompletionRate.Late * StatsPointsLatePenalty
	sc.OverduePenalty = stats.OpenOverdue * StatsPointsOverduePenalty
	sc.Score = max(0, sc.CompletionPoints+sc.OnTimePoints-sc.LatePenalty-sc.OverduePenalty)
	sc.Level, sc.LevelScore, sc.NextLevelScore = StatsLevelFor(sc.Score)

	return stats
}
