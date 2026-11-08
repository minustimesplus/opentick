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

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/notifications"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fixtures: pkg/db/fixtures/project_invitations.yml. user1 owns project 1,
// user2 has no access to it, user1 is admin of project 11 through a share and
// only reads project 3.
const (
	testLinkToken    = "testlinktoken0000000000000000000000000000000001"
	testEmailToken   = "testemailtoken000000000000000000000000000000001"
	testExpiredToken = "testexpiredtoken00000000000000000000000000000001"
)

func fakeNotifications(t *testing.T) {
	notifications.Fake()
	t.Cleanup(notifications.Unfake)
}

func TestProjectInvitation_Permissions(t *testing.T) {
	u1 := &user.User{ID: 1}
	u2 := &user.User{ID: 2}

	t.Run("owner may invite", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, err := (&ProjectInvitation{ProjectID: 1}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.True(t, can)
	})
	t.Run("admin through a share may invite", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, err := (&ProjectInvitation{ProjectID: 11}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.True(t, can)
	})
	t.Run("read only member may not invite", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, err := (&ProjectInvitation{ProjectID: 3}).CanCreate(s, u1)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("stranger may not invite", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, err := (&ProjectInvitation{ProjectID: 1}).CanCreate(s, u2)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("link share may not invite", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, err := (&ProjectInvitation{ProjectID: 1}).CanCreate(s, &LinkSharing{ID: 1, ProjectID: 1, Permission: PermissionAdmin})
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("owner may revoke", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, err := (&ProjectInvitation{ID: 2, ProjectID: 1}).CanDelete(s, u1)
		require.NoError(t, err)
		assert.True(t, can)
	})
	t.Run("stranger may not revoke", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		can, err := (&ProjectInvitation{ID: 2, ProjectID: 1}).CanDelete(s, u2)
		require.NoError(t, err)
		assert.False(t, can)
	})
	t.Run("revoking through another project fails", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, err := (&ProjectInvitation{ID: 2, ProjectID: 11}).CanDelete(s, u1)
		require.Error(t, err)
		assert.True(t, IsErrProjectInvitationDoesNotExist(err))
	})
}

func TestProjectInvitation_Create(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("by username joins right away", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		s := db.NewSession()
		defer s.Close()
		// project 2 is user3's; user1 is not a member.
		inv := &ProjectInvitation{ProjectID: 2, Username: "user1", Permission: PermissionWrite}
		require.NoError(t, inv.Create(s, &user.User{ID: 3}))
		require.NoError(t, s.Commit())
		assert.Equal(t, ProjectInvitationStatusAccepted, inv.Status)
		assert.Equal(t, int64(0), inv.ID)
		db.AssertExists(t, "users_projects", map[string]any{"project_id": 2, "user_id": 1, "permission": PermissionWrite}, false)
		notifications.AssertSent(t, &ProjectSharedNotification{})
	})
	t.Run("by username waits when the user confirms invitations", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		s := db.NewSession()
		defer s.Close()
		_, err := s.ID(1).Cols("confirm_project_invites").Update(&user.User{ConfirmProjectInvites: true})
		require.NoError(t, err)
		inv := &ProjectInvitation{ProjectID: 2, Username: "user1", Permission: PermissionRead}
		require.NoError(t, inv.Create(s, &user.User{ID: 3}))
		require.NoError(t, s.Commit())
		assert.Equal(t, ProjectInvitationStatusPending, inv.Status)
		assert.NotZero(t, inv.ID)
		db.AssertMissing(t, "users_projects", map[string]any{"project_id": 2, "user_id": 1})
		db.AssertExists(t, "project_invitations", map[string]any{"id": inv.ID, "project_id": 2, "invitee_id": 1, "kind": ProjectInvitationKindUser}, false)
		notifications.AssertSent(t, &ProjectInvitationNotification{})
	})
	t.Run("by username fails for a member", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := (&ProjectInvitation{ProjectID: 3, Username: "user1"}).Create(s, &user.User{ID: 3})
		require.Error(t, err)
		assert.True(t, IsErrUserAlreadyHasAccess(err))
	})
	t.Run("by username fails for the owner", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := (&ProjectInvitation{ProjectID: 1, Username: "user1"}).Create(s, u1)
		require.Error(t, err)
		assert.True(t, IsErrUserAlreadyHasAccess(err))
	})
	t.Run("by username fails while an invitation is pending", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := (&ProjectInvitation{ProjectID: 1, Username: "user2"}).Create(s, u1)
		require.Error(t, err)
		assert.True(t, IsErrProjectInvitationExists(err))
	})
	t.Run("by unknown username", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := (&ProjectInvitation{ProjectID: 1, Username: "nobody-here"}).Create(s, u1)
		require.Error(t, err)
		assert.True(t, user.IsErrUserDoesNotExist(err))
	})
	t.Run("by email without an account", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		s := db.NewSession()
		defer s.Close()
		inv := &ProjectInvitation{ProjectID: 1, Email: "  Friend@Example.com ", Permission: PermissionWrite}
		require.NoError(t, inv.Create(s, u1))
		require.NoError(t, s.Commit())
		assert.Equal(t, ProjectInvitationStatusPending, inv.Status)
		assert.Equal(t, ProjectInvitationKindEmail, inv.Kind)
		assert.Equal(t, "friend@example.com", inv.Email)
		assert.Contains(t, inv.InviteURL, frontendURL("invite/"))
		assert.NotNil(t, inv.ExpiresAt)
		assert.NotNil(t, inv.pendingMail, "an address without an account gets a plain mail")
		db.AssertExists(t, "project_invitations", map[string]any{"id": inv.ID, "email": "friend@example.com", "invitee_id": 0}, false)
	})
	t.Run("by email of an account with a verified address", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		old := config.MailerEnabled.GetBool()
		config.MailerEnabled.Set(true)
		t.Cleanup(func() { config.MailerEnabled.Set(old) })
		s := db.NewSession()
		defer s.Close()
		inv := &ProjectInvitation{ProjectID: 2, Email: "user1@example.com"}
		require.NoError(t, inv.Create(s, &user.User{ID: 3}))
		require.NoError(t, s.Commit())
		assert.Nil(t, inv.pendingMail)
		assert.Nil(t, inv.Invitee, "the response must not reveal the account")
		db.AssertExists(t, "project_invitations", map[string]any{"id": inv.ID, "invitee_id": 1, "kind": ProjectInvitationKindEmail}, false)
		notifications.AssertSent(t, &ProjectInvitationNotification{})
	})
	t.Run("by email of an unverified local account is not attached", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		old := config.MailerEnabled.GetBool()
		config.MailerEnabled.Set(false)
		t.Cleanup(func() { config.MailerEnabled.Set(old) })
		s := db.NewSession()
		defer s.Close()
		inv := &ProjectInvitation{ProjectID: 2, Email: "user1@example.com"}
		require.NoError(t, inv.Create(s, &user.User{ID: 3}))
		require.NoError(t, s.Commit())
		db.AssertExists(t, "project_invitations", map[string]any{"id": inv.ID, "invitee_id": 0}, false)
	})
	t.Run("by invalid email", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := (&ProjectInvitation{ProjectID: 1, Email: "not an email"}).Create(s, u1)
		require.Error(t, err)
	})
	t.Run("by email twice", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := (&ProjectInvitation{ProjectID: 1, Email: "newcomer@example.com"}).Create(s, u1)
		require.Error(t, err)
		assert.True(t, IsErrProjectInvitationExists(err))
	})
	t.Run("join link", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		inv := &ProjectInvitation{ProjectID: 1, Link: true}
		require.NoError(t, inv.Create(s, u1))
		require.NoError(t, s.Commit())
		assert.Equal(t, ProjectInvitationKindLink, inv.Kind)
		assert.NotEmpty(t, inv.InviteURL)
		assert.Nil(t, inv.ExpiresAt)
	})
	t.Run("needs exactly one target", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		require.Error(t, (&ProjectInvitation{ProjectID: 1}).Create(s, u1))
		require.Error(t, (&ProjectInvitation{ProjectID: 1, Username: "user2", Link: true}).Create(s, u1))
	})
	t.Run("invalid permission", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		require.Error(t, (&ProjectInvitation{ProjectID: 1, Link: true, Permission: 5}).Create(s, u1))
	})
}

func TestProjectInvitation_ReadAll(t *testing.T) {
	t.Run("admin sees open invitations", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		result, _, total, err := (&ProjectInvitation{ProjectID: 1}).ReadAll(s, &user.User{ID: 1}, "", 1, 50)
		require.NoError(t, err)
		all := result.([]*ProjectInvitation)
		require.Len(t, all, 3, "the expired one is hidden")
		assert.Equal(t, int64(3), total)
		assert.Equal(t, "user2", all[0].Invitee.Username)
		assert.Empty(t, all[0].Invitee.Email)
		assert.Equal(t, frontendURL("invite/")+testLinkToken, all[1].InviteURL)
		assert.Empty(t, all[2].InviteURL, "an email invitation's link is only shown once")
		assert.Equal(t, "user1", all[2].Inviter.Username)
	})
	t.Run("non admin is forbidden", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, _, _, err := (&ProjectInvitation{ProjectID: 1}).ReadAll(s, &user.User{ID: 2}, "", 1, 50)
		require.Error(t, err)
	})
}

func TestProjectInvitation_Received(t *testing.T) {
	t.Run("accept", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		s := db.NewSession()
		defer s.Close()
		u2 := &user.User{ID: 2}
		all, err := ListReceivedProjectInvitations(s, u2)
		require.NoError(t, err)
		require.Len(t, all, 1)
		assert.Equal(t, "Test1", all[0].ProjectTitle)
		p, err := AcceptProjectInvitation(s, u2, 1)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		assert.Equal(t, int64(1), p.ID)
		db.AssertExists(t, "users_projects", map[string]any{"project_id": 1, "user_id": 2, "permission": PermissionWrite}, false)
		db.AssertMissing(t, "project_invitations", map[string]any{"id": 1})
		notifications.AssertSent(t, &ProjectInvitationAcceptedNotification{})
	})
	t.Run("decline", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		require.NoError(t, DeclineProjectInvitation(s, &user.User{ID: 2}, 1))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "project_invitations", map[string]any{"id": 1})
		db.AssertMissing(t, "users_projects", map[string]any{"project_id": 1, "user_id": 2})
	})
	t.Run("someone else's invitation", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, err := AcceptProjectInvitation(s, &user.User{ID: 3}, 1)
		assert.True(t, IsErrProjectInvitationDoesNotExist(err))
		err = DeclineProjectInvitation(s, &user.User{ID: 3}, 1)
		assert.True(t, IsErrProjectInvitationDoesNotExist(err))
		_, err = AcceptProjectInvitation(s, &user.User{ID: 1}, 2)
		assert.True(t, IsErrProjectInvitationDoesNotExist(err), "a join link is not a received invitation")
	})
	t.Run("email invitations attach to a verified address", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		old := config.MailerEnabled.GetBool()
		config.MailerEnabled.Set(true)
		t.Cleanup(func() { config.MailerEnabled.Set(old) })
		s := db.NewSession()
		defer s.Close()
		_, err := s.Insert(&ProjectInvitation{ProjectID: 2, InviterID: 3, Email: "user2@example.com", Kind: ProjectInvitationKindEmail, TokenHash: "x"})
		require.NoError(t, err)
		all, err := ListReceivedProjectInvitations(s, &user.User{ID: 2})
		require.NoError(t, err)
		// The pending user invitation and the new one; never the expired one.
		require.Len(t, all, 2)
		assert.Equal(t, int64(2), all[0].ProjectID)
	})
}

func TestProjectInvitation_Claim(t *testing.T) {
	t.Run("join link stays usable", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		s := db.NewSession()
		defer s.Close()
		p, err := ClaimProjectInvitation(s, &user.User{ID: 2}, testLinkToken)
		require.NoError(t, err)
		assert.Equal(t, int64(1), p.ID)
		// Joining again changes nothing.
		_, err = ClaimProjectInvitation(s, &user.User{ID: 2}, testLinkToken)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "users_projects", map[string]any{"project_id": 1, "user_id": 2, "permission": PermissionRead}, false)
		db.AssertExists(t, "project_invitations", map[string]any{"id": 2, "uses": 1}, false)
	})
	t.Run("email invitation works once", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		s := db.NewSession()
		defer s.Close()
		_, err := ClaimProjectInvitation(s, &user.User{ID: 2}, testEmailToken)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		db.AssertExists(t, "users_projects", map[string]any{"project_id": 1, "user_id": 2, "permission": PermissionWrite}, false)
		db.AssertMissing(t, "project_invitations", map[string]any{"id": 3})
	})
	t.Run("expired token", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, err := ClaimProjectInvitation(s, &user.User{ID: 2}, testExpiredToken)
		assert.True(t, IsErrProjectInvitationDoesNotExist(err))
		_, err = ClaimProjectInvitation(s, &user.User{ID: 2}, "")
		assert.True(t, IsErrProjectInvitationDoesNotExist(err))
	})
	t.Run("preview", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		preview, err := PreviewProjectInvitation(s, testEmailToken)
		require.NoError(t, err)
		assert.Equal(t, "Test1", preview.ProjectTitle)
		assert.Equal(t, "user1", preview.InviterName)
		assert.Equal(t, ProjectInvitationKindEmail, preview.Kind)
		_, err = PreviewProjectInvitation(s, "wrong")
		assert.True(t, IsErrProjectInvitationDoesNotExist(err))
	})
}
