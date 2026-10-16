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
	"context"
	"errors"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/modules/icalfeed"
	"code.vikunja.io/api/pkg/utils"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// MaxCalendarSubscriptionRange is the longest time range events can be requested for at once.
const MaxCalendarSubscriptionRange = 366 * 24 * time.Hour

// CalendarSubscription is an external iCalendar (ICS) feed a user subscribed to.
// Its events are shown read-only next to the user's tasks.
type CalendarSubscription struct {
	ID       int64  `xorm:"bigint autoincr not null unique pk" json:"id" param:"calendarsubscription" readOnly:"true" doc:"The unique, numeric id of this calendar subscription."`
	UserID   int64  `xorm:"bigint not null index" json:"-"`
	Title    string `xorm:"varchar(250) not null" json:"title" valid:"runelength(1|250)" minLength:"1" maxLength:"250" doc:"The display name of the calendar."`
	URL      string `xorm:"text not null" json:"url" valid:"required" minLength:"1" maxLength:"2048" doc:"The http(s) or webcal(s) url of the ICS feed. webcal urls are stored as https. Private feed urls often contain a secret token; only the owner can read it."`
	HexColor string `xorm:"varchar(6) null" json:"hex_color" valid:"hexcolor" maxLength:"7" doc:"The color the calendar's events are shown in, in hex format."`

	Created time.Time `xorm:"created not null" json:"created" readOnly:"true" doc:"A timestamp when this subscription was created. You cannot change this value."`
	Updated time.Time `xorm:"updated not null" json:"updated" readOnly:"true" doc:"A timestamp when this subscription was last updated. You cannot change this value."`

	web.CRUDable    `xorm:"-" json:"-"`
	web.Permissions `xorm:"-" json:"-"`
}

// TableName returns the table name for calendar subscriptions.
func (*CalendarSubscription) TableName() string {
	return "calendar_subscriptions"
}

func (cs *CalendarSubscription) normalize() error {
	u, err := icalfeed.NormalizeURL(cs.URL)
	if err != nil {
		return ErrCalendarSubscriptionInvalidURL{}
	}
	cs.URL = u
	cs.HexColor = utils.NormalizeHex(cs.HexColor)
	return nil
}

// Create subscribes the authenticated user to a calendar feed.
func (cs *CalendarSubscription) Create(s *xorm.Session, a web.Auth) error {
	if err := cs.normalize(); err != nil {
		return err
	}
	cs.ID = 0
	cs.UserID = a.GetID()
	_, err := s.Insert(cs)
	return err
}

// ReadOne returns a calendar subscription. CanRead already loaded it.
func (cs *CalendarSubscription) ReadOne(_ *xorm.Session, _ web.Auth) error {
	return nil
}

// ReadAll returns all calendar subscriptions of the authenticated user.
func (cs *CalendarSubscription) ReadAll(s *xorm.Session, a web.Auth, search string, page int, perPage int) (result any, resultCount int, numberOfTotalItems int64, err error) {
	// DoReadAll does not call CanRead, so link shares must be refused here.
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, 0, 0, ErrGenericForbidden{}
	}

	cond := builder.NewCond().And(builder.Eq{"user_id": a.GetID()})
	if search != "" {
		cond = cond.And(db.ILIKE("title", search))
	}

	subs := []*CalendarSubscription{}
	query := s.Where(cond).OrderBy("id ASC")
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	if err := query.Find(&subs); err != nil {
		return nil, 0, 0, err
	}

	total, err := s.Where(cond).Count(&CalendarSubscription{})
	if err != nil {
		return nil, 0, 0, err
	}
	return subs, len(subs), total, nil
}

// Update changes a calendar subscription.
func (cs *CalendarSubscription) Update(s *xorm.Session, _ web.Auth) error {
	if err := cs.normalize(); err != nil {
		return err
	}
	if _, err := s.ID(cs.ID).Cols("title", "url", "hex_color").Update(cs); err != nil {
		return err
	}
	// Show the effect of a changed url right away instead of after the cache expires.
	icalfeed.Invalidate(cs.URL)
	return cs.load(s)
}

// Delete removes a calendar subscription.
func (cs *CalendarSubscription) Delete(s *xorm.Session, _ web.Auth) error {
	_, err := s.ID(cs.ID).Delete(&CalendarSubscription{})
	return err
}

func (cs *CalendarSubscription) load(s *xorm.Session) error {
	exists, err := s.ID(cs.ID).Get(cs)
	if err != nil {
		return err
	}
	if !exists {
		return ErrCalendarSubscriptionDoesNotExist{ID: cs.ID}
	}
	return nil
}

// CalendarSubscriptionEvents is the expanded content of a subscribed calendar for a time range.
type CalendarSubscriptionEvents struct {
	SubscriptionID int64             `json:"subscription_id" doc:"The id of the calendar subscription these events belong to."`
	CalendarName   string            `json:"calendar_name" doc:"The calendar's own name from the feed (X-WR-CALNAME), if it sets one."`
	Events         []*icalfeed.Event `json:"events" doc:"The event occurrences overlapping the requested range, sorted by start. Recurring events are expanded into one entry per occurrence."`
}

// GetEvents fetches the subscribed feed and returns its events overlapping
// [from, to). All-day and floating events are placed in loc. Callers must
// have checked CanRead, which also loads the subscription.
func (cs *CalendarSubscription) GetEvents(ctx context.Context, from, to time.Time, loc *time.Location) (*CalendarSubscriptionEvents, error) {
	if !to.After(from) || to.Sub(from) > MaxCalendarSubscriptionRange {
		return nil, ErrCalendarSubscriptionInvalidRange{}
	}

	feed, err := icalfeed.Fetch(ctx, cs.URL)
	if err != nil {
		var fe *icalfeed.FetchError
		if errors.As(err, &fe) {
			return nil, ErrCalendarSubscriptionFetchFailed{Reason: fe.Reason}
		}
		if errors.Is(err, icalfeed.ErrInvalidURL) {
			return nil, ErrCalendarSubscriptionInvalidURL{}
		}
		return nil, err
	}

	return &CalendarSubscriptionEvents{
		SubscriptionID: cs.ID,
		CalendarName:   feed.Name,
		Events:         feed.Expand(from, to, loc),
	}, nil
}
