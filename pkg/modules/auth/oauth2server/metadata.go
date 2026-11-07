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
	"fmt"
	"net/http"
	"strings"

	"code.vikunja.io/api/pkg/config"

	"github.com/labstack/echo/v5"
)

// MCPPath is the MCP endpoint's path. It mirrors mcp.RoutePrefix, which can't
// be imported here: the mcp module imports apiv2, which imports this package.
const MCPPath = "/api/v2/mcp"

// ProtectedResourceMetadataPath is where clients discover how to authorize against the MCP endpoint (RFC 9728).
const ProtectedResourceMetadataPath = "/.well-known/oauth-protected-resource"

// AuthorizationServerMetadataPath is the RFC 8414 metadata document.
const AuthorizationServerMetadataPath = "/.well-known/oauth-authorization-server"

// ProtectedResourceMetadata is the RFC 9728 document for the MCP endpoint.
type ProtectedResourceMetadata struct {
	Resource               string   `json:"resource"`
	AuthorizationServers   []string `json:"authorization_servers"`
	BearerMethodsSupported []string `json:"bearer_methods_supported"`
	ResourceName           string   `json:"resource_name,omitempty"`
}

// AuthorizationServerMetadata is the RFC 8414 document.
type AuthorizationServerMetadata struct {
	Issuer                            string   `json:"issuer"`
	AuthorizationEndpoint             string   `json:"authorization_endpoint"`
	TokenEndpoint                     string   `json:"token_endpoint"`
	RegistrationEndpoint              string   `json:"registration_endpoint"`
	ResponseTypesSupported            []string `json:"response_types_supported"`
	GrantTypesSupported               []string `json:"grant_types_supported"`
	CodeChallengeMethodsSupported     []string `json:"code_challenge_methods_supported"`
	TokenEndpointAuthMethodsSupported []string `json:"token_endpoint_auth_methods_supported"`
	ScopesSupported                   []string `json:"scopes_supported"`
}

// publicBase is the configured public URL without a trailing slash, or "" when unset.
// Discovery needs absolute URLs, so without it OAuth for MCP is unavailable.
func publicBase() string {
	return strings.TrimRight(config.ServicePublicURL.GetString(), "/")
}

// MCPResourceURL is the canonical resource identifier of the MCP endpoint.
func MCPResourceURL() string {
	return publicBase() + MCPPath
}

// WWWAuthenticateChallenge is the header value a 401 from the MCP endpoint
// carries so clients can find the resource metadata (RFC 9728 §5.1). Empty
// when no public URL is configured.
func WWWAuthenticateChallenge() string {
	base := publicBase()
	if base == "" {
		return ""
	}
	return fmt.Sprintf(`Bearer resource_metadata="%s%s%s"`, base, ProtectedResourceMetadataPath, MCPPath)
}

// HandleProtectedResourceMetadata serves the RFC 9728 document, both at the
// root well-known path and at the path-suffixed one for the MCP endpoint.
func HandleProtectedResourceMetadata(c *echo.Context) error {
	base := publicBase()
	if base == "" {
		return echo.ErrNotFound
	}
	return c.JSON(http.StatusOK, ProtectedResourceMetadata{
		Resource:               base + MCPPath,
		AuthorizationServers:   []string{base},
		BearerMethodsSupported: []string{"header"},
		ResourceName:           "OpenTick MCP",
	})
}

// HandleAuthorizationServerMetadata serves the RFC 8414 document. The
// authorization endpoint is the web frontend's consent page, which posts
// the user's decision to /api/v2/oauth/authorize.
func HandleAuthorizationServerMetadata(c *echo.Context) error {
	base := publicBase()
	if base == "" {
		return echo.ErrNotFound
	}
	return c.JSON(http.StatusOK, AuthorizationServerMetadata{
		Issuer:                            base,
		AuthorizationEndpoint:             base + "/oauth/authorize",
		TokenEndpoint:                     base + "/api/v2/oauth/token",
		RegistrationEndpoint:              base + "/api/v2/oauth/register",
		ResponseTypesSupported:            []string{"code"},
		GrantTypesSupported:               []string{"authorization_code", "refresh_token"},
		CodeChallengeMethodsSupported:     []string{"S256"},
		TokenEndpointAuthMethodsSupported: []string{"none"},
		ScopesSupported:                   []string{},
	})
}
