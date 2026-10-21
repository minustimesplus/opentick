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

package apiv2

import (
	"context"
	"net/http"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/modules/auth"
	"code.vikunja.io/api/pkg/modules/auth/clerk"
	"code.vikunja.io/api/pkg/modules/humabridge"

	"github.com/danielgtaylor/huma/v2"
)

func init() { AddRouteRegistrar(RegisterClerkRoutes) }

// RegisterClerkRoutes wires the Clerk token exchange. It is only registered
// when Clerk auth is enabled.
func RegisterClerkRoutes(api huma.API) {
	if !config.AuthClerkEnabled.GetBool() {
		return
	}

	Register(api, huma.Operation{
		OperationID:   "auth-clerk-exchange",
		Summary:       "Authenticate with Clerk",
		Description:   "Exchanges a Clerk session token for a Vikunja JWT, creating the matching user on first sign in. The Clerk user needs a verified primary email. A long-lived refresh token is set as an HttpOnly cookie. When the user has 2FA enabled in Vikunja, the call returns 412 and must be retried with totp_passcode set.",
		Method:        http.MethodPost,
		Path:          "/auth/clerk",
		DefaultStatus: http.StatusOK,
		Tags:          []string{"auth"},
		Security:      publicSecurity,
	}, authClerkExchange)
}

func authClerkExchange(ctx context.Context, in *struct {
	Body clerk.Exchange
}) (*authTokenBody, error) {
	u, err := clerk.Authenticate(ctx, &in.Body)
	if err != nil {
		return nil, translateDomainError(err)
	}

	deviceInfo, ipAddress := requestClientInfo(ctx)
	// Clerk keeps its own long session, so a short one here re-exchanges cheaply.
	token, err := auth.IssueUserToken(ctx, u, deviceInfo, ipAddress, false, nil)
	if err != nil {
		return nil, translateDomainError(err)
	}

	if ec := humabridge.EchoContextFrom(ctx); ec != nil {
		auth.WriteUserAuthCookies(ec, token)
	}

	out := &authTokenBody{CacheControl: "no-store"}
	out.Body.Token = token.AccessToken
	return out, nil
}
