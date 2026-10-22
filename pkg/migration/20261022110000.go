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

type habits20261022110000 struct {
	ID        int64     `xorm:"bigint autoincr not null unique pk"`
	UserID    int64     `xorm:"bigint not null index"`
	Title     string    `xorm:"varchar(250) not null"`
	Icon      string    `xorm:"varchar(50) null"`
	HexColor  string    `xorm:"varchar(6) null"`
	Frequency string    `xorm:"varchar(20) not null default 'daily'"`
	Weekdays  int       `xorm:"int not null default 127"`
	Goal      int       `xorm:"int not null default 1"`
	Unit      string    `xorm:"varchar(50) null"`
	Archived  bool      `xorm:"bool not null default false"`
	Created   time.Time `xorm:"created not null"`
	Updated   time.Time `xorm:"updated not null"`
}

func (habits20261022110000) TableName() string {
	return "habits"
}

type habitCheckins20261022110000 struct {
	ID      int64     `xorm:"bigint autoincr not null unique pk"`
	HabitID int64     `xorm:"bigint not null unique(habit_day)"`
	Day     time.Time `xorm:"datetime not null unique(habit_day)"`
	Value   int       `xorm:"int not null"`
	Created time.Time `xorm:"created not null"`
	Updated time.Time `xorm:"updated not null"`
}

func (habitCheckins20261022110000) TableName() string {
	return "habit_checkins"
}

func addHabits20261022110000(tx *xorm.Engine) error {
	return tx.Sync(habits20261022110000{}, habitCheckins20261022110000{}) //nolint:forbidigo // brand-new tables, nothing to drop
}

func dropHabits20261022110000(tx *xorm.Engine) error {
	return tx.DropTables(habitCheckins20261022110000{}, habits20261022110000{})
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261022110000",
		Description: "Add habits and habit check-ins",
		Migrate:     addHabits20261022110000,
		Rollback:    dropHabits20261022110000,
	})
}
