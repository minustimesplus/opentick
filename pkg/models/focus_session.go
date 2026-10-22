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
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// FocusSessionType is the timer mode a focus session was recorded with.
type FocusSessionType string

const (
	// FocusSessionTypePomo is a countdown pomodoro. Each one counts as a pomo in the stats.
	FocusSessionTypePomo FocusSessionType = "pomo"
	// FocusSessionTypeStopwatch is an open-ended, counting-up focus session.
	FocusSessionTypeStopwatch FocusSessionType = "stopwatch"
)

// MaxFocusStatsDays is the longest per-day history the stats endpoint returns.
const MaxFocusStatsDays = 366

// FocusSession is a finished span of focused work from the pomodoro / stopwatch
// timer, optionally linked to the task it was spent on. Sessions are private to
// the user who recorded them.
type FocusSession struct {
	ID     int64 `xorm:"bigint autoincr not null unique pk" json:"id" param:"focussession" readOnly:"true" doc:"The unique, numeric id of this focus session."`
	UserID int64 `xorm:"bigint not null INDEX" json:"-"`

	TaskID int64            `xorm:"bigint null INDEX" json:"task_id" doc:"The task this session was spent on, or 0 for none. You need read access to the task."`
	Type   FocusSessionType `xorm:"varchar(20) not null" json:"type" enum:"pomo,stopwatch" doc:"pomo for a countdown pomodoro (counted in the pomo stats), stopwatch for an open-ended session."`

	StartTime time.Time `xorm:"not null INDEX" json:"start_time" doc:"When the session started."`
	EndTime   time.Time `xorm:"not null" json:"end_time" doc:"When the session ended. Defaults to now."`
	Duration  int64     `xorm:"bigint not null default 0" json:"duration" minimum:"0" doc:"Focused time in seconds, excluding pauses. Must not exceed end_time - start_time. Defaults to end_time - start_time."`

	Note string `xorm:"text null" json:"note" doc:"An optional note about what was done."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this session was created. You cannot change this value."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this session was last updated. You cannot change this value."`

	// Range filter for ReadAll (not persisted): set by the v2 list route.
	From *time.Time `xorm:"-" json:"-"`
	To   *time.Time `xorm:"-" json:"-"`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for focus sessions.
func (*FocusSession) TableName() string {
	return "focus_sessions"
}

// normalize fills in defaults and validates type, times and duration.
func (fs *FocusSession) normalize() error {
	if fs.Type != FocusSessionTypePomo && fs.Type != FocusSessionTypeStopwatch {
		return ErrFocusSessionInvalidType{Type: string(fs.Type)}
	}
	if fs.StartTime.IsZero() {
		return ErrFocusSessionInvalidTimes{}
	}
	if fs.EndTime.IsZero() {
		fs.EndTime = time.Now()
	}
	if fs.EndTime.Before(fs.StartTime) {
		return ErrFocusSessionInvalidTimes{}
	}
	span := int64(fs.EndTime.Sub(fs.StartTime).Seconds())
	if fs.Duration == 0 {
		fs.Duration = span
	}
	if fs.Duration < 0 || fs.Duration > span {
		return ErrFocusSessionInvalidTimes{}
	}
	return nil
}

// Create records a focus session for the authenticated user.
func (fs *FocusSession) Create(s *xorm.Session, a web.Auth) error {
	if err := fs.normalize(); err != nil {
		return err
	}
	fs.ID = 0
	fs.UserID = a.GetID()
	_, err := s.Insert(fs)
	return err
}

// ReadOne returns a focus session. CanRead already loaded it.
func (fs *FocusSession) ReadOne(_ *xorm.Session, _ web.Auth) error {
	return nil
}

// ReadAll returns the authenticated user's focus sessions, newest first,
// optionally limited to those starting in [From, To).
func (fs *FocusSession) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result any, resultCount int, numberOfTotalItems int64, err error) {
	// DoReadAll does not call CanRead, so link shares must be refused here.
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, 0, 0, ErrGenericForbidden{}
	}

	cond := builder.NewCond().And(builder.Eq{"user_id": a.GetID()})
	if fs.From != nil {
		cond = cond.And(builder.Gte{"start_time": *fs.From})
	}
	if fs.To != nil {
		cond = cond.And(builder.Lt{"start_time": *fs.To})
	}
	if fs.TaskID != 0 {
		cond = cond.And(builder.Eq{"task_id": fs.TaskID})
	}
	if search != "" {
		cond = cond.And(db.ILIKE("note", search))
	}

	sessions := []*FocusSession{}
	query := s.Where(cond).OrderBy("start_time DESC, id DESC")
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	if err := query.Find(&sessions); err != nil {
		return nil, 0, 0, err
	}

	total, err := s.Where(cond).Count(&FocusSession{})
	if err != nil {
		return nil, 0, 0, err
	}
	return sessions, len(sessions), total, nil
}

// Update changes a focus session.
func (fs *FocusSession) Update(s *xorm.Session, _ web.Auth) error {
	if err := fs.normalize(); err != nil {
		return err
	}
	if _, err := s.ID(fs.ID).Cols("task_id", "type", "start_time", "end_time", "duration", "note").Update(fs); err != nil {
		return err
	}
	return fs.load(s)
}

// Delete removes a focus session.
func (fs *FocusSession) Delete(s *xorm.Session, _ web.Auth) error {
	_, err := s.ID(fs.ID).Delete(&FocusSession{})
	return err
}

func (fs *FocusSession) load(s *xorm.Session) error {
	exists, err := s.ID(fs.ID).Get(fs)
	if err != nil {
		return err
	}
	if !exists {
		return ErrFocusSessionDoesNotExist{ID: fs.ID}
	}
	return nil
}

// FocusDay is the focus summary of a single calendar day.
type FocusDay struct {
	Date         string `json:"date" doc:"The day, as YYYY-MM-DD in the requested timezone."`
	Pomos        int64  `json:"pomos" doc:"Number of pomodoro sessions started that day."`
	FocusMinutes int64  `json:"focus_minutes" doc:"Focused minutes (pomodoro and stopwatch) of sessions started that day."`
}

// FocusStats summarises a user's focus sessions.
type FocusStats struct {
	TodayPomos        int64       `json:"today_pomos" doc:"Number of pomodoro sessions started today."`
	TodayFocusMinutes int64       `json:"today_focus_minutes" doc:"Focused minutes of sessions started today."`
	TotalPomos        int64       `json:"total_pomos" doc:"Number of pomodoro sessions ever recorded."`
	TotalFocusMinutes int64       `json:"total_focus_minutes" doc:"Focused minutes of all sessions ever recorded."`
	History           []*FocusDay `json:"history" doc:"One entry per day, oldest first and ending with today, including days without focus."`
}

// GetFocusStats returns the focus stats of the authenticated user, with the
// last `days` days of history. Days are cut in loc; nil uses the user's
// timezone setting.
func GetFocusStats(s *xorm.Session, a web.Auth, loc *time.Location, days int, now time.Time) (*FocusStats, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, ErrGenericForbidden{}
	}
	if days < 1 {
		days = 1
	}
	if days > MaxFocusStatsDays {
		days = MaxFocusStatsDays
	}
	if loc == nil {
		loc = time.UTC
		u, err := user.GetUserByID(s, a.GetID())
		if err != nil {
			return nil, err
		}
		if userLoc, err := time.LoadLocation(u.Timezone); err == nil && u.Timezone != "" {
			loc = userLoc
		}
	}

	owned := builder.Eq{"user_id": a.GetID()}
	stats := &FocusStats{}

	totalPomos, err := s.Where(owned).And(builder.Eq{"type": FocusSessionTypePomo}).Count(&FocusSession{})
	if err != nil {
		return nil, err
	}
	stats.TotalPomos = totalPomos
	totalSeconds, err := s.Where(owned).SumInt(&FocusSession{}, "duration")
	if err != nil {
		return nil, err
	}
	stats.TotalFocusMinutes = totalSeconds / 60

	localNow := now.In(loc)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, loc)
	first := today.AddDate(0, 0, -(days - 1))

	sessions := []*FocusSession{}
	if err := s.Where(owned).
		And(builder.Gte{"start_time": first}).
		And(builder.Lt{"start_time": today.AddDate(0, 0, 1)}).
		Cols("type", "start_time", "duration").
		Find(&sessions); err != nil {
		return nil, err
	}

	stats.History = make([]*FocusDay, days)
	index := make(map[string]*FocusDay, days)
	seconds := make(map[string]int64, days)
	for i := range days {
		day := &FocusDay{Date: first.AddDate(0, 0, i).Format(time.DateOnly)}
		stats.History[i] = day
		index[day.Date] = day
	}
	for _, session := range sessions {
		date := session.StartTime.In(loc).Format(time.DateOnly)
		day, ok := index[date]
		if !ok {
			continue
		}
		if session.Type == FocusSessionTypePomo {
			day.Pomos++
		}
		seconds[date] += session.Duration
	}
	for date, secs := range seconds {
		index[date].FocusMinutes = secs / 60
	}

	todayStats := stats.History[days-1]
	stats.TodayPomos = todayStats.Pomos
	stats.TodayFocusMinutes = todayStats.FocusMinutes
	return stats, nil
}
