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
	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

const (
	// HabitFrequencyDaily habits are due every day.
	HabitFrequencyDaily = "daily"
	// HabitFrequencyWeekdays habits are due on the weekdays set in Habit.Weekdays.
	HabitFrequencyWeekdays = "weekdays"

	// HabitAllWeekdays is the Weekdays bitmask with every day set.
	HabitAllWeekdays = 0b1111111
	// MaxHabitGoal is the highest daily goal a habit can have.
	MaxHabitGoal = 100000
	// MaxHabitCheckinRange is the longest range check-ins can be listed for at once, in days.
	MaxHabitCheckinRange = 366
	// habitRecentDays is how many days of check-ins habit lists carry, ending today.
	habitRecentDays = 7

	habitDateFormat  = "2006-01-02"
	habitMonthFormat = "2006-01"
)

// Habit is something a user wants to do regularly, like drinking water or
// reading. Each day they check in with how much they did.
type Habit struct {
	ID        int64  `xorm:"bigint autoincr not null unique pk" json:"id" param:"habit" readOnly:"true" doc:"The unique, numeric id of this habit."`
	UserID    int64  `xorm:"bigint not null index" json:"-"`
	Title     string `xorm:"varchar(250) not null" json:"title" valid:"runelength(1|250)" minLength:"1" maxLength:"250" doc:"The name of the habit."`
	Icon      string `xorm:"varchar(50) null" json:"icon" maxLength:"50" doc:"An emoji shown next to the habit."`
	HexColor  string `xorm:"varchar(6) null" json:"hex_color" valid:"hexcolor" maxLength:"7" doc:"The habit's color, in hex format."`
	Frequency string `xorm:"varchar(20) not null default 'daily'" json:"frequency" enum:"daily,weekdays" doc:"When the habit is due: every day, or only on the days in weekdays. Defaults to daily."`
	Weekdays  int    `xorm:"int not null default 127" json:"weekdays" minimum:"0" maximum:"127" doc:"Bitmask of the days a weekdays habit is due on: bit 0 is Sunday, bit 6 Saturday. Always 127 for daily habits."`
	Goal      int    `xorm:"int not null default 1" json:"goal" minimum:"0" maximum:"100000" doc:"How much to check in per day for the day to count as done, e.g. 8 glasses of water. Defaults to 1."`
	Unit      string `xorm:"varchar(50) null" json:"unit" maxLength:"50" doc:"What the goal counts, e.g. glasses. Display only."`
	Archived  bool   `xorm:"bool not null default false" json:"archived" doc:"Archived habits keep their history but are no longer tracked."`

	// Today's value and streak, as of the requested (or the user's) day.
	TodayValue     int             `xorm:"-" json:"today_value" readOnly:"true" doc:"How much was checked in today. Set by the server."`
	CurrentStreak  int             `xorm:"-" json:"current_streak" readOnly:"true" doc:"The number of consecutive due days the goal was reached, up to today. Today only breaks the streak once it is over. Set by the server."`
	RecentCheckins []*HabitCheckin `xorm:"-" json:"recent_checkins" readOnly:"true" doc:"The check-ins of the last 7 days, today included. Set by the server."`

	// Today overrides the day the computed fields are relative to, as YYYY-MM-DD.
	// Defaults to today in the user's timezone.
	Today string `xorm:"-" json:"-"`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this habit was created. You cannot change this value."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this habit was last updated. You cannot change this value."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for habits.
func (*Habit) TableName() string {
	return "habits"
}

// HabitCheckin is how much of a habit was done on one day. A habit has at
// most one check-in per day.
type HabitCheckin struct {
	ID      int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	HabitID int64 `xorm:"bigint not null unique(habit_day)" json:"habit_id" readOnly:"true" doc:"The habit this check-in belongs to."`
	// Midnight UTC of the checked-in calendar day.
	Day   time.Time `xorm:"datetime not null unique(habit_day)" json:"-"`
	Date  string    `xorm:"-" json:"date" readOnly:"true" doc:"The checked-in day, YYYY-MM-DD."`
	Value int       `xorm:"int not null" json:"value" minimum:"0" maximum:"100000" doc:"How much was done that day."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this check-in was created."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this check-in was last changed."`
}

// TableName returns the table name for habit check-ins.
func (*HabitCheckin) TableName() string {
	return "habit_checkins"
}

// ParseHabitDate parses a YYYY-MM-DD day into midnight UTC.
func ParseHabitDate(value string) (time.Time, error) {
	d, err := time.ParseInLocation(habitDateFormat, value, time.UTC)
	if err != nil {
		return time.Time{}, ErrHabitInvalidDate{Date: value}
	}
	return d, nil
}

// habitDayRange matches the check-ins from the first to the last day, both
// inclusive. SQLite keeps the day as text whose exact form depends on the
// writer (xorm omits the zone, a bound time.Time adds it), so days are matched
// as half-open ranges of zone-less UTC strings instead of by equality.
func habitDayRange(first, last time.Time) builder.Cond {
	return builder.And(
		builder.Gte{"day": first.UTC().Format(dbTimeFormat)},
		builder.Lt{"day": last.UTC().AddDate(0, 0, 1).Format(dbTimeFormat)},
	)
}

func habitDateKey(d time.Time) string {
	return d.UTC().Format(habitDateFormat)
}

func (h *Habit) normalize() error {
	h.HexColor = utils.NormalizeHex(h.HexColor)
	switch h.Frequency {
	case "", HabitFrequencyDaily:
		h.Frequency = HabitFrequencyDaily
		h.Weekdays = HabitAllWeekdays
	case HabitFrequencyWeekdays:
		h.Weekdays &= HabitAllWeekdays
		if h.Weekdays == 0 {
			return ErrHabitInvalidWeekdays{}
		}
	default:
		return ErrHabitInvalidFrequency{Frequency: h.Frequency}
	}
	if h.Goal < 1 {
		h.Goal = 1
	}
	if h.Goal > MaxHabitGoal {
		h.Goal = MaxHabitGoal
	}
	return nil
}

// Create adds a habit for the authenticated user.
func (h *Habit) Create(s *xorm.Session, a web.Auth) error {
	if err := h.normalize(); err != nil {
		return err
	}
	h.ID = 0
	h.UserID = a.GetID()
	if _, err := s.Insert(h); err != nil {
		return err
	}
	return h.addProgress(s, a, nil)
}

// ReadOne returns a habit with today's progress. CanRead already loaded it.
func (h *Habit) ReadOne(s *xorm.Session, a web.Auth) error {
	return h.addProgress(s, a, nil)
}

// ReadAll returns all habits of the authenticated user, each with today's progress.
func (h *Habit) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result any, resultCount int, numberOfTotalItems int64, err error) {
	// DoReadAll does not call CanRead, so link shares must be refused here.
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, 0, 0, ErrGenericForbidden{}
	}

	cond := builder.NewCond().And(builder.Eq{"user_id": a.GetID()})
	if search != "" {
		cond = cond.And(db.ILIKE("title", search))
	}

	habits := []*Habit{}
	query := s.Where(cond).OrderBy("id ASC")
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	if err := query.Find(&habits); err != nil {
		return nil, 0, 0, err
	}

	total, err := s.Where(cond).Count(&Habit{})
	if err != nil {
		return nil, 0, 0, err
	}

	if len(habits) > 0 {
		ids := make([]int64, 0, len(habits))
		for _, habit := range habits {
			ids = append(ids, habit.ID)
		}
		checkins := []*HabitCheckin{}
		if err := s.In("habit_id", ids).Find(&checkins); err != nil {
			return nil, 0, 0, err
		}
		byHabit := make(map[int64][]*HabitCheckin, len(habits))
		for _, c := range checkins {
			byHabit[c.HabitID] = append(byHabit[c.HabitID], c)
		}
		for _, habit := range habits {
			habit.Today = h.Today
			if err := habit.addProgress(s, a, byHabit[habit.ID]); err != nil {
				return nil, 0, 0, err
			}
		}
	}

	return habits, len(habits), total, nil
}

// Update changes a habit. Its history stays; it is re-evaluated against the new schedule and goal.
func (h *Habit) Update(s *xorm.Session, a web.Auth) error {
	if err := h.normalize(); err != nil {
		return err
	}
	if _, err := s.ID(h.ID).Cols("title", "icon", "hex_color", "frequency", "weekdays", "goal", "unit", "archived").Update(h); err != nil {
		return err
	}
	if err := h.load(s); err != nil {
		return err
	}
	return h.addProgress(s, a, nil)
}

// Delete removes a habit and its check-ins.
func (h *Habit) Delete(s *xorm.Session, _ web.Auth) error {
	if _, err := s.Where("habit_id = ?", h.ID).Delete(&HabitCheckin{}); err != nil {
		return err
	}
	_, err := s.ID(h.ID).Delete(&Habit{})
	return err
}

// load replaces h with the stored habit, keeping h.Today.
func (h *Habit) load(s *xorm.Session) error {
	// A fresh bean, as Get would also filter by h's other non-zero fields.
	stored := &Habit{}
	exists, err := s.ID(h.ID).Get(stored)
	if err != nil {
		return err
	}
	if !exists {
		return ErrHabitDoesNotExist{ID: h.ID}
	}
	stored.Today = h.Today
	*h = *stored
	return nil
}

// deleteHabitsOfUser removes all habits of a user with their check-ins.
func deleteHabitsOfUser(s *xorm.Session, userID int64) error {
	_, err := s.
		Where(builder.In("habit_id", builder.Select("id").From("habits").Where(builder.Eq{"user_id": userID}))).
		Delete(&HabitCheckin{})
	if err != nil {
		return err
	}
	_, err = s.Where("user_id = ?", userID).Delete(&Habit{})
	return err
}

// resolveToday returns the day h.Today names, or today in the user's timezone.
func (h *Habit) resolveToday(s *xorm.Session, a web.Auth) (time.Time, error) {
	if h.Today != "" {
		return ParseHabitDate(h.Today)
	}
	loc := time.UTC
	u, err := user.GetUserByID(s, a.GetID())
	if err != nil {
		return time.Time{}, err
	}
	if u.Timezone != "" {
		if userLoc, err := time.LoadLocation(u.Timezone); err == nil {
			loc = userLoc
		}
	}
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC), nil
}

// addProgress sets today's value, the current streak and the recent
// check-ins. checkins are all of the habit's check-ins; nil loads them.
func (h *Habit) addProgress(s *xorm.Session, a web.Auth, checkins []*HabitCheckin) error {
	today, err := h.resolveToday(s, a)
	if err != nil {
		return err
	}
	h.Today = habitDateKey(today)
	if checkins == nil {
		if checkins, err = h.allCheckins(s); err != nil {
			return err
		}
	}

	hist := newHabitHistory(h, checkins)
	h.TodayValue = hist.values[habitDateKey(today)]
	h.CurrentStreak = hist.currentStreak(today)
	h.RecentCheckins = []*HabitCheckin{}
	from := today.AddDate(0, 0, -(habitRecentDays - 1))
	for _, c := range checkins {
		if !c.Day.Before(from) && !c.Day.After(today) {
			h.RecentCheckins = append(h.RecentCheckins, c)
		}
	}
	sortCheckins(h.RecentCheckins)
	return nil
}

func (h *Habit) allCheckins(s *xorm.Session) ([]*HabitCheckin, error) {
	checkins := []*HabitCheckin{}
	if err := s.Where("habit_id = ?", h.ID).OrderBy("day ASC").Find(&checkins); err != nil {
		return nil, err
	}
	return checkins, nil
}

// AfterLoad fills the date from the stored day.
func (c *HabitCheckin) AfterLoad() {
	c.Day = c.Day.UTC()
	c.Date = habitDateKey(c.Day)
}

// GetCheckins returns the check-ins between from and to, both inclusive.
// Callers must have checked CanRead.
func (h *Habit) GetCheckins(s *xorm.Session, from, to time.Time) ([]*HabitCheckin, error) {
	if to.Before(from) || to.Sub(from) >= MaxHabitCheckinRange*24*time.Hour {
		return nil, ErrHabitInvalidRange{}
	}
	checkins := []*HabitCheckin{}
	err := s.
		Where("habit_id = ?", h.ID).And(habitDayRange(from, to)).
		OrderBy("day ASC").
		Find(&checkins)
	return checkins, err
}

// SetCheckin records how much of the habit was done on day; a value of 0
// removes the check-in. It then reloads the habit with its progress as of
// h.Today. Callers must have checked CanUpdate.
func (h *Habit) SetCheckin(s *xorm.Session, a web.Auth, day time.Time, value int) error {
	// Allow a day of slack for users ahead of UTC.
	if day.After(time.Now().UTC().AddDate(0, 0, 1)) {
		return ErrHabitCheckinInFuture{}
	}
	value = min(value, MaxHabitGoal)

	existing := &HabitCheckin{}
	exists, err := s.Where("habit_id = ?", h.ID).And(habitDayRange(day, day)).Get(existing)
	if err != nil {
		return err
	}
	switch {
	case value <= 0:
		err = h.DeleteCheckin(s, day)
	case exists:
		existing.Value = value
		if _, err = s.ID(existing.ID).Cols("value").Update(existing); err == nil {
			err = h.touch(s)
		}
	default:
		if _, err = s.Insert(&HabitCheckin{HabitID: h.ID, Day: day, Value: value}); err == nil {
			err = h.touch(s)
		}
	}
	if err != nil {
		return err
	}

	if err := h.load(s); err != nil {
		return err
	}
	return h.addProgress(s, a, nil)
}

// DeleteCheckin removes the check-in of day, if there is one. Callers must
// have checked CanUpdate.
func (h *Habit) DeleteCheckin(s *xorm.Session, day time.Time) error {
	deleted, err := s.Where("habit_id = ?", h.ID).And(habitDayRange(day, day)).Delete(&HabitCheckin{})
	if err != nil || deleted == 0 {
		return err
	}
	return h.touch(s)
}

// touch bumps the habit's updated timestamp, so its ETag changes with its check-ins.
func (h *Habit) touch(s *xorm.Session) error {
	_, err := s.ID(h.ID).Cols("updated").Update(&Habit{Updated: time.Now()})
	return err
}
