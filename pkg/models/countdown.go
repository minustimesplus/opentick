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
	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// CountdownType is the kind of date a countdown tracks.
type CountdownType string

const (
	// CountdownTypeCountdown counts the days until (or since) a one-off date.
	CountdownTypeCountdown CountdownType = "countdown"
	// CountdownTypeAnniversary counts the days since a date and its yearly returns.
	CountdownTypeAnniversary CountdownType = "anniversary"
	// CountdownTypeBirthday is an anniversary of someone's birth.
	CountdownTypeBirthday CountdownType = "birthday"
)

func (t CountdownType) valid() bool {
	switch t {
	case CountdownTypeCountdown, CountdownTypeAnniversary, CountdownTypeBirthday:
		return true
	}
	return false
}

// Countdown is a date a user counts down to or up from, like a trip, an
// anniversary or a birthday. The days left or passed are computed by clients
// in the user's own timezone.
type Countdown struct {
	ID           int64         `xorm:"bigint autoincr not null unique pk" json:"id" param:"countdown" readOnly:"true" doc:"The unique, numeric id of this countdown."`
	UserID       int64         `xorm:"bigint not null index" json:"-"`
	Title        string        `xorm:"varchar(250) not null" json:"title" valid:"runelength(1|250)" minLength:"1" maxLength:"250" doc:"What is counted down to, e.g. \"Trip to Japan\" or \"Anna's birthday\"."`
	Date         time.Time     `xorm:"datetime not null" json:"date" doc:"The date counted to or from. Only the calendar date in UTC is used; send it as midnight UTC (e.g. 2026-12-24T00:00:00Z)."`
	Type         CountdownType `xorm:"varchar(20) not null default 'countdown'" json:"type" enum:"countdown,anniversary,birthday" doc:"The kind of date. Defaults to countdown."`
	RepeatYearly bool          `xorm:"bool not null default false" json:"repeat_yearly" doc:"Whether the date comes back every year. Clients then count to its next occurrence and show how many years have passed since the original date."`
	Icon         string        `xorm:"varchar(50) null" json:"icon" maxLength:"50" doc:"An emoji shown next to the countdown."`
	HexColor     string        `xorm:"varchar(6) null" json:"hex_color" valid:"hexcolor" maxLength:"7" doc:"The color of the countdown, in hex format."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this countdown was created. You cannot change this value."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this countdown was last updated. You cannot change this value."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for countdowns.
func (*Countdown) TableName() string {
	return "countdowns"
}

func (c *Countdown) normalize() error {
	if c.Type == "" {
		c.Type = CountdownTypeCountdown
	}
	if !c.Type.valid() {
		return ErrCountdownInvalidType{Type: string(c.Type)}
	}
	// Only the day matters; dropping the time keeps every client on the same date.
	d := c.Date.UTC()
	c.Date = time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	c.HexColor = utils.NormalizeHex(c.HexColor)
	return nil
}

// Create adds a countdown for the authenticated user.
func (c *Countdown) Create(s *xorm.Session, a web.Auth) error {
	if err := c.normalize(); err != nil {
		return err
	}
	c.ID = 0
	c.UserID = a.GetID()
	_, err := s.Insert(c)
	return err
}

// ReadOne returns a countdown. CanRead already loaded it.
func (c *Countdown) ReadOne(_ *xorm.Session, _ web.Auth) error {
	return nil
}

// ReadAll returns all countdowns of the authenticated user, soonest date first.
func (c *Countdown) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result any, resultCount int, numberOfTotalItems int64, err error) {
	// DoReadAll does not call CanRead, so link shares must be refused here.
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, 0, 0, ErrGenericForbidden{}
	}

	cond := builder.NewCond().And(builder.Eq{"user_id": a.GetID()})
	if search != "" {
		cond = cond.And(db.ILIKE("title", search))
	}

	countdowns := []*Countdown{}
	query := s.Where(cond).OrderBy("date ASC, id ASC")
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	if err := query.Find(&countdowns); err != nil {
		return nil, 0, 0, err
	}

	total, err := s.Where(cond).Count(&Countdown{})
	if err != nil {
		return nil, 0, 0, err
	}
	return countdowns, len(countdowns), total, nil
}

// Update changes a countdown.
func (c *Countdown) Update(s *xorm.Session, _ web.Auth) error {
	if err := c.normalize(); err != nil {
		return err
	}
	if _, err := s.ID(c.ID).Cols("title", "date", "type", "repeat_yearly", "icon", "hex_color").Update(c); err != nil {
		return err
	}
	return c.load(s)
}

// Delete removes a countdown.
func (c *Countdown) Delete(s *xorm.Session, _ web.Auth) error {
	_, err := s.ID(c.ID).Delete(&Countdown{})
	return err
}

func (c *Countdown) load(s *xorm.Session) error {
	exists, err := s.ID(c.ID).Get(c)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCountdownDoesNotExist{ID: c.ID}
	}
	return nil
}
