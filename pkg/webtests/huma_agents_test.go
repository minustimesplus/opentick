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
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures: agent_configs.yml (user1 has Claude and Instinct set up),
// project_agent_settings.yml (project 1 has AI context), agent_dispatches.yml (task 1).
func TestHumaAgents(t *testing.T) {
	owner := webHandlerTestV2{user: &testuser1, t: t}
	stranger := webHandlerTestV2{user: &testuser2, t: t}

	orig := models.FireClaudeRoutine
	models.FireClaudeRoutine = func(context.Context, string, string, string) (string, error) {
		return "https://claude.ai/code/session_webtest", nil
	}
	t.Cleanup(func() { models.FireClaudeRoutine = orig })

	t.Run("Config hides the token", func(t *testing.T) {
		rec, err := owner.serve(http.MethodGet, "/api/v2/agents/config", "")
		require.NoError(t, err)
		body := rec.Body.String()
		assert.Contains(t, body, `"claude_token_set":true`)
		assert.Contains(t, body, `"instinct_email":"instinct@example.com"`)
		assert.Contains(t, body, `"default_system_prompt":"You are an assistant`)
		assert.NotContains(t, body, "sk-ant-oat01-test")
	})
	t.Run("Config update", func(t *testing.T) {
		rec, err := stranger.serve(http.MethodPut, "/api/v2/agents/config", `{"instinct_email":"me@example.com","system_prompt":"Be brief."}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"claude_token_set":false`)
		db.AssertExists(t, "agent_configs", map[string]any{"user_id": 2, "instinct_email": "me@example.com"}, false)
	})
	t.Run("Config rejects foreign fire URLs", func(t *testing.T) {
		_, err := owner.serve(http.MethodPut, "/api/v2/agents/config", `{"claude_fire_url":"https://example.com/fire"}`)
		require.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeInvalidClaudeFireURL)
	})

	t.Run("List settings", func(t *testing.T) {
		rec, err := owner.serve(http.MethodGet, "/api/v2/projects/1/agent-settings", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"ai_context":"Always write in French."`)

		_, err = stranger.serve(http.MethodGet, "/api/v2/projects/1/agent-settings", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("List settings update", func(t *testing.T) {
		rec, err := owner.serve(http.MethodPut, "/api/v2/projects/1/agent-settings", `{"ai_context":"Écris en français.","agent":"instinct"}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"agent":"instinct"`)
		db.AssertExists(t, "project_agent_settings", map[string]any{"project_id": 1, "agent": "instinct", "agent_user_id": 1}, false)

		_, err = stranger.serve(http.MethodPut, "/api/v2/projects/1/agent-settings", `{"ai_context":"x"}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("All list settings", func(t *testing.T) {
		rec, err := owner.serve(http.MethodGet, "/api/v2/agents/projects", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"project_id":1`)
	})

	t.Run("Prompt preview", func(t *testing.T) {
		rec, err := owner.serve(http.MethodGet, "/api/v2/tasks/1/agent-prompt?extra=Hurry", "")
		require.NoError(t, err)
		body := rec.Body.String()
		assert.Contains(t, body, `Task: task #1`)
		assert.Contains(t, body, `Additional request:\nHurry`)

		_, err = stranger.serve(http.MethodGet, "/api/v2/tasks/1/agent-prompt", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("Dispatch", func(t *testing.T) {
		rec, err := owner.serve(http.MethodPost, "/api/v2/tasks/1/agent-dispatches", `{"agent":"claude","extra_prompt":"Go"}`)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), `"session_url":"https://claude.ai/code/session_webtest"`)
		assert.Contains(t, rec.Body.String(), `"status":"sent"`)

		_, err = stranger.serve(http.MethodPost, "/api/v2/tasks/1/agent-dispatches", `{"agent":"claude"}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("Dispatch to Instinct without a mailer", func(t *testing.T) {
		_, err := owner.serve(http.MethodPost, "/api/v2/tasks/1/agent-dispatches", `{"agent":"instinct"}`)
		require.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeAgentMailerDisabled)
	})
	t.Run("Dispatch lists", func(t *testing.T) {
		rec, err := owner.serve(http.MethodGet, "/api/v2/tasks/1/agent-dispatches", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"task_id":1`)

		rec, err = owner.serve(http.MethodGet, "/api/v2/agents/dispatches", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"task_id":1`)

		_, err = stranger.serve(http.MethodGet, "/api/v2/tasks/1/agent-dispatches", "")
		require.Error(t, err)
	})
}

// mailer_enabled is computed from the server config, so turning the mailer on
// must change the ETag; otherwise a client that cached the config earlier keeps
// getting 304s and still says Instinct can't be reached.
func TestHumaAgents_ConfigETagFollowsMailer(t *testing.T) {
	e, err := setupTestEnv()
	require.NoError(t, err)
	token := humaTokenFor(t, &testuser1)

	old := config.MailerEnabled.GetBool()
	t.Cleanup(func() { config.MailerEnabled.Set(old) })

	config.MailerEnabled.Set(false)
	rec := humaRequest(t, e, http.MethodGet, "/api/v2/agents/config", "", token, "")
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"mailer_enabled":false`)
	etag := rec.Header().Get("ETag")
	require.NotEmpty(t, etag)

	conditionalGet := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v2/agents/config", strings.NewReader(""))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("If-None-Match", etag)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		return rec
	}

	require.Equal(t, http.StatusNotModified, conditionalGet().Code)

	config.MailerEnabled.Set(true)
	rec = conditionalGet()
	require.Equal(t, http.StatusOK, rec.Code, "a mailer change must not be answered with 304")
	assert.Contains(t, rec.Body.String(), `"mailer_enabled":true`)
	assert.NotEqual(t, etag, rec.Header().Get("ETag"))
}
