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
	"net/http"
	"testing"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func enableInboundMail(t *testing.T) {
	config.InboundMailEnabled.Set(true)
	config.InboundMailHost.Set("imap.example.com")
	config.InboundMailAddress.Set("inbox@example.com")
	t.Cleanup(func() {
		config.InboundMailEnabled.Set(false)
		config.InboundMailHost.Set("")
		config.InboundMailAddress.Set("")
	})
}

// Fixtures: agent_configs.yml gives user1 the inbound secret abcdef123456; user2 has no config.
func TestHumaAgents_InboundAddress(t *testing.T) {
	owner := webHandlerTestV2{user: &testuser1, t: t}
	stranger := webHandlerTestV2{user: &testuser2, t: t}

	t.Run("hidden while the server reads no mailbox", func(t *testing.T) {
		rec, err := owner.serve(http.MethodGet, "/api/v2/agents/config", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"inbound_enabled":false`)
		assert.Contains(t, rec.Body.String(), `"inbound_address":""`)
	})
	t.Run("shown once it does", func(t *testing.T) {
		enableInboundMail(t)
		rec, err := owner.serve(http.MethodGet, "/api/v2/agents/config", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"inbound_address":"inbox+abcdef123456@example.com"`)
		assert.NotContains(t, rec.Body.String(), `inbound_secret`)
	})
	t.Run("rotate replaces the address", func(t *testing.T) {
		enableInboundMail(t)
		rec, err := owner.serve(http.MethodPost, "/api/v2/agents/inbound-address", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"inbound_address":"inbox+`)
		assert.NotContains(t, rec.Body.String(), "abcdef123456")
		db.AssertMissing(t, "agent_configs", map[string]any{"user_id": 1, "inbound_secret": "abcdef123456"})
	})
	t.Run("rotate creates the first address", func(t *testing.T) {
		enableInboundMail(t)
		rec, err := stranger.serve(http.MethodPost, "/api/v2/agents/inbound-address", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"inbound_address":"inbox+`)
	})
	t.Run("senders and list are saved", func(t *testing.T) {
		_, err := owner.serve(http.MethodPut, "/api/v2/agents/config", `{"inbound_senders":"Me@Work.com, @example.org","inbound_project_id":1}`)
		require.NoError(t, err)
		db.AssertExists(t, "agent_configs", map[string]any{"user_id": 1, "inbound_senders": "me@work.com, @example.org", "inbound_project_id": 1}, false)
	})
	t.Run("a list without write access is refused", func(t *testing.T) {
		_, err := stranger.serve(http.MethodPut, "/api/v2/agents/config", `{"inbound_project_id":1}`)
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("invalid senders are refused", func(t *testing.T) {
		_, err := owner.serve(http.MethodPut, "/api/v2/agents/config", `{"inbound_senders":"not an address"}`)
		require.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeInvalidAgentEmail)
	})
}

// Fixtures: task 28 in project 1 (user1) repeats every hour; task 1 doesn't repeat.
func TestHumaTaskSkipOccurrence(t *testing.T) {
	owner := webHandlerTestV2{user: &testuser1, t: t}
	stranger := webHandlerTestV2{user: &testuser2, t: t}

	t.Run("skips", func(t *testing.T) {
		rec, err := owner.serve(http.MethodPost, "/api/v2/tasks/28/skip-occurrence", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"deleted":false`)
		assert.Contains(t, rec.Body.String(), `"done":false`)
	})
	t.Run("a task that doesn't repeat", func(t *testing.T) {
		_, err := owner.serve(http.MethodPost, "/api/v2/tasks/1/skip-occurrence", "")
		require.Error(t, err)
		assertHandlerErrorCode(t, err, models.ErrCodeTaskNotRepeating)
	})
	t.Run("forbidden", func(t *testing.T) {
		_, err := stranger.serve(http.MethodPost, "/api/v2/tasks/28/skip-occurrence", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("nonexistent", func(t *testing.T) {
		_, err := owner.serve(http.MethodPost, "/api/v2/tasks/99999/skip-occurrence", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
	})
}
