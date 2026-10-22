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

type countdowns20261022100000 struct {
	ID           int64     `xorm:"bigint autoincr not null unique pk"`
	UserID       int64     `xorm:"bigint not null index"`
	Title        string    `xorm:"varchar(250) not null"`
	Date         time.Time `xorm:"datetime not null"`
	Type         string    `xorm:"varchar(20) not null default 'countdown'"`
	RepeatYearly bool      `xorm:"bool not null default false"`
	Icon         string    `xorm:"varchar(50) null"`
	HexColor     string    `xorm:"varchar(6) null"`
	Created      time.Time `xorm:"created not null"`
	Updated      time.Time `xorm:"updated not null"`
}

func (countdowns20261022100000) TableName() string {
	return "countdowns"
}

func addCountdowns20261022100000(tx *xorm.Engine) error {
	return tx.Sync(countdowns20261022100000{}) //nolint:forbidigo // brand-new table, nothing to drop
}

func dropCountdowns20261022100000(tx *xorm.Engine) error {
	return tx.DropTables(countdowns20261022100000{})
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261022100000",
		Description: "Add countdowns and anniversaries",
		Migrate:     addCountdowns20261022100000,
		Rollback:    dropCountdowns20261022100000,
	})
}
