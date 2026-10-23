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

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func statsBucketCounts(buckets []*StatsBucket) map[string]int64 {
	out := map[string]int64{}
	for _, b := range buckets {
		if b.Count > 0 {
			out[b.Date] = b.Count
		}
	}
	return out
}

func TestComputeUserStatistics(t *testing.T) {
	berlin, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)
	// Thursday 22 October 2026, 10:00 in Berlin.
	now := time.Date(2026, 10, 22, 10, 0, 0, 0, berlin)
	at := func(month time.Month, day, hour int) time.Time {
		return time.Date(2026, month, day, hour, 0, 0, 0, berlin)
	}

	tasks := []*statsTask{
		// On time, done today.
		{ID: 1, ProjectID: 1, Done: true, DoneAt: at(10, 22, 9), DueDate: at(10, 22, 23)},
		// Late, done on Monday of this week.
		{ID: 2, ProjectID: 1, Done: true, DoneAt: at(10, 19, 12), DueDate: at(10, 18, 12)},
		// No due date, done last month. 23:30 UTC is already the next day in Berlin.
		{ID: 3, ProjectID: 2, Done: true, DoneAt: time.Date(2026, 9, 30, 23, 30, 0, 0, time.UTC)},
		// Done without a completion time: counted in totals only.
		{ID: 4, ProjectID: 2, Done: true},
		// Open and overdue.
		{ID: 5, ProjectID: 1, DueDate: at(10, 20, 12)},
		// Open, due later today: not overdue.
		{ID: 6, ProjectID: 1, DueDate: at(10, 22, 18)},
	}

	stats := computeUserStatistics(tasks, now, time.Monday, UserStatisticsOptions{})

	assert.Equal(t, int64(4), stats.TotalCompleted)
	assert.Equal(t, int64(1), stats.CompletedToday)
	assert.Equal(t, int64(1), stats.OpenOverdue)

	require.Len(t, stats.Daily, 30)
	assert.Equal(t, "2026-10-22", stats.Daily[29].Date)
	assert.Equal(t, map[string]int64{"2026-10-22": 1, "2026-10-19": 1, "2026-10-01": 1}, statsBucketCounts(stats.Daily))

	require.Len(t, stats.Weekly, 12)
	assert.Equal(t, "2026-10-19", stats.Weekly[11].Date)
	assert.Equal(t, map[string]int64{"2026-10-19": 2, "2026-09-28": 1}, statsBucketCounts(stats.Weekly))

	require.Len(t, stats.Monthly, 12)
	assert.Equal(t, "2025-11-01", stats.Monthly[0].Date)
	assert.Equal(t, map[string]int64{"2026-10-01": 3}, statsBucketCounts(stats.Monthly))

	assert.Equal(t, StatsCompletionRate{OnTime: 1, Late: 1, NoDueDate: 2, Rate: 0.5}, stats.CompletionRate)

	require.Len(t, stats.Projects, 2)
	// Equal counts sort by project id.
	assert.Equal(t, int64(1), stats.Projects[0].ProjectID)
	assert.Equal(t, int64(2), stats.Projects[0].Count)
	assert.Equal(t, int64(2), stats.Projects[1].ProjectID)

	// 4*10 completed + 1*5 on time - 1*5 late - 1*2 overdue.
	assert.Equal(t, StatsScore{
		Score:            38,
		Level:            1,
		LevelScore:       0,
		NextLevelScore:   50,
		CompletionPoints: 40,
		OnTimePoints:     5,
		LatePenalty:      5,
		OverduePenalty:   2,
	}, stats.Score)

	t.Run("sunday week start", func(t *testing.T) {
		stats := computeUserStatistics(tasks, now, time.Sunday, UserStatisticsOptions{Weeks: 2})
		require.Len(t, stats.Weekly, 2)
		assert.Equal(t, "2026-10-11", stats.Weekly[0].Date)
		assert.Equal(t, "2026-10-18", stats.Weekly[1].Date)
		assert.Equal(t, int64(2), stats.Weekly[1].Count)
	})

	t.Run("period limits", func(t *testing.T) {
		stats := computeUserStatistics(nil, now, time.Monday, UserStatisticsOptions{Days: 1000, Weeks: 1, Months: -3})
		assert.Len(t, stats.Daily, statsMaxDays)
		assert.Len(t, stats.Weekly, 1)
		assert.Len(t, stats.Monthly, 12)
		assert.Empty(t, stats.Projects)
		assert.Zero(t, stats.CompletionRate.Rate)
	})

	t.Run("score never negative", func(t *testing.T) {
		overdue := []*statsTask{{ID: 1, DueDate: at(1, 1, 0)}, {ID: 2, DueDate: at(1, 2, 0)}}
		stats := computeUserStatistics(overdue, now, time.Monday, UserStatisticsOptions{})
		assert.Equal(t, int64(0), stats.Score.Score)
		assert.Equal(t, int64(4), stats.Score.OverduePenalty)
		assert.Equal(t, int64(1), stats.Score.Level)
	})
}

func TestStatsLevelFor(t *testing.T) {
	cases := []struct {
		score, level, levelScore, next int64
	}{
		{0, 1, 0, 50},
		{49, 1, 0, 50},
		{50, 2, 50, 150},
		{149, 2, 50, 150},
		{150, 3, 150, 300},
		{300, 4, 300, 500},
	}
	for _, c := range cases {
		level, levelScore, next := StatsLevelFor(c.score)
		assert.Equal(t, []int64{c.level, c.levelScore, c.next}, []int64{level, levelScore, next}, "score %d", c.score)
	}
}

func TestGetUserStatistics(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()

	now := time.Now()
	u1 := &user.User{ID: 1}
	// user 13 has no access to project 1.
	u13 := &user.User{ID: 13}

	before, err := GetUserStatistics(s, u1, now, UserStatisticsOptions{})
	require.NoError(t, err)
	outsiderBefore, err := GetUserStatistics(s, u13, now, UserStatisticsOptions{})
	require.NoError(t, err)

	_, err = s.Insert(&Task{Title: "done on time", ProjectID: 1, Index: 9001, CreatedByID: 1, Done: true, DoneAt: now.Add(-time.Minute), DueDate: now.Add(time.Hour)})
	require.NoError(t, err)
	_, err = s.Insert(&Task{Title: "overdue", ProjectID: 1, Index: 9002, CreatedByID: 1, DueDate: now.Add(-time.Hour)})
	require.NoError(t, err)
	// Done by someone else in a project user 1 can see, and not assigned to user 1.
	_, err = s.Insert(&Task{Title: "someone else's", ProjectID: 1, Index: 9003, CreatedByID: 2, Done: true, DoneAt: now})
	require.NoError(t, err)
	// Assigned to user 1, so it counts although someone else created it.
	assigned := &Task{Title: "assigned", ProjectID: 1, Index: 9004, CreatedByID: 2, Done: true, DoneAt: now.Add(-time.Minute)}
	_, err = s.Insert(assigned)
	require.NoError(t, err)
	_, err = s.Insert(&TaskAssginee{TaskID: assigned.ID, UserID: 1})
	require.NoError(t, err)
	// Trashed tasks are not counted.
	trashed := &Task{Title: "trashed", ProjectID: 1, Index: 9005, CreatedByID: 1, Done: true, DoneAt: now}
	_, err = s.Insert(trashed)
	require.NoError(t, err)
	_, err = s.ID(trashed.ID).Delete(&Task{})
	require.NoError(t, err)

	after, err := GetUserStatistics(s, u1, now, UserStatisticsOptions{})
	require.NoError(t, err)

	assert.Equal(t, before.TotalCompleted+2, after.TotalCompleted)
	assert.Equal(t, before.CompletedToday+2, after.CompletedToday)
	assert.Equal(t, before.OpenOverdue+1, after.OpenOverdue)
	assert.Equal(t, before.CompletionRate.OnTime+1, after.CompletionRate.OnTime)
	assert.NotEmpty(t, after.Timezone)

	var project1 *StatsProject
	for _, p := range after.Projects {
		if p.ProjectID == 1 {
			project1 = p
		}
	}
	require.NotNil(t, project1)
	assert.Equal(t, "Test1", project1.Title)

	outsiderAfter, err := GetUserStatistics(s, u13, now, UserStatisticsOptions{})
	require.NoError(t, err)
	assert.Equal(t, outsiderBefore.TotalCompleted, outsiderAfter.TotalCompleted)
	assert.Equal(t, outsiderBefore.OpenOverdue, outsiderAfter.OpenOverdue)
}
