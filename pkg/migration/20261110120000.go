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

type agentConfigs20261110120000 struct {
	ID            int64     `xorm:"bigint autoincr not null unique pk"`
	UserID        int64     `xorm:"bigint not null unique"`
	ClaudeFireURL string    `xorm:"varchar(500) null"`
	ClaudeToken   string    `xorm:"text null"`
	InstinctEmail string    `xorm:"varchar(250) null"`
	SystemPrompt  string    `xorm:"longtext null"`
	Created       time.Time `xorm:"created not null"`
	Updated       time.Time `xorm:"updated not null"`
}

func (agentConfigs20261110120000) TableName() string {
	return "agent_configs"
}

type projectAgentSettings20261110120000 struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk"`
	ProjectID   int64     `xorm:"bigint not null unique"`
	AIContext   string    `xorm:"longtext null 'ai_context'"`
	Agent       string    `xorm:"varchar(20) not null default ''"`
	AgentUserID int64     `xorm:"bigint not null default 0"`
	Created     time.Time `xorm:"created not null"`
	Updated     time.Time `xorm:"updated not null"`
}

func (projectAgentSettings20261110120000) TableName() string {
	return "project_agent_settings"
}

type agentDispatches20261110120000 struct {
	ID          int64     `xorm:"bigint autoincr not null unique pk"`
	TaskID      int64     `xorm:"bigint not null INDEX"`
	Agent       string    `xorm:"varchar(20) not null"`
	Status      string    `xorm:"varchar(20) not null"`
	ExtraPrompt string    `xorm:"text null"`
	SessionURL  string    `xorm:"varchar(500) null"`
	Error       string    `xorm:"text null"`
	Automatic   bool      `xorm:"bool not null default false"`
	CreatedByID int64     `xorm:"bigint not null INDEX"`
	Created     time.Time `xorm:"created not null"`
}

func (agentDispatches20261110120000) TableName() string {
	return "agent_dispatches"
}

func addAgents20261110120000(tx *xorm.Engine) error {
	//nolint:forbidigo // brand-new tables, nothing to drop
	return tx.Sync(agentConfigs20261110120000{}, projectAgentSettings20261110120000{}, agentDispatches20261110120000{})
}

func dropAgents20261110120000(tx *xorm.Engine) error {
	return tx.DropTables(agentConfigs20261110120000{}, projectAgentSettings20261110120000{}, agentDispatches20261110120000{})
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261110120000",
		Description: "Add AI agent configs, per-list agent settings and agent dispatches",
		Migrate:     addAgents20261110120000,
		Rollback:    dropAgents20261110120000,
	})
}
