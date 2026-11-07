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

type oauthClients20261107120000 struct {
	ID           int64     `xorm:"bigint autoincr not null unique pk"`
	ClientID     string    `xorm:"varchar(64) not null unique"`
	Name         string    `xorm:"varchar(255) not null"`
	RedirectURIs []string  `xorm:"json not null"`
	Created      time.Time `xorm:"created not null"`
}

func (oauthClients20261107120000) TableName() string {
	return "oauth_clients"
}

type oauthGrants20261107120000 struct {
	ID                 int64     `xorm:"bigint autoincr not null unique pk"`
	UserID             int64     `xorm:"bigint not null INDEX"`
	ClientID           string    `xorm:"varchar(64) not null"`
	APITokenID         int64     `xorm:"bigint not null INDEX"`
	RefreshTokenSha256 string    `xorm:"varchar(64) not null unique"`
	ExpiresAt          time.Time `xorm:"not null"`
	Created            time.Time `xorm:"created not null"`
	Updated            time.Time `xorm:"updated not null"`
}

func (oauthGrants20261107120000) TableName() string {
	return "oauth_grants"
}

type oauthCodes20261107120000 struct {
	Permissions map[string][]string `xorm:"json null"`
	Resource    string              `xorm:"text null"`
}

func (oauthCodes20261107120000) TableName() string {
	return "oauth_codes"
}

func init() {
	migrations = append(migrations, &xormigrate.Migration{
		ID:          "20261107120000",
		Description: "Add OAuth dynamic client registration and MCP grants",
		Migrate: func(tx *xorm.Engine) error {
			if err := tx.Sync(oauthClients20261107120000{}, oauthGrants20261107120000{}); err != nil { //nolint:forbidigo // brand-new tables, nothing to drop
				return err
			}
			return partialSync(tx, oauthCodes20261107120000{})
		},
		Rollback: func(tx *xorm.Engine) error {
			return tx.DropTables(oauthClients20261107120000{}, oauthGrants20261107120000{})
		},
	})
}
