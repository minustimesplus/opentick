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

// Package clerk signs users in with Clerk (https://clerk.com). The frontend
// runs Clerk's sign-in flows itself and hands the resulting session token to
// the API, which verifies it and exchanges it for a regular user session.
package clerk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/auth"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/utils"

	"github.com/coreos/go-oidc/v3/oidc"
	"xorm.io/xorm"
)

// Exchange is the request body of the token exchange.
type Exchange struct {
	Token        string `json:"token" minLength:"1" doc:"The Clerk session token (a JWT) from the signed in Clerk session."`
	TOTPPasscode string `json:"totp_passcode,omitempty" doc:"The TOTP passcode, required when the user enabled two-factor authentication in this app."`
}

// Overridden in tests, which serve the frontend and backend APIs over plain http.
var (
	issuerScheme  = "https"
	backendAPIURL = "https://api.clerk.com/v1"
)

var httpClient = sync.OnceValue(utils.NewUnguardedHTTPClient)

// IssuerFromPublishableKey returns the issuer of a Clerk instance's session
// tokens. The publishable key encodes the instance's frontend API host as
// base64("<host>$").
func IssuerFromPublishableKey(publishableKey string) (string, error) {
	parts := strings.SplitN(publishableKey, "_", 3)
	if len(parts) != 3 || parts[0] != "pk" {
		return "", fmt.Errorf("malformed Clerk publishable key")
	}
	encoded := strings.TrimRight(parts[2], "=")
	decoded, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("malformed Clerk publishable key: %w", err)
	}
	host := strings.TrimSuffix(string(decoded), "$")
	if host == "" || strings.ContainsAny(host, "/?#@ ") {
		return "", fmt.Errorf("malformed Clerk publishable key")
	}
	return issuerScheme + "://" + host, nil
}

var (
	verifierMu     sync.Mutex
	verifier       *oidc.IDTokenVerifier
	verifierIssuer string
)

// getVerifier caches one verifier per issuer so the JWKS is fetched once and
// then only again when Clerk rotates its signing keys.
func getVerifier() (*oidc.IDTokenVerifier, string, error) {
	issuer, err := IssuerFromPublishableKey(config.AuthClerkPublishableKey.GetString())
	if err != nil {
		return nil, "", err
	}

	verifierMu.Lock()
	defer verifierMu.Unlock()
	if verifier == nil || verifierIssuer != issuer {
		keyCtx := oidc.ClientContext(context.Background(), httpClient())
		keySet := oidc.NewRemoteKeySet(keyCtx, issuer+"/.well-known/jwks.json")
		// Clerk session tokens carry no audience; the issuer and signature
		// tie them to this instance.
		verifier = oidc.NewVerifier(issuer, keySet, &oidc.Config{SkipClientIDCheck: true})
		verifierIssuer = issuer
	}
	return verifier, issuer, nil
}

type clerkEmail struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
	Verification *struct {
		Status string `json:"status"`
	} `json:"verification"`
}

type clerkUser struct {
	ID                    string       `json:"id"`
	Username              string       `json:"username"`
	FirstName             string       `json:"first_name"`
	LastName              string       `json:"last_name"`
	PrimaryEmailAddressID string       `json:"primary_email_address_id"`
	EmailAddresses        []clerkEmail `json:"email_addresses"`
}

// verifiedPrimaryEmail returns the user's primary email when Clerk verified it.
func (u *clerkUser) verifiedPrimaryEmail() string {
	for _, e := range u.EmailAddresses {
		if e.ID == u.PrimaryEmailAddressID && e.Verification != nil && e.Verification.Status == "verified" {
			return e.EmailAddress
		}
	}
	return ""
}

func (u *clerkUser) displayName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

// fetchUser loads the profile from Clerk's backend API, since session tokens
// only carry the user id.
func fetchUser(ctx context.Context, userID string) (*clerkUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, backendAPIURL+"/users/"+url.PathEscape(userID), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+config.AuthClerkSecretKey.GetString())
	req.Header.Set("Accept", "application/json")

	res, err := httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Clerk: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, fmt.Errorf("clerk returned %d for user %s: %s", res.StatusCode, userID, body)
	}

	cu := &clerkUser{}
	if err := json.NewDecoder(res.Body).Decode(cu); err != nil {
		return nil, fmt.Errorf("could not decode Clerk user: %w", err)
	}
	return cu, nil
}

// Authenticate verifies a Clerk session token and returns the matching user,
// creating it on first sign in. The caller issues the session.
func Authenticate(ctx context.Context, in *Exchange) (*user.User, error) {
	v, issuer, err := getVerifier() //nolint:contextcheck // the cached key set outlives this request, so it fetches keys on a background context.
	if err != nil {
		return nil, err
	}

	token, err := v.Verify(ctx, in.Token)
	if err != nil {
		return nil, &models.ErrClerkInvalidToken{Reason: err.Error()}
	}

	cu, err := fetchUser(ctx, token.Subject)
	if err != nil {
		return nil, err
	}
	email := cu.verifiedPrimaryEmail()
	if email == "" {
		return nil, &models.ErrClerkEmailNotVerified{}
	}

	s := db.NewSession()
	defer s.Close()
	// Discards events queued during a rolled-back transaction.
	defer events.CleanupPending(s)

	u, err := getOrCreateUser(s, issuer, token.Subject, cu, email)
	if err != nil {
		_ = s.Rollback()
		return nil, err
	}

	if u.Status == user.StatusDisabled {
		_ = s.Rollback()
		return nil, &user.ErrAccountDisabled{UserID: u.ID}
	}
	if u.Status == user.StatusAccountLocked {
		_ = s.Rollback()
		return nil, &user.ErrAccountLocked{UserID: u.ID}
	}

	// Commit before HandleFailedTOTPAuth so its own session can take the
	// SQLite write lock, as the OIDC flow does.
	if err := auth.EnforceTOTPIfRequired(s, u, in.TOTPPasscode); err != nil {
		if commitErr := s.Commit(); commitErr != nil {
			log.Errorf("Error committing session after failed Clerk TOTP attempt for user %d: %v", u.ID, commitErr)
		} else {
			events.DispatchPending(ctx, s)
		}
		if user.IsErrInvalidTOTPPasscode(err) {
			user.HandleFailedTOTPAuth(u)
		}
		return nil, err
	}

	if err := s.Commit(); err != nil {
		return nil, err
	}
	events.DispatchPending(ctx, s)

	return u, nil
}

func getOrCreateUser(s *xorm.Session, issuer, subject string, cu *clerkUser, email string) (*user.User, error) {
	u, err := user.GetUserWithEmail(s, &user.User{Issuer: issuer, Subject: subject})
	if user.IsErrUserStatusError(err) {
		// Disabled or locked: the caller rejects it, so leave the profile alone.
		return u, nil
	}
	if err != nil && !user.IsErrUserDoesNotExist(err) {
		return nil, err
	}

	if err == nil {
		if u.Email == email && u.Name == cu.displayName() {
			return u, nil
		}
		u.Email = email
		u.Name = cu.displayName()
		return user.UpdateUser(s, u, false)
	}

	username := cu.Username
	if username == "" {
		username, _, _ = strings.Cut(email, "@")
	}
	return auth.CreateUserWithRandomUsername(s, &user.User{
		Username: strings.ReplaceAll(username, " ", "-"),
		Email:    email,
		Name:     cu.displayName(),
		Status:   user.StatusActive,
		Issuer:   issuer,
		Subject:  subject,
	})
}
