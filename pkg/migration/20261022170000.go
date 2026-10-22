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

type focusSessions20261022170000 struct {
	ID        int64     `xorm:"bigint autoincr not null unique pk"`
	UserID    int64     `xorm:"bigint not null INDEX"`
	TaskID    int64     `xorm:"bigint null INDEX"`
	Type      string    `xorm:"varchar(20) not null"`
	StartTime time.Time `xorm:"not null INDEX"`
	EndTime   time.Time `xorm:"not null"`
	Duration  int64     `xorm:"bigint not null default 0"`
	Note      string    `xorm:"text null"`
	Created   time.Time `xorm:"created not null"`
	Updated   time.Time `xorm:"updated not null"`
}

func (focusSessions20261022170000) TableName() string {
	return "focus_sessions"
}

func addFocusSessions20261022170000(tx *xorm.Engine) error {
	return tx.Sync(focusSessions20261022170000{}) //nolint:forbidigo // brand-new table, nothing to drop
}

func dropFocusSessions20261022170000(tx *xorm.Engine) error {
	return tx.DropTables(focusSessions20261022170000{})
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261022170000",
		Description: "Add focus_sessions table for the pomodoro / focus timer",
		Migrate:     addFocusSessions20261022170000,
		Rollback:    dropFocusSessions20261022170000,
	})
}
