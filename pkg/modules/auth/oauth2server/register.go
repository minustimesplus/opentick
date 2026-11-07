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
	"net"
	"net/url"
	"slices"
	"strings"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
)

const (
	maxRedirectURIs   = 5
	maxRedirectURILen = 2000
	maxClientNameLen  = 100
	defaultClientName = "MCP client"
	authMethodNone    = "none"
	grantTypeAuthCode = "authorization_code"
	grantTypeRefresh  = "refresh_token"
	responseTypeCode  = "code"
)

// RegisterRequest is an RFC 7591 client registration request. Only public
// clients using the authorization code flow with PKCE are supported.
type RegisterRequest struct {
	ClientName              string   `json:"client_name,omitempty" maxLength:"255" doc:"Human-readable name shown on the consent screen."`
	RedirectURIs            []string `json:"redirect_uris" required:"true" minItems:"1" doc:"Redirect URIs. Must be https, or http on a loopback address."`
	GrantTypes              []string `json:"grant_types,omitempty" doc:"Only authorization_code and refresh_token are supported."`
	ResponseTypes           []string `json:"response_types,omitempty" doc:"Only code is supported."`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method,omitempty" doc:"Only none (public client) is supported."`
	// Clients send further metadata (logo_uri, client_uri, scope, ...); it is ignored.
	_ struct{} `additionalProperties:"true"`
}

// RegisterResponse is the RFC 7591 client information response.
type RegisterResponse struct {
	ClientID                string   `json:"client_id"`
	ClientIDIssuedAt        int64    `json:"client_id_issued_at"`
	ClientName              string   `json:"client_name"`
	RedirectURIs            []string `json:"redirect_uris"`
	GrantTypes              []string `json:"grant_types"`
	ResponseTypes           []string `json:"response_types"`
	TokenEndpointAuthMethod string   `json:"token_endpoint_auth_method"`
}

// validRegisteredRedirectURI accepts https URIs and loopback http URIs, the
// redirect URIs OAuth 2.1 allows for clients without a custom scheme.
func validRegisteredRedirectURI(raw string) bool {
	if len(raw) > maxRedirectURILen {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.Fragment != "" || u.User != nil {
		return false
	}
	switch u.Scheme {
	case "https":
		return true
	case "http":
		host := u.Hostname()
		if strings.EqualFold(host, "localhost") {
			return true
		}
		ip := net.ParseIP(host)
		return ip != nil && ip.IsLoopback()
	}
	return false
}

// RegisterClient validates an RFC 7591 request and stores the client.
func RegisterClient(req *RegisterRequest) (*RegisterResponse, error) {
	if len(req.RedirectURIs) == 0 || len(req.RedirectURIs) > maxRedirectURIs {
		return nil, &models.ErrOAuthInvalidClientMetadata{Reason: "between 1 and 5 redirect_uris are required"}
	}
	for _, uri := range req.RedirectURIs {
		if !validRegisteredRedirectURI(uri) {
			return nil, &models.ErrOAuthInvalidRedirectURI{}
		}
	}
	if req.TokenEndpointAuthMethod != "" && req.TokenEndpointAuthMethod != authMethodNone {
		return nil, &models.ErrOAuthInvalidClientMetadata{Reason: "only public clients (token_endpoint_auth_method none) are supported"}
	}
	for _, gt := range req.GrantTypes {
		if gt != grantTypeAuthCode && gt != grantTypeRefresh {
			return nil, &models.ErrOAuthInvalidClientMetadata{Reason: "unsupported grant type " + gt}
		}
	}
	if len(req.ResponseTypes) > 0 && !slices.Equal(req.ResponseTypes, []string{responseTypeCode}) {
		return nil, &models.ErrOAuthInvalidClientMetadata{Reason: "only the code response type is supported"}
	}

	name := strings.TrimSpace(req.ClientName)
	if name == "" {
		name = defaultClientName
	}
	if r := []rune(name); len(r) > maxClientNameLen {
		name = string(r[:maxClientNameLen])
	}

	s := db.NewSession()
	defer s.Close()

	client, err := models.CreateOAuthClient(s, name, slices.Clone(req.RedirectURIs))
	if err != nil {
		_ = s.Rollback()
		return nil, err
	}
	if err := s.Commit(); err != nil {
		return nil, err
	}

	return &RegisterResponse{
		ClientID:                client.ClientID,
		ClientIDIssuedAt:        client.Created.Unix(),
		ClientName:              client.Name,
		RedirectURIs:            client.RedirectURIs,
		GrantTypes:              []string{grantTypeAuthCode, grantTypeRefresh},
		ResponseTypes:           []string{responseTypeCode},
		TokenEndpointAuthMethod: authMethodNone,
	}, nil
}

// ClientInfo is the public metadata of a registered client, for the consent screen.
type ClientInfo struct {
	ClientID     string   `json:"client_id" doc:"The client identifier."`
	ClientName   string   `json:"client_name" doc:"Human-readable client name."`
	RedirectURIs []string `json:"redirect_uris" doc:"Registered redirect URIs."`
}

// GetClientInfo returns a registered client's public metadata or ErrOAuthClientNotFound.
func GetClientInfo(clientID string) (*ClientInfo, error) {
	s := db.NewSession()
	defer s.Close()

	client, err := models.GetOAuthClient(s, clientID)
	if err != nil {
		return nil, err
	}
	return &ClientInfo{ClientID: client.ClientID, ClientName: client.Name, RedirectURIs: client.RedirectURIs}, nil
}
