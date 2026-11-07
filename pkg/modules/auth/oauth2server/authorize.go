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

package oauth2server

import (
	"net/http"
	"strings"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"

	"github.com/labstack/echo/v5"
)

// AuthorizeRequest represents the body for the authorize endpoint.
type AuthorizeRequest struct {
	ResponseType        string `json:"response_type"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	State               string `json:"state"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	// Permissions picks the access preset (typed, read_only or full) for dynamically registered clients; ignored for first-party clients.
	Permissions string `json:"permissions,omitempty" enum:"typed,read_only,full" doc:"Access preset for dynamically registered clients. Defaults to typed."`
	// Resource is the RFC 8707 resource indicator from the authorization request.
	Resource string `json:"resource,omitempty"`
}

// PermissionsForPreset expands an access preset (typed, read_only, full) into
// API token permissions including mcp:access. The routes package sets it once
// the MCP module is built, since this package can't import that module.
var PermissionsForPreset func(preset string) (models.APIPermissions, bool)

// DefaultPermissionsPreset matches the default in the MCP settings UI.
const DefaultPermissionsPreset = "typed"

// AuthorizeResponse is returned on successful authorization code creation.
type AuthorizeResponse struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
	State       string `json:"state"`
}

// HandleAuthorize handles POST /oauth/authorize.
// It validates the OAuth parameters, creates an authorization code, and
// returns it as JSON. Authentication is handled by the token middleware.
// @Summary OAuth 2.0 authorize endpoint
// @Description Creates an authorization code for an OAuth 2.0 client on behalf of the authenticated user. PKCE is required. API tokens cannot be used to authorize a client.
// @tags auth
// @Accept json
// @Produce json
// @Security JWTKeyAuth
// @Param request body AuthorizeRequest true "The authorization request"
// @Success 200 {object} AuthorizeResponse "The authorization code and the redirect URI to return it to."
// @Failure 400 {object} web.HTTPError "response_type is not 'code', the redirect URI is invalid, or the PKCE challenge is missing."
// @Failure 403 {object} models.Message "An API token was used to authorize an OAuth client."
// @Failure 500 {object} models.Message "Internal server error."
// @Router /oauth/authorize [post]
func HandleAuthorize(c *echo.Context) error {
	if c.Get("api_token") != nil {
		return echo.NewHTTPError(http.StatusForbidden, "API tokens cannot be used to authorize OAuth clients")
	}

	var req AuthorizeRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
	}

	// Get the authenticated user from the middleware
	u, err := user.GetCurrentUser(c)
	if err != nil {
		return err
	}

	resp, err := Authorize(&req, u.ID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, resp)
}

// Authorize validates the OAuth authorization parameters for the given
// authenticated user and creates a single-use authorization code, independent
// of the HTTP layer. Callers own request binding and resolving the user.
func Authorize(req *AuthorizeRequest, userID int64) (*AuthorizeResponse, error) {
	// Validate response_type
	if req.ResponseType != "code" {
		return nil, echo.NewHTTPError(http.StatusBadRequest, "response_type must be 'code'")
	}

	// Validate PKCE (required)
	if req.CodeChallenge == "" || req.CodeChallengeMethod != "S256" {
		return nil, &models.ErrOAuthMissingPKCE{}
	}

	s := db.NewSession()
	defer s.Close()

	oauthCode := &models.OAuthCode{
		ClientID:            req.ClientID,
		RedirectURI:         req.RedirectURI,
		CodeChallenge:       req.CodeChallenge,
		CodeChallengeMethod: req.CodeChallengeMethod,
		Resource:            req.Resource,
	}

	if strings.HasPrefix(req.ClientID, models.OAuthClientIDPrefix) {
		// Dynamically registered clients may only use the redirect URIs they registered.
		client, err := models.GetOAuthClient(s, req.ClientID)
		if err != nil {
			_ = s.Rollback()
			return nil, err
		}
		if !client.AllowsRedirectURI(req.RedirectURI) {
			_ = s.Rollback()
			return nil, &models.ErrOAuthInvalidRedirectURI{}
		}
		preset := req.Permissions
		if preset == "" {
			preset = DefaultPermissionsPreset
		}
		if PermissionsForPreset == nil {
			_ = s.Rollback()
			return nil, echo.NewHTTPError(http.StatusServiceUnavailable, "MCP is not available")
		}
		permissions, ok := PermissionsForPreset(preset)
		if !ok {
			_ = s.Rollback()
			return nil, echo.NewHTTPError(http.StatusBadRequest, "permissions must be one of typed, read_only, full")
		}
		oauthCode.Permissions = permissions
	} else if !ValidateRedirectURI(req.RedirectURI) {
		_ = s.Rollback()
		return nil, &models.ErrOAuthInvalidRedirectURI{}
	}

	fullUser, err := user.GetUserByID(s, userID)
	if err != nil {
		_ = s.Rollback()
		return nil, err
	}
	oauthCode.UserID = fullUser.ID

	code, err := models.CreateOAuthCode(s, oauthCode)
	if err != nil {
		_ = s.Rollback()
		return nil, err
	}

	if err := s.Commit(); err != nil {
		return nil, err
	}

	return &AuthorizeResponse{
		Code:        code,
		RedirectURI: req.RedirectURI,
		State:       req.State,
	}, nil
}
