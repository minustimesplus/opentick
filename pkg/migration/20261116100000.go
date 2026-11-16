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

type habitsTracksWeight20261116100000 struct {
	TracksWeight bool `xorm:"bool not null default false"`
}

func (habitsTracksWeight20261116100000) TableName() string {
	return "habits"
}

type weights20261116100000 struct {
	ID      int64     `xorm:"bigint autoincr not null unique pk"`
	UserID  int64     `xorm:"bigint not null unique(user_day)"`
	Day     time.Time `xorm:"datetime not null unique(user_day)"`
	Weight  float64   `xorm:"double not null"`
	Created time.Time `xorm:"created not null"`
	Updated time.Time `xorm:"updated not null"`
}

func (weights20261116100000) TableName() string {
	return "weights"
}

func addWeights20261116100000(tx *xorm.Engine) error {
	if err := partialSync(tx, habitsTracksWeight20261116100000{}); err != nil {
		return err
	}
	return tx.Sync(weights20261116100000{}) //nolint:forbidigo // brand-new table, nothing to drop
}

func dropWeights20261116100000(tx *xorm.Engine) error {
	if err := tx.DropTables(weights20261116100000{}); err != nil {
		return err
	}
	return dropTableColum(tx, "habits", "tracks_weight")
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261116100000",
		Description: "Add weights and weigh-in habits",
		Migrate:     addWeights20261116100000,
		Rollback:    dropWeights20261116100000,
	})
}
