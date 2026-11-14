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
	"src.techknowlogick.com/xormigrate"
	"xorm.io/xorm"
)

type habitsPosition20261114130000 struct {
	Position float64 `xorm:"double not null default 0"`
}

func (habitsPosition20261114130000) TableName() string {
	return "habits"
}

// habitPositionStep spaces out positions so a habit can be moved between two others.
const habitPositionStep20261114130000 = 65536

func addHabitPosition20261114130000(tx *xorm.Engine) error {
	if err := partialSync(tx, habitsPosition20261114130000{}); err != nil {
		return err
	}
	// Keep the creation order habits were listed in until now.
	_, err := tx.Exec("UPDATE habits SET position = id * ? WHERE position = 0", habitPositionStep20261114130000)
	return err
}

func dropHabitPosition20261114130000(tx *xorm.Engine) error {
	return dropTableColum(tx, "habits", "position")
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261114130000",
		Description: "Add a manual position to habits",
		Migrate:     addHabitPosition20261114130000,
		Rollback:    dropHabitPosition20261114130000,
	})
}
