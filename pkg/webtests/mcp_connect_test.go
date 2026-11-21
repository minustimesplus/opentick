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

package webtests

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/modules/mcp"
	"code.vikunja.io/api/pkg/routes"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for what broke connecting real clients: Claude's connector, Claude Code and
// assistants that are less strict about the streamable HTTP transport.

// Behind a reverse proxy every client shares one IP, so discovery used to spend the login budget and get 429s.
func TestMCPConnect_DiscoveryIsNotRateLimited(t *testing.T) {
	_, err := setupTestEnv()
	require.NoError(t, err)
	previous := config.RateLimitNoAuthRoutesLimit.GetInt64()
	config.RateLimitNoAuthRoutesLimit.Set(2)
	t.Cleanup(func() { config.RateLimitNoAuthRoutesLimit.Set(previous) })
	e := routes.NewEcho()
	routes.RegisterRoutes(e)

	for i := 0; i < 10; i++ {
		for _, path := range []string{
			"/.well-known/oauth-protected-resource/api/v2/mcp",
			"/.well-known/oauth-protected-resource",
			"/.well-known/oauth-authorization-server",
		} {
			rec := oauthJSON(t, e, http.MethodGet, path, "", nil)
			require.Equal(t, http.StatusOK, rec.Code, "request %d to %s: %s", i, path, rec.Body.String())
			assert.Empty(t, rec.Header().Get("X-RateLimit-Limit"), path)
		}
	}
}

// Clients probe OpenID discovery too; the SPA's index.html with a 200 made them fail instead of moving on.
func TestMCPConnect_UnknownWellKnownIs404(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	for _, path := range []string{
		"/.well-known/openid-configuration",
		"/.well-known/oauth-authorization-server/api/v2/mcp",
	} {
		rec := oauthJSON(t, e, http.MethodGet, path, "", nil)
		assert.Equal(t, http.StatusNotFound, rec.Code, path)
		assert.NotContains(t, rec.Header().Get("Content-Type"), "text/html", path)
	}
}

// Claude Code registers a loopback redirect once and signs in again later on another port (RFC 8252 §7.3).
func TestMCPConnect_LoopbackRedirectOnAnotherPort(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	rec := oauthJSON(t, e, http.MethodPost, "/api/v2/oauth/register", "", map[string]any{
		"client_name":   "Claude Code",
		"redirect_uris": []string{"http://localhost:33418/callback"},
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	clientID := decode[map[string]any](t, rec)["client_id"].(string)

	verifier := "loopback-test-verifier-0123456789-abcdefghijklmnop"
	sum := sha256.Sum256([]byte(verifier))
	authorize := func(redirect string) *httptest.ResponseRecorder {
		return oauthJSON(t, e, http.MethodPost, "/api/v2/oauth/authorize", oauthUserJWT(t), map[string]any{
			"response_type":         "code",
			"client_id":             clientID,
			"redirect_uri":          redirect,
			"code_challenge":        base64.RawURLEncoding.EncodeToString(sum[:]),
			"code_challenge_method": "S256",
		})
	}

	rec = authorize("http://localhost:51234/other")
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	redirect := "http://localhost:51234/callback"
	rec = authorize(redirect)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = oauthForm(e, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {decode[map[string]any](t, rec)["code"].(string)},
		"client_id":     {clientID},
		"redirect_uri":  {redirect},
		"code_verifier": {verifier},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.NotEmpty(t, decode[map[string]any](t, rec)["access_token"])
}

func TestMCPConnect_TrailingSlash(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	post := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v2/mcp/", strings.NewReader(initializeBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if token != "" {
			req.Header.Set(echo.HeaderAuthorization, "Bearer "+token)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	rec := post("")
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, rec.Header().Get("WWW-Authenticate"), "resource_metadata=")

	rec = post(mcpOnlyToken)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// The spec asks clients to accept both JSON and SSE, but several send only one of them.
func TestMCPConnect_LenientAcceptAndJSONReplies(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	for _, accept := range []string{
		"application/json",
		"text/event-stream",
		"*/*",
		"",
		"application/json, text/event-stream",
	} {
		req := mcpRequest(http.MethodPost, initializeBody)
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+mcpOnlyToken)
		if accept == "" {
			req.Header.Del("Accept")
		} else {
			req.Header.Set("Accept", accept)
		}
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code, "Accept %q: %s", accept, rec.Body.String())
		assert.Equal(t, "application/json", strings.Split(rec.Header().Get("Content-Type"), ";")[0], accept)
		var msg map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &msg), "Accept %q: %s", accept, rec.Body.String())
		assert.NotNil(t, msg["result"])
	}
}

func TestMCPConnect_ToolResultsOmitSchemaLink(t *testing.T) {
	c := newAllScopesMCPClient(t)
	result := c.callTool("search_contacts", map[string]any{"location": []string{"London"}})
	require.NotEqual(t, true, result["isError"], "%v", result)
	assert.NotContains(t, toolResultText(t, result), "$schema")
	structured, _ := result["structuredContent"].(map[string]any)
	assert.NotContains(t, structured, "$schema")
}

// Browser-based MCP clients must be able to read the OAuth challenge and the session id.
func TestMCPConnect_CORSExposesMCPHeaders(t *testing.T) {
	_, err := setupTestEnv()
	require.NoError(t, err)
	prevEnable, prevOrigins := config.CorsEnable.GetBool(), config.CorsOrigins.GetStringSlice()
	config.CorsEnable.Set(true)
	config.CorsOrigins.Set([]string{"http://localhost:*"})
	t.Cleanup(func() {
		config.CorsEnable.Set(prevEnable)
		config.CorsOrigins.Set(prevOrigins)
	})
	e := routes.NewEcho()
	routes.RegisterRoutes(e)

	req := mcpRequest(http.MethodPost, initializeBody)
	req.Header.Set("Origin", "http://localhost:6274")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
	exposed := rec.Header().Get("Access-Control-Expose-Headers")
	assert.Contains(t, exposed, "WWW-Authenticate")
	assert.Contains(t, exposed, "Mcp-Session-Id")
}

// The Personal CRM's agent tools, by name, with the same bulk-write contract.
func TestMCPConnect_PersonalCRMToolNames(t *testing.T) {
	c := newAllScopesMCPClient(t)
	tools := map[string]map[string]any{}
	for _, tool := range c.listTools() {
		tools[tool["name"].(string)] = tool
	}
	for _, name := range []string{
		"search_contacts",
		"get_contact",
		"distinct_values",
		"list_groups",
		"list_tags",
		"create_group",
		"update_group",
		"add_to_group",
		"remove_from_group",
		"tag_contacts",
		"untag_contacts",
		"update_contact",
		"create_contact",
		"add_note",
		"add_experience",
		"add_education",
	} {
		assert.Contains(t, tools, name)
	}
	for name := range tools {
		assert.False(t, strings.HasPrefix(name, "contacts_") || strings.HasPrefix(name, "contact_"), "%s kept its generated name", name)
		assert.NotContains(t, name, "delete_contact")
	}
	for _, a := range findActions(t, c, map[string]any{}) {
		name := a["name"].(string)
		assert.False(t, strings.Contains(name, "contact") && strings.Contains(name, "delete"), "%s deletes contacts", name)
	}

	for _, name := range []string{
		"add_to_group",
		"remove_from_group",
		"tag_contacts",
		"untag_contacts",
	} {
		schema := tools[name]["inputSchema"].(map[string]any)
		props := schema["properties"].(map[string]any)
		assert.Contains(t, props, "dry_run", name)
		assert.Subset(t, schema["required"], []any{"contact_ids"}, name)
	}
	meta, _ := tools["add_note"]["_meta"].(map[string]any)
	assert.Equal(t, []any{"id", "body"}, meta[mcp.PositionalMetaKey])
	assert.Contains(t, tools["search_contacts"]["description"], "why it matched")

	res := c.callTool("tag_contacts", map[string]any{
		"tag":         "CRM",
		"contact_ids": []int{1, 999999},
		"dry_run":     true,
	})
	require.NotEqual(t, true, res["isError"], "%v", res)
	var out struct {
		DryRun     bool    `json:"dry_run"`
		Changed    int     `json:"changed"`
		Unchanged  int     `json:"unchanged"`
		UnknownIDs []int64 `json:"unknown_ids"`
	}
	toolResultJSON(t, res, &out)
	assert.True(t, out.DryRun)
	assert.Equal(t, 1, out.Changed)
	assert.Equal(t, []int64{999999}, out.UnknownIDs)
}
