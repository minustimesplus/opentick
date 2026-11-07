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
	"errors"
	"net/http"

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/auth/oauth2server"
	"code.vikunja.io/api/pkg/modules/humabridge"
	"code.vikunja.io/api/pkg/user"

	"github.com/danielgtaylor/huma/v2"
	"github.com/labstack/echo/v5"
)

// oauthTokenBody wraps the OAuth 2.0 token response.
type oauthTokenBody struct {
	// Cache-Control: no-store is required by RFC 6749 §5.1 so tokens are not
	// cached. v2 already sets it globally, but declaring it keeps the contract
	// explicit in the spec.
	CacheControl string `header:"Cache-Control"`
	Body         *oauth2server.TokenResponse
}

// oauthAuthorizeBody wraps the OAuth 2.0 authorization response.
type oauthAuthorizeBody struct {
	Body *oauth2server.AuthorizeResponse
}

func init() { AddRouteRegistrar(RegisterOAuthRoutes) }

// RegisterOAuthRoutes wires the OAuth 2.0 token and authorize endpoints. The
// token endpoint is public (it authenticates the request itself); authorize
// inherits the global JWT auth.
func RegisterOAuthRoutes(api huma.API) {
	tags := []string{"auth"}

	Register(api, huma.Operation{
		OperationID:   "oauth-token",
		Summary:       "OAuth 2.0 token endpoint",
		Description:   "Exchanges an authorization code (grant_type=authorization_code) or a refresh token (grant_type=refresh_token) for an access token. Accepts application/x-www-form-urlencoded per RFC 6749 as well as JSON.",
		Method:        http.MethodPost,
		Path:          "/oauth/token",
		DefaultStatus: http.StatusOK,
		Tags:          tags,
		Security:      publicSecurity,
	}, oauthToken)

	Register(api, huma.Operation{
		OperationID:   "oauth-authorize",
		Summary:       "OAuth 2.0 authorize endpoint",
		Description:   "Creates a single-use authorization code for the authenticated user. PKCE (code_challenge with method S256) and a loopback or vikunja- scheme redirect_uri are required.",
		Method:        http.MethodPost,
		Path:          "/oauth/authorize",
		DefaultStatus: http.StatusOK,
		Tags:          tags,
	}, oauthAuthorize)

	Register(api, huma.Operation{
		OperationID: "oauth-register",
		Summary:     "OAuth 2.0 dynamic client registration",
		Description: "Registers a public OAuth client (RFC 7591), e.g. an MCP client such as Claude. Only the authorization code flow with PKCE and token_endpoint_auth_method none are supported. Redirect URIs must be https or loopback http. Tokens issued to registered clients are API tokens limited to the permissions the user approves.",
		Method:      http.MethodPost,
		Path:        "/oauth/register",
		Tags:        tags,
		Security:    publicSecurity,
	}, oauthRegister)

	Register(api, huma.Operation{
		OperationID: "oauth-client-read",
		Summary:     "Get a registered OAuth client",
		Description: "Returns the public metadata of a dynamically registered client so the consent screen can show who is asking for access. Requires a user session; API tokens are rejected.",
		Method:      http.MethodGet,
		Path:        "/oauth/clients/{client_id}",
		Tags:        tags,
	}, oauthClientInfo)
}

// oauthRegisterBody wraps the RFC 7591 client information response.
type oauthRegisterBody struct {
	CacheControl string `header:"Cache-Control"`
	Body         *oauth2server.RegisterResponse
}

// oauthErrorModel adds the RFC 6749 §5.2 error fields OAuth clients parse to
// the regular problem+json body, so first-party clients keep reading `code`.
type oauthErrorModel struct {
	*vikunjaErrorModel
	OAuthError       string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

// oauthError translates err like translateDomainError and adds the OAuth
// error code that classify picks for the domain error code and status.
func oauthError(err error, classify func(code, status int) string) error {
	var echoErr *echo.HTTPError
	if errors.As(err, &echoErr) {
		err = huma.NewError(echoErr.Code, echoErr.Message)
	}
	err = translateDomainError(err)
	vm := &vikunjaErrorModel{}
	ok := errors.As(err, &vm)
	if !ok {
		return err
	}
	return &oauthErrorModel{vikunjaErrorModel: vm, OAuthError: classify(vm.Code, vm.Status), ErrorDescription: vm.Detail}
}

func classifyTokenError(code, status int) string {
	switch {
	case status >= 500:
		return "server_error"
	case code == models.ErrCodeOAuthInvalidGrantType:
		return "unsupported_grant_type"
	case status == http.StatusUnprocessableEntity:
		return "invalid_request"
	default:
		// Unknown, used, or expired codes and refresh tokens, PKCE and redirect mismatches.
		return "invalid_grant"
	}
}

func classifyRegisterError(code, status int) string {
	switch {
	case status >= 500:
		return "server_error"
	case code == models.ErrCodeOAuthInvalidRedirectURI:
		return "invalid_redirect_uri"
	default:
		return "invalid_client_metadata"
	}
}

func oauthToken(ctx context.Context, in *struct {
	Body oauth2server.TokenRequest `contentType:"application/x-www-form-urlencoded"`
}) (*oauthTokenBody, error) {
	deviceInfo, ipAddress := requestClientInfo(ctx)
	resp, err := oauth2server.ExchangeToken(ctx, &in.Body, deviceInfo, ipAddress)
	if err != nil {
		return nil, oauthError(err, classifyTokenError)
	}
	return &oauthTokenBody{CacheControl: "no-store", Body: resp}, nil
}

func oauthAuthorize(ctx context.Context, in *struct{ Body oauth2server.AuthorizeRequest }) (*oauthAuthorizeBody, error) {
	if ec := humabridge.EchoContextFrom(ctx); ec != nil && (*ec).Get("api_token") != nil {
		return nil, huma.Error403Forbidden("API tokens cannot be used to authorize OAuth clients")
	}

	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	u, err := user.GetFromAuth(a)
	if err != nil {
		return nil, translateDomainError(err)
	}

	resp, err := oauth2server.Authorize(&in.Body, u.ID)
	if err != nil {
		var echoErr *echo.HTTPError
		if errors.As(err, &echoErr) {
			return nil, huma.NewError(echoErr.Code, echoErr.Message)
		}
		return nil, translateDomainError(err)
	}
	return &oauthAuthorizeBody{Body: resp}, nil
}

func oauthRegister(_ context.Context, in *struct{ Body oauth2server.RegisterRequest }) (*oauthRegisterBody, error) {
	resp, err := oauth2server.RegisterClient(&in.Body)
	if err != nil {
		return nil, oauthError(err, classifyRegisterError)
	}
	return &oauthRegisterBody{CacheControl: "no-store", Body: resp}, nil
}

func oauthClientInfo(ctx context.Context, in *struct {
	ClientID string `path:"client_id" doc:"A client ID issued by dynamic client registration."`
}) (*singleBody[oauth2server.ClientInfo], error) {
	if ec := humabridge.EchoContextFrom(ctx); ec != nil && (*ec).Get("api_token") != nil {
		return nil, huma.Error403Forbidden("API tokens cannot read OAuth clients")
	}
	if _, err := authFromCtx(ctx); err != nil {
		return nil, err
	}
	info, err := oauth2server.GetClientInfo(in.ClientID)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[oauth2server.ClientInfo]{Body: info}, nil
}

// requestClientInfo pulls the user agent and client IP off the underlying Echo
// request so the authorization_code grant (and login) can record them on the
// session they create, mirroring v1. Both fall back to "" when the context is
// unavailable.
func requestClientInfo(ctx context.Context) (deviceInfo, ipAddress string) {
	ec := humabridge.EchoContextFrom(ctx)
	if ec == nil {
		return "", ""
	}
	return (*ec).Request().UserAgent(), (*ec).RealIP()
}
