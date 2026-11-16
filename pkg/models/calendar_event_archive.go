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
	"crypto/sha256"
	"encoding/hex"
	"time"

	"code.vikunja.io/api/pkg/modules/icalfeed"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// CalendarEventArchive marks one occurrence of a subscribed calendar's event
// as archived. Feeds are read-only, so this is the only state kept per event.
type CalendarEventArchive struct {
	ID             int64 `xorm:"bigint autoincr not null unique pk"`
	SubscriptionID int64 `xorm:"bigint not null unique(subscription_event)"`
	// The SHA-256 of the occurrence id (UID plus start): UIDs have no length limit.
	EventHash string    `xorm:"varchar(64) not null unique(subscription_event)"`
	Created   time.Time `xorm:"created not null"`
}

// TableName returns the table name for archived calendar events.
func (*CalendarEventArchive) TableName() string {
	return "calendar_event_archives"
}

func calendarEventHash(eventID string) string {
	sum := sha256.Sum256([]byte(eventID))
	return hex.EncodeToString(sum[:])
}

// SetEventArchived archives or restores one event occurrence of the
// subscription. Both are idempotent. Callers must have checked CanUpdate.
func (cs *CalendarSubscription) SetEventArchived(s *xorm.Session, eventID string, archived bool) error {
	cond := builder.Eq{"subscription_id": cs.ID, "event_hash": calendarEventHash(eventID)}
	if !archived {
		_, err := s.Where(cond).Delete(&CalendarEventArchive{})
		return err
	}
	exists, err := s.Where(cond).Exist(&CalendarEventArchive{})
	if err != nil || exists {
		return err
	}
	_, err = s.Insert(&CalendarEventArchive{SubscriptionID: cs.ID, EventHash: calendarEventHash(eventID)})
	return err
}

// MarkArchivedEvents sets Archived on the events the user archived.
func (cs *CalendarSubscription) MarkArchivedEvents(s *xorm.Session, events []*icalfeed.Event) error {
	if len(events) == 0 {
		return nil
	}
	archives := []*CalendarEventArchive{}
	if err := s.Where("subscription_id = ?", cs.ID).Cols("event_hash").Find(&archives); err != nil {
		return err
	}
	if len(archives) == 0 {
		return nil
	}
	archived := make(map[string]bool, len(archives))
	for _, a := range archives {
		archived[a.EventHash] = true
	}
	for _, e := range events {
		e.Archived = archived[calendarEventHash(e.ID)]
	}
	return nil
}

func deleteCalendarEventArchives(s *xorm.Session, cond builder.Cond) error {
	_, err := s.Where(cond).Delete(&CalendarEventArchive{})
	return err
}
