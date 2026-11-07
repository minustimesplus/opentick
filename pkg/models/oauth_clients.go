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

package models

import (
	"slices"
	"time"

	"code.vikunja.io/api/pkg/utils"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// OAuthClientIDPrefix marks client IDs issued by dynamic client registration, so
// they can never be confused with first-party client IDs such as "vikunja-desktop".
const OAuthClientIDPrefix = "oc_"

// OAuthClient is a public OAuth client registered through dynamic client
// registration (RFC 7591), e.g. Claude connecting to the MCP endpoint.
type OAuthClient struct {
	ID           int64     `xorm:"bigint autoincr not null unique pk" json:"-"`
	ClientID     string    `xorm:"varchar(64) not null unique" json:"client_id" readOnly:"true" doc:"The issued client identifier."`
	Name         string    `xorm:"varchar(255) not null" json:"client_name" doc:"Human-readable client name, shown on the consent screen."`
	RedirectURIs []string  `xorm:"json not null" json:"redirect_uris" doc:"Redirect URIs the client may use. An authorization request must match one exactly."`
	Created      time.Time `xorm:"created not null" json:"-"`
}

func (*OAuthClient) TableName() string {
	return "oauth_clients"
}

// CreateOAuthClient registers a new public client and returns it with its issued client ID.
func CreateOAuthClient(s *xorm.Session, name string, redirectURIs []string) (*OAuthClient, error) {
	id, err := utils.CryptoRandomString(32)
	if err != nil {
		return nil, err
	}
	c := &OAuthClient{
		ClientID:     OAuthClientIDPrefix + id,
		Name:         name,
		RedirectURIs: redirectURIs,
	}
	if _, err := s.Insert(c); err != nil {
		return nil, err
	}
	return c, nil
}

// GetOAuthClient returns the registered client or ErrOAuthClientNotFound.
func GetOAuthClient(s *xorm.Session, clientID string) (*OAuthClient, error) {
	c := &OAuthClient{}
	has, err := s.Where(builder.Eq{"client_id": clientID}).Get(c)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, &ErrOAuthClientNotFound{}
	}
	return c, nil
}

// AllowsRedirectURI reports whether uri exactly matches a registered redirect URI.
func (c *OAuthClient) AllowsRedirectURI(uri string) bool {
	return slices.Contains(c.RedirectURIs, uri)
}
