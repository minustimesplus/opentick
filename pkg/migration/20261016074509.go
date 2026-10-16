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

type calendarSubscriptions20261016074509 struct {
	ID       int64     `xorm:"bigint autoincr not null unique pk"`
	UserID   int64     `xorm:"bigint not null index"`
	Title    string    `xorm:"varchar(250) not null"`
	URL      string    `xorm:"text not null"`
	HexColor string    `xorm:"varchar(6) null"`
	Created  time.Time `xorm:"created not null"`
	Updated  time.Time `xorm:"updated not null"`
}

func (calendarSubscriptions20261016074509) TableName() string {
	return "calendar_subscriptions"
}

func addCalendarSubscriptions20261016074509(tx *xorm.Engine) error {
	return tx.Sync(calendarSubscriptions20261016074509{}) //nolint:forbidigo // brand-new table, nothing to drop
}

func dropCalendarSubscriptions20261016074509(tx *xorm.Engine) error {
	return tx.DropTables(calendarSubscriptions20261016074509{})
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261016074509",
		Description: "Add external calendar (ICS) subscriptions",
		Migrate:     addCalendarSubscriptions20261016074509,
		Rollback:    dropCalendarSubscriptions20261016074509,
	})
}
