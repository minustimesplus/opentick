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

package migration

import (
	"time"

	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

type calendarEventArchives20261114130100 struct {
	ID             int64     `xorm:"bigint autoincr not null unique pk"`
	SubscriptionID int64     `xorm:"bigint not null unique(subscription_event)"`
	EventHash      string    `xorm:"varchar(64) not null unique(subscription_event)"`
	Created        time.Time `xorm:"created not null"`
}

func (calendarEventArchives20261114130100) TableName() string {
	return "calendar_event_archives"
}

func addCalendarEventArchives20261114130100(tx *xorm.Engine) error {
	return tx.Sync(calendarEventArchives20261114130100{}) //nolint:forbidigo // brand-new table, nothing to drop
}

func dropCalendarEventArchives20261114130100(tx *xorm.Engine) error {
	return tx.DropTables(calendarEventArchives20261114130100{})
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261114130100",
		Description: "Add archived events of subscribed calendars",
		Migrate:     addCalendarEventArchives20261114130100,
		Rollback:    dropCalendarEventArchives20261114130100,
	})
}
