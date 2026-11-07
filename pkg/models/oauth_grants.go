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
	"encoding/hex"
	"time"

	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/utils"

	"xorm.io/builder"
	"xorm.io/xorm"
)

const (
	// OAuthGrantAccessTokenTTL is how long an API token minted for an OAuth grant lives before the client must refresh.
	OAuthGrantAccessTokenTTL = time.Hour
	// OAuthGrantRefreshTTL is the sliding lifetime of a grant's refresh token; every refresh extends it.
	OAuthGrantRefreshTTL = 90 * 24 * time.Hour
)

// OAuthGrant ties a third-party OAuth client (registered via DCR) to the API
// token it acts with. The access token handed to the client is that API
// token, so scoping, expiry and revocation reuse the API token machinery:
// deleting the token in settings revokes the grant on its next refresh.
type OAuthGrant struct {
	ID                 int64     `xorm:"bigint autoincr not null unique pk" json:"-"`
	UserID             int64     `xorm:"bigint not null INDEX" json:"-"`
	ClientID           string    `xorm:"varchar(64) not null" json:"-"`
	APITokenID         int64     `xorm:"bigint not null INDEX" json:"-"`
	RefreshTokenSha256 string    `xorm:"varchar(64) not null unique" json:"-"`
	ExpiresAt          time.Time `xorm:"not null" json:"-"`
	Created            time.Time `xorm:"created not null" json:"-"`
	Updated            time.Time `xorm:"updated not null" json:"-"`
}

func (*OAuthGrant) TableName() string {
	return "oauth_grants"
}

// OAuthGrantTokens is what the token endpoint returns for a grant.
type OAuthGrantTokens struct {
	AccessToken  string
	RefreshToken string
	ExpiresIn    int64
}

// CreateOAuthGrant mints a short-lived API token with permissions for u and
// a refresh token bound to client.
func CreateOAuthGrant(s *xorm.Session, u *user.User, client *OAuthClient, permissions APIPermissions) (*OAuthGrantTokens, error) {
	title := "MCP: " + client.Name
	if len(title) > 250 {
		title = title[:250]
	}
	token := &APIToken{
		Title:          title,
		APIPermissions: permissions,
		ExpiresAt:      time.Now().Add(OAuthGrantAccessTokenTTL),
	}
	if err := token.Create(s, u); err != nil {
		return nil, err
	}

	refresh, refreshHash, err := generateHashedToken()
	if err != nil {
		return nil, err
	}
	grant := &OAuthGrant{
		UserID:             u.ID,
		ClientID:           client.ClientID,
		APITokenID:         token.ID,
		RefreshTokenSha256: refreshHash,
		ExpiresAt:          time.Now().Add(OAuthGrantRefreshTTL),
	}
	if _, err := s.Insert(grant); err != nil {
		return nil, err
	}

	return &OAuthGrantTokens{
		AccessToken:  token.Token,
		RefreshToken: refresh,
		ExpiresIn:    int64(OAuthGrantAccessTokenTTL.Seconds()),
	}, nil
}

// RefreshOAuthGrant rotates both the grant's refresh token and its API token
// secret. found is false when rawRefresh belongs to no grant, so the caller can
// fall back to session refresh tokens. A grant whose API token was deleted or
// whose refresh token expired is removed and reported as ErrOAuthGrantInvalid.
// A non-empty clientID must match the client the grant was issued to.
func RefreshOAuthGrant(s *xorm.Session, rawRefresh, clientID string) (tokens *OAuthGrantTokens, found bool, err error) {
	oldHash := HashSessionToken(rawRefresh)
	grant := &OAuthGrant{}
	has, err := s.Where(builder.Eq{"refresh_token_sha256": oldHash}).Get(grant)
	if err != nil || !has {
		return nil, false, err
	}

	if clientID != "" && clientID != grant.ClientID {
		return nil, true, &ErrOAuthGrantInvalid{}
	}

	revoke := func() (*OAuthGrantTokens, bool, error) {
		if _, err := s.Where("id = ?", grant.ID).Delete(&OAuthGrant{}); err != nil {
			return nil, true, err
		}
		return nil, true, &ErrOAuthGrantInvalid{}
	}

	if time.Now().After(grant.ExpiresAt) {
		return revoke()
	}
	exists, err := s.Where("id = ? AND owner_id = ?", grant.APITokenID, grant.UserID).Exist(&APIToken{})
	if err != nil {
		return nil, true, err
	}
	if !exists {
		return revoke()
	}

	newRefresh, newRefreshHash, err := generateHashedToken()
	if err != nil {
		return nil, true, err
	}
	// Matching on the old hash makes a concurrent replay of the same refresh token lose.
	affected, err := s.Where("id = ? AND refresh_token_sha256 = ?", grant.ID, oldHash).
		Cols("refresh_token_sha256", "expires_at").
		Update(&OAuthGrant{RefreshTokenSha256: newRefreshHash, ExpiresAt: time.Now().Add(OAuthGrantRefreshTTL)})
	if err != nil {
		return nil, true, err
	}
	if affected == 0 {
		return nil, true, &ErrOAuthGrantInvalid{}
	}

	secret, err := utils.CryptoRandomBytes(20)
	if err != nil {
		return nil, true, err
	}
	access := APITokenPrefix + hex.EncodeToString(secret)
	if _, err := s.Where("id = ?", grant.APITokenID).
		Cols("token_sha256", "expires_at").
		Update(&APIToken{TokenSha256: HashAPIToken(access), ExpiresAt: time.Now().Add(OAuthGrantAccessTokenTTL)}); err != nil {
		return nil, true, err
	}

	return &OAuthGrantTokens{
		AccessToken:  access,
		RefreshToken: newRefresh,
		ExpiresIn:    int64(OAuthGrantAccessTokenTTL.Seconds()),
	}, true, nil
}
