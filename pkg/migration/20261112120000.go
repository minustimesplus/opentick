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

type agentConfigs20261112120000 struct {
	InboundSecret    string `xorm:"varchar(32) null INDEX 'inbound_secret'"`
	InboundSenders   string `xorm:"text null 'inbound_senders'"`
	InboundProjectID int64  `xorm:"bigint not null default 0 'inbound_project_id'"`
}

func (agentConfigs20261112120000) TableName() string {
	return "agent_configs"
}

type inboundMailMessages20261112120000 struct {
	ID         int64     `xorm:"bigint autoincr not null unique pk"`
	MessageKey string    `xorm:"varchar(64) not null unique 'message_key'"`
	UserID     int64     `xorm:"bigint not null default 0 INDEX"`
	Created    time.Time `xorm:"created not null"`
}

func (inboundMailMessages20261112120000) TableName() string {
	return "inbound_mail_messages"
}

func addInboundMail20261112120000(tx *xorm.Engine) error {
	if err := partialSync(tx, agentConfigs20261112120000{}); err != nil {
		return err
	}
	//nolint:forbidigo // brand-new table, nothing to drop
	return tx.Sync(inboundMailMessages20261112120000{})
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261112120000",
		Description: "Add inbound mail addresses to agent configs and remember processed inbound mails",
		Migrate:     addInboundMail20261112120000,
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(inboundMailMessages20261112120000{})
		},
	})
}
