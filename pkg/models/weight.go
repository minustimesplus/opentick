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
	"math"
	"time"

	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

const (
	// MinWeight and MaxWeight bound a logged body weight, in kilograms.
	MinWeight = 1
	MaxWeight = 1000
	// MaxWeightRange is the longest range weights can be listed for at once, in days.
	MaxWeightRange = 3660
)

// Weight is the body weight a user logged on one day. A user has at most one
// weight per day; logging again replaces it.
type Weight struct {
	ID     int64 `xorm:"bigint autoincr not null unique pk" json:"-"`
	UserID int64 `xorm:"bigint not null unique(user_day)" json:"-"`
	// Midnight UTC of the weighed-in calendar day.
	Day    time.Time `xorm:"datetime not null unique(user_day)" json:"-"`
	Date   string    `xorm:"-" json:"date" readOnly:"true" doc:"The day of the weigh-in, YYYY-MM-DD."`
	Weight float64   `xorm:"double not null" json:"weight" minimum:"1" maximum:"1000" doc:"The body weight in kilograms, rounded to 0.1."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this weight was first logged."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this weight was last changed."`
}

// TableName returns the table name for weights.
func (*Weight) TableName() string {
	return "weights"
}

// AfterLoad fills the date from the stored day.
func (w *Weight) AfterLoad() {
	w.Day = w.Day.UTC()
	w.Date = habitDateKey(w.Day)
}

// Weights are private to their user; link shares have none.
func weightOwner(a web.Auth) (int64, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return 0, ErrGenericForbidden{}
	}
	return a.GetID(), nil
}

// GetWeights returns the user's weights between from and to, both inclusive, oldest first.
func GetWeights(s *xorm.Session, a web.Auth, from, to time.Time) ([]*Weight, error) {
	userID, err := weightOwner(a)
	if err != nil {
		return nil, err
	}
	if to.Before(from) || to.Sub(from) >= MaxWeightRange*24*time.Hour {
		return nil, ErrWeightInvalidRange{}
	}
	weights := []*Weight{}
	err = s.
		Where("user_id = ?", userID).And(habitDayRange(from, to)).
		OrderBy("day ASC").
		Find(&weights)
	return weights, err
}

// SetWeight logs the user's weight for day, replacing an earlier one, and
// checks off their weigh-in habits (those that track weight) for that day.
// today is the day the habits' progress is relative to, as for check-ins.
func SetWeight(s *xorm.Session, a web.Auth, day time.Time, kg float64, today string) (*Weight, error) {
	userID, err := weightOwner(a)
	if err != nil {
		return nil, err
	}
	// Allow a day of slack for users ahead of UTC.
	if day.After(time.Now().UTC().AddDate(0, 0, 1)) {
		return nil, ErrHabitCheckinInFuture{}
	}
	kg = math.Round(kg*10) / 10
	if kg < MinWeight || kg > MaxWeight {
		return nil, ErrWeightInvalid{}
	}

	w := &Weight{}
	exists, err := s.Where("user_id = ?", userID).And(habitDayRange(day, day)).Get(w)
	if err != nil {
		return nil, err
	}
	if exists {
		w.Weight = kg
		if _, err := s.ID(w.ID).Cols("weight").Update(w); err != nil {
			return nil, err
		}
	} else {
		w = &Weight{UserID: userID, Day: day, Weight: kg}
		if _, err := s.Insert(w); err != nil {
			return nil, err
		}
	}
	w.Day = day
	w.Date = habitDateKey(day)

	habits := []*Habit{}
	if err := s.Where("user_id = ? AND tracks_weight = ? AND archived = ?", userID, true, false).Find(&habits); err != nil {
		return nil, err
	}
	for _, h := range habits {
		existing := &HabitCheckin{}
		has, err := s.Where("habit_id = ?", h.ID).And(habitDayRange(day, day)).Get(existing)
		if err != nil {
			return nil, err
		}
		if has && existing.Value >= h.Goal {
			continue
		}
		h.Today = today
		if err := h.SetCheckin(s, a, day, h.Goal); err != nil {
			return nil, err
		}
	}
	return w, nil
}

// DeleteWeight removes the user's weight of day, if there is one. Check-ins
// of weigh-in habits are left alone.
func DeleteWeight(s *xorm.Session, a web.Auth, day time.Time) error {
	userID, err := weightOwner(a)
	if err != nil {
		return err
	}
	_, err = s.Where("user_id = ?", userID).And(habitDayRange(day, day)).Delete(&Weight{})
	return err
}

// deleteWeightsOfUser removes all weights of a user.
func deleteWeightsOfUser(s *xorm.Session, userID int64) error {
	_, err := s.Where(builder.Eq{"user_id": userID}).Delete(&Weight{})
	return err
}
