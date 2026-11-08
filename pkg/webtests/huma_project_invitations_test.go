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

	"code.vikunja.io/api/pkg/db"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures (pkg/db/fixtures/project_invitations.yml): on project 1 (owned by
// user1) #1 invites user2, #2 is a join link, #3 an email invitation and #4
// an expired email invitation.
const (
	webtestLinkToken  = "testlinktoken0000000000000000000000000000000001"
	webtestEmailToken = "testemailtoken000000000000000000000000000000001"
)

func TestHumaProjectInvitations(t *testing.T) {
	owner := webHandlerTestV2{user: &testuser1, t: t}
	stranger := webHandlerTestV2{user: &testuser2, t: t}

	t.Run("List", func(t *testing.T) {
		rec, err := owner.serve(http.MethodGet, "/api/v2/projects/1/invitations", "")
		require.NoError(t, err)
		body := rec.Body.String()
		assert.Contains(t, body, `"total":3`)
		assert.Contains(t, body, `"invite_url":"https://localhost/invite/`+webtestLinkToken+`"`)
		assert.Contains(t, body, `"email":"newcomer@example.com"`)
		assert.NotContains(t, body, "user2@example.com", "the expired invitation and invitee emails stay hidden")
	})
	t.Run("List forbidden", func(t *testing.T) {
		_, err := stranger.serve(http.MethodGet, "/api/v2/projects/1/invitations", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})

	t.Run("Create by username", func(t *testing.T) {
		_, err := stranger.serve(http.MethodPost, "/api/v2/projects/1/invitations", `{"username":"user6"}`)
		require.Error(t, err, "user2 cannot invite to project 1")
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))

		rec, err := owner.serve(http.MethodPost, "/api/v2/projects/1/invitations", `{"username":"user6","permission":1}`)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Contains(t, rec.Body.String(), `"status":"accepted"`)
		db.AssertExists(t, "users_projects", map[string]any{"project_id": 1, "user_id": 6, "permission": 1}, false)
	})
	t.Run("Create pending duplicate", func(t *testing.T) {
		_, err := owner.serve(http.MethodPost, "/api/v2/projects/1/invitations", `{"username":"user2"}`)
		require.Error(t, err)
		assertHandlerErrorCode(t, err, 24002)
	})
	t.Run("Create by email", func(t *testing.T) {
		rec, err := owner.serve(http.MethodPost, "/api/v2/projects/1/invitations", `{"email":"someone@example.com","permission":0}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"kind":"email"`)
		assert.Contains(t, rec.Body.String(), `"invite_url":"https://localhost/invite/`)
		db.AssertExists(t, "project_invitations", map[string]any{"project_id": 1, "email": "someone@example.com"}, false)
	})
	t.Run("Create link", func(t *testing.T) {
		rec, err := owner.serve(http.MethodPost, "/api/v2/projects/1/invitations", `{"link":true}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"kind":"link"`)
	})
	t.Run("Delete forbidden", func(t *testing.T) {
		_, err := stranger.serve(http.MethodDelete, "/api/v2/projects/1/invitations/2", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusForbidden, getHTTPErrorCode(err))
	})
	t.Run("Delete", func(t *testing.T) {
		rec, err := owner.serve(http.MethodDelete, "/api/v2/projects/1/invitations/2", "")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		db.AssertMissing(t, "project_invitations", map[string]any{"id": 2})
	})
}

func TestHumaProjectInvitations_Received(t *testing.T) {
	invitee := webHandlerTestV2{user: &testuser2, t: t}

	t.Run("List", func(t *testing.T) {
		rec, err := invitee.serve(http.MethodGet, "/api/v2/project-invitations", "")
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"project_title":"Test1"`)
		assert.Contains(t, rec.Body.String(), `"id":1`)
	})
	t.Run("Accept someone else's", func(t *testing.T) {
		other := webHandlerTestV2{user: &testuser6, t: t}
		_, err := other.serve(http.MethodPost, "/api/v2/project-invitations/1/accept", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
	})
	t.Run("Accept", func(t *testing.T) {
		rec, err := invitee.serve(http.MethodPost, "/api/v2/project-invitations/1/accept", "")
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Contains(t, rec.Body.String(), `"project_id":1`)
		db.AssertExists(t, "users_projects", map[string]any{"project_id": 1, "user_id": 2, "permission": 1}, false)
	})
	t.Run("Decline", func(t *testing.T) {
		fresh := webHandlerTestV2{user: &testuser2, t: t}
		rec, err := fresh.serve(http.MethodDelete, "/api/v2/project-invitations/1", "")
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, rec.Code)
		db.AssertMissing(t, "project_invitations", map[string]any{"id": 1})
		db.AssertMissing(t, "users_projects", map[string]any{"project_id": 1, "user_id": 2})

		_, err = fresh.serve(http.MethodDelete, "/api/v2/project-invitations/1", "")
		require.Error(t, err)
		assert.Equal(t, http.StatusNotFound, getHTTPErrorCode(err))
	})
}

func TestHumaProjectInvitations_Token(t *testing.T) {
	t.Run("Preview without an account", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/project-invitations/preview", `{"token":"`+webtestEmailToken+`"}`, "", "")
		assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"project_title":"Test1"`)

		rec = humaRequest(t, e, http.MethodPost, "/api/v2/project-invitations/preview", `{"token":"nope"}`, "", "")
		assert.Equal(t, http.StatusNotFound, rec.Code)
	})
	t.Run("Claim needs an account", func(t *testing.T) {
		e, err := setupTestEnv()
		require.NoError(t, err)
		rec := humaRequest(t, e, http.MethodPost, "/api/v2/project-invitations/claim", `{"token":"`+webtestLinkToken+`"}`, "", "")
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
	t.Run("Claim", func(t *testing.T) {
		joiner := webHandlerTestV2{user: &testuser6, t: t}
		rec, err := joiner.serve(http.MethodPost, "/api/v2/project-invitations/claim", `{"token":"`+webtestLinkToken+`"}`)
		require.NoError(t, err)
		assert.Contains(t, rec.Body.String(), `"project_id":1`)
		db.AssertExists(t, "users_projects", map[string]any{"project_id": 1, "user_id": 6, "permission": 0}, false)
	})
}
