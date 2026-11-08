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

type projectInvitations20261108120000 struct {
	ID         int64      `xorm:"bigint autoincr not null unique pk"`
	ProjectID  int64      `xorm:"bigint not null INDEX"`
	InviterID  int64      `xorm:"bigint not null INDEX"`
	InviteeID  int64      `xorm:"bigint not null default 0 INDEX"`
	Email      string     `xorm:"varchar(250) null INDEX"`
	Kind       string     `xorm:"varchar(10) not null"`
	Permission int64      `xorm:"bigint not null default 0"`
	TokenHash  string     `xorm:"varchar(64) null INDEX"`
	LinkToken  string     `xorm:"varchar(64) null"`
	Uses       int64      `xorm:"bigint not null default 0"`
	ExpiresAt  *time.Time `xorm:"datetime null"`
	Created    time.Time  `xorm:"created not null"`
	Updated    time.Time  `xorm:"updated not null"`
}

func (projectInvitations20261108120000) TableName() string {
	return "project_invitations"
}

type usersCollaboration20261108120000 struct {
	ConfirmProjectInvites   bool `xorm:"bool not null default false"`
	NotifySharedTaskDone    bool `xorm:"bool not null default false"`
	NotifySharedTaskCreated bool `xorm:"bool not null default false"`
	NotifySharedTaskDeleted bool `xorm:"bool not null default false"`
}

func (usersCollaboration20261108120000) TableName() string {
	return "users"
}

var usersCollaborationColumns20261108120000 = []string{
	"confirm_project_invites",
	"notify_shared_task_done",
	"notify_shared_task_created",
	"notify_shared_task_deleted",
}

func addCollaboration20261108120000(tx *xorm.Engine) error {
	if err := tx.Sync(projectInvitations20261108120000{}); err != nil { //nolint:forbidigo // brand-new table, nothing to drop
		return err
	}
	return partialSync(tx, usersCollaboration20261108120000{})
}

func dropCollaboration20261108120000(tx *xorm.Engine) error {
	if err := tx.DropTables(projectInvitations20261108120000{}); err != nil {
		return err
	}
	for _, col := range usersCollaborationColumns20261108120000 {
		if err := dropTableColum(tx, "users", col); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261108120000",
		Description: "Add project invitations and collaboration settings",
		Migrate:     addCollaboration20261108120000,
		Rollback:    dropCollaboration20261108120000,
	})
}
