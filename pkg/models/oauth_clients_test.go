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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthClientAllowsRedirectURI(t *testing.T) {
	c := &OAuthClient{RedirectURIs: []string{
		"https://claude.ai/api/mcp/auth_callback",
		"http://localhost:33418/callback",
		"http://127.0.0.1/cb",
		"http://[::1]:9000/cb",
	}}
	for _, tc := range []struct {
		uri  string
		want bool
	}{
		{"https://claude.ai/api/mcp/auth_callback", true},
		{"https://claude.ai:8443/api/mcp/auth_callback", false},
		{"https://claude.ai/api/mcp/auth_callback/x", false},
		// Loopback clients pick a fresh port for every sign-in (RFC 8252 §7.3).
		{"http://localhost:51234/callback", true},
		{"http://LOCALHOST:1/callback", true},
		{"http://127.0.0.1:8080/cb", true},
		{"http://[::1]/cb", true},
		{"http://localhost:51234/other", false},
		{"http://127.0.0.1:8080/callback", false},
		{"http://127.0.0.1:8080/cb?x=1", false},
		{"https://localhost:51234/callback", false},
		{"http://evil.example:33418/callback", false},
		{"http://user@localhost:33418/callback", false},
	} {
		assert.Equal(t, tc.want, c.AllowsRedirectURI(tc.uri), tc.uri)
	}
}
