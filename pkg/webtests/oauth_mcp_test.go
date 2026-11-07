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
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/modules/auth"

	"github.com/labstack/echo/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const claudeCallback = "https://claude.ai/api/mcp/auth_callback"

func oauthJSON(t *testing.T, e http.Handler, method, path, bearer string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

// oauthForm posts an application/x-www-form-urlencoded token request, as MCP clients do.
func oauthForm(e http.Handler, params url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v2/oauth/token", strings.NewReader(params.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &out), "body: %s", rec.Body.String())
	return out
}

func registerClaude(t *testing.T, e http.Handler) string {
	t.Helper()
	rec := oauthJSON(t, e, http.MethodPost, "/api/v2/oauth/register", "", map[string]any{
		"client_name":                "Claude",
		"redirect_uris":              []string{claudeCallback},
		"grant_types":                []string{"authorization_code", "refresh_token"},
		"response_types":             []string{"code"},
		"token_endpoint_auth_method": "none",
		"scope":                      "claudeai",
	})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	resp := decode[map[string]any](t, rec)
	clientID, _ := resp["client_id"].(string)
	require.True(t, strings.HasPrefix(clientID, models.OAuthClientIDPrefix), clientID)
	return clientID
}

func oauthUserJWT(t *testing.T) string {
	t.Helper()
	jwt, err := auth.NewUserJWTAuthtoken(&testuser1, "test-session-id")
	require.NoError(t, err)
	return jwt
}

// connectClaude runs registration, consent and the code exchange and returns the token response.
func connectClaude(t *testing.T, e http.Handler, preset string) (clientID string, tokens map[string]any) {
	t.Helper()
	clientID = registerClaude(t, e)
	verifier := "claude-test-verifier-0123456789-abcdefghijklmnop"
	sum := sha256.Sum256([]byte(verifier))

	rec := oauthJSON(t, e, http.MethodPost, "/api/v2/oauth/authorize", oauthUserJWT(t), map[string]any{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          claudeCallback,
		"state":                 "st",
		"code_challenge":        base64.RawURLEncoding.EncodeToString(sum[:]),
		"code_challenge_method": "S256",
		"resource":              "https://localhost/api/v2/mcp",
		"permissions":           preset,
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	code := decode[map[string]any](t, rec)["code"].(string)

	rec = oauthForm(e, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"client_id":     {clientID},
		"redirect_uri":  {claudeCallback},
		"code_verifier": {verifier},
		"resource":      {"https://localhost/api/v2/mcp"},
	})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return clientID, decode[map[string]any](t, rec)
}

func TestOAuthMCP_Discovery(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)

	for _, path := range []string{"/.well-known/oauth-protected-resource", "/.well-known/oauth-protected-resource/api/v2/mcp"} {
		rec := oauthJSON(t, e, http.MethodGet, path, "", nil)
		require.Equal(t, http.StatusOK, rec.Code, path)
		doc := decode[map[string]any](t, rec)
		assert.Equal(t, "https://localhost/api/v2/mcp", doc["resource"])
		assert.Equal(t, []any{"https://localhost"}, doc["authorization_servers"])
	}

	rec := oauthJSON(t, e, http.MethodGet, "/.well-known/oauth-authorization-server", "", nil)
	require.Equal(t, http.StatusOK, rec.Code)
	doc := decode[map[string]any](t, rec)
	assert.Equal(t, "https://localhost", doc["issuer"])
	assert.Equal(t, "https://localhost/oauth/authorize", doc["authorization_endpoint"])
	assert.Equal(t, "https://localhost/api/v2/oauth/token", doc["token_endpoint"])
	assert.Equal(t, "https://localhost/api/v2/oauth/register", doc["registration_endpoint"])
	assert.Equal(t, []any{"S256"}, doc["code_challenge_methods_supported"])
}

func TestOAuthMCP_UnauthorizedHasChallenge(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	want := `Bearer resource_metadata="https://localhost/.well-known/oauth-protected-resource/api/v2/mcp"`

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, mcpRequest(http.MethodPost, initializeBody))
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, want, rec.Header().Get("WWW-Authenticate"))

	// A user session is not enough for MCP either.
	req := mcpRequest(http.MethodPost, initializeBody)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+oauthUserJWT(t))
	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Equal(t, want, rec.Header().Get("WWW-Authenticate"))
}

func TestOAuthMCP_Register(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)

	t.Run("accepts Claude", func(t *testing.T) {
		registerClaude(t, e)
	})
	for name, body := range map[string]map[string]any{
		"http redirect":       {"redirect_uris": []string{"http://evil.example/cb"}},
		"no redirect":         {"redirect_uris": []string{}},
		"confidential client": {"redirect_uris": []string{claudeCallback}, "token_endpoint_auth_method": "client_secret_basic"},
		"implicit grant":      {"redirect_uris": []string{claudeCallback}, "grant_types": []string{"implicit"}},
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			rec := oauthJSON(t, e, http.MethodPost, "/api/v2/oauth/register", "", body)
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnprocessableEntity}, rec.Code, rec.Body.String())
		})
	}
	t.Run("loopback http allowed", func(t *testing.T) {
		rec := oauthJSON(t, e, http.MethodPost, "/api/v2/oauth/register", "", map[string]any{"redirect_uris": []string{"http://127.0.0.1:33418/callback"}})
		assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	})
}

func TestOAuthMCP_ClientInfo(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	clientID := registerClaude(t, e)

	rec := oauthJSON(t, e, http.MethodGet, "/api/v2/oauth/clients/"+clientID, oauthUserJWT(t), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	info := decode[map[string]any](t, rec)
	assert.Equal(t, "Claude", info["client_name"])
	assert.Equal(t, []any{claudeCallback}, info["redirect_uris"])

	rec = oauthJSON(t, e, http.MethodGet, "/api/v2/oauth/clients/"+clientID, "", nil)
	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestOAuthMCP_AuthorizeRejectsUnregisteredRedirect(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	clientID := registerClaude(t, e)

	rec := oauthJSON(t, e, http.MethodPost, "/api/v2/oauth/authorize", oauthUserJWT(t), map[string]any{
		"response_type":         "code",
		"client_id":             clientID,
		"redirect_uri":          "https://evil.example/cb",
		"code_challenge":        "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM",
		"code_challenge_method": "S256",
		"permissions":           "full",
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestOAuthMCP_FullFlow(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)

	_, tokens := connectClaude(t, e, "full")
	access := tokens["access_token"].(string)
	assert.True(t, strings.HasPrefix(access, models.APITokenPrefix))
	assert.Equal(t, "Bearer", tokens["token_type"])
	assert.InDelta(t, 3600, tokens["expires_in"], 0)
	assert.NotEmpty(t, tokens["refresh_token"])

	full := newMCPClientOn(t, e, access).toolNames()
	assert.True(t, full["tasks_create"])
	assert.True(t, full["projects_list"])
}

func TestOAuthMCP_ReadOnlyGrantHidesWriteTools(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)

	_, tokens := connectClaude(t, e, "read_only")
	tools := newMCPClientOn(t, e, tokens["access_token"].(string)).toolNames()
	assert.True(t, tools["tasks_list"])
	assert.False(t, tools["tasks_create"])
	assert.False(t, tools["projects_delete"])
}

func TestOAuthMCP_Refresh(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)

	clientID, tokens := connectClaude(t, e, "typed")
	oldAccess := tokens["access_token"].(string)
	oldRefresh := tokens["refresh_token"].(string)

	// Another client cannot use the refresh token.
	rec := oauthForm(e, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {oldRefresh}, "client_id": {registerClaude(t, e)}})
	assert.Equal(t, "invalid_grant", decode[map[string]any](t, rec)["error"])

	rec = oauthForm(e, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {oldRefresh}, "client_id": {clientID}})
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	refreshed := decode[map[string]any](t, rec)
	newAccess := refreshed["access_token"].(string)
	assert.NotEqual(t, oldAccess, newAccess)
	assert.NotEqual(t, oldRefresh, refreshed["refresh_token"])

	// The new access token works; the old one and the old refresh token do not.
	newMCPClientOn(t, e, newAccess)
	req := mcpRequest(http.MethodPost, initializeBody)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+oldAccess)
	old := httptest.NewRecorder()
	e.ServeHTTP(old, req)
	assert.Equal(t, http.StatusUnauthorized, old.Code)

	// A rotated refresh token falls through to session refresh tokens, which answer 401.
	rec = oauthForm(e, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {oldRefresh}, "client_id": {clientID}})
	assert.Contains(t, []int{http.StatusBadRequest, http.StatusUnauthorized}, rec.Code, rec.Body.String())
	assert.Equal(t, "invalid_grant", decode[map[string]any](t, rec)["error"])
}

func TestOAuthMCP_DeletingTokenRevokesGrant(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)

	clientID, tokens := connectClaude(t, e, "typed")

	s := db.NewSession()
	defer s.Close()
	grant := &models.OAuthGrant{}
	has, err := s.Where("client_id = ?", clientID).Get(grant)
	require.NoError(t, err)
	require.True(t, has)
	_, err = s.Where("id = ?", grant.APITokenID).Delete(&models.APIToken{})
	require.NoError(t, err)
	require.NoError(t, s.Commit())

	rec := oauthForm(e, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {tokens["refresh_token"].(string)}, "client_id": {clientID}})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Equal(t, "invalid_grant", decode[map[string]any](t, rec)["error"])

	s2 := db.NewSession()
	defer s2.Close()
	exists, err := s2.Where("id = ?", grant.ID).Exist(&models.OAuthGrant{})
	require.NoError(t, err)
	assert.False(t, exists, "revoked grant is deleted")
}
