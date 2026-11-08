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
	"fmt"
	"net/http"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/log"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"xorm.io/xorm"
)

type projectInvitationListBody struct {
	Body Paginated[*models.ProjectInvitation]
}

type receivedProjectInvitationsBody struct {
	Body []*models.ProjectInvitation
}

type projectInvitationTokenBody struct {
	Token string `json:"token" minLength:"1" doc:"The secret token from the invitation link (the last part of /invite/<token>)."`
}

type joinedProject struct {
	ProjectID int64 `json:"project_id" doc:"The project the user now has access to."`
}

// RegisterProjectInvitationRoutes registers inviting people to a project and
// answering invitations.
func RegisterProjectInvitationRoutes(api huma.API) {
	tags := []string{"sharing"}

	Register(api, huma.Operation{
		OperationID: "project-invitations-list",
		Summary:     "List a project's open invitations",
		Description: "Returns the invitations nobody answered yet and the join links of the project. Only project admins may see them. Email invitations show only the address, never the account it belongs to.",
		Method:      http.MethodGet,
		Path:        "/projects/{project}/invitations",
		Tags:        tags,
	}, projectInvitationsList)

	Register(api, huma.Operation{
		OperationID: "project-invitations-create",
		Summary:     "Invite someone to a project",
		Description: "Set exactly one of username, email and link. A user invited by username joins right away (status accepted) unless they confirm invitations in their settings; then the invitation waits for them. An email invitation is mailed to the address and valid for 30 days; its invite_url is returned only here, so it can be shared by hand when the server has no mailer. A join link works for anyone signed in until it is deleted. Only project admins may invite.",
		Method:      http.MethodPost,
		Path:        "/projects/{project}/invitations",
		Tags:        tags,
	}, projectInvitationsCreate)

	Register(api, huma.Operation{
		OperationID: "project-invitations-delete",
		Summary:     "Revoke an invitation or join link",
		Description: "Only project admins may revoke invitations. People who already joined keep their access.",
		Method:      http.MethodDelete,
		Path:        "/projects/{project}/invitations/{invitation}",
		Tags:        tags,
	}, projectInvitationsDelete)

	Register(api, huma.Operation{
		OperationID: "project-invitations-received",
		Summary:     "List the invitations you received",
		Description: "Returns the invitations waiting for the authenticated user. Email invitations sent to the user's address before they had an account show up here too, once the address is verified.",
		Method:      http.MethodGet,
		Path:        "/project-invitations",
		Tags:        tags,
	}, projectInvitationsReceived)

	Register(api, huma.Operation{
		OperationID: "project-invitations-accept",
		Summary:     "Accept an invitation",
		Description: "Shares the project with the authenticated user with the invitation's permission and removes the invitation.",
		Method:      http.MethodPost,
		Path:        "/project-invitations/{invitation}/accept",
		// Joins an existing project, creates no resource at this path.
		DefaultStatus: http.StatusOK,
		Tags:          tags,
	}, projectInvitationsAccept)

	Register(api, huma.Operation{
		OperationID: "project-invitations-decline",
		Summary:     "Decline an invitation",
		Description: "Removes an invitation the authenticated user received.",
		Method:      http.MethodDelete,
		Path:        "/project-invitations/{invitation}",
		Tags:        tags,
	}, projectInvitationsDecline)

	Register(api, huma.Operation{
		OperationID: "project-invitations-claim",
		Summary:     "Join through an invitation link",
		Description: "Uses the token of an email invitation or a join link to share the project with the authenticated user. An email invitation works once; a join link until it is revoked. Joining a project the user already has access to succeeds without changing their permission.",
		Method:      http.MethodPost,
		Path:        "/project-invitations/claim",
		// Joins an existing project, creates no resource at this path.
		DefaultStatus: http.StatusOK,
		Tags:          tags,
	}, projectInvitationsClaim)

	Register(api, huma.Operation{
		OperationID: "project-invitations-preview",
		Summary:     "Look at an invitation link",
		Description: "Available without authentication, so the invitee can see the project's title and who invited them before signing up. Invalid, expired and used tokens return 404.",
		Method:      http.MethodPost,
		Path:        "/project-invitations/preview",
		// Reads, despite the POST that keeps the token out of URLs and logs.
		DefaultStatus: http.StatusOK,
		Tags:          tags,
		Security:      publicSecurity,
	}, projectInvitationsPreview)
}

func init() { AddRouteRegistrar(RegisterProjectInvitationRoutes) }

func projectInvitationsList(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
	ListParams
}) (*projectInvitationListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.ProjectInvitation{ProjectID: in.ProjectID}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.ProjectInvitation)
	if !ok {
		return nil, fmt.Errorf("projectInvitations.ReadAll returned unexpected type %T (expected []*models.ProjectInvitation)", result)
	}
	return &projectInvitationListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

func projectInvitationsCreate(ctx context.Context, in *struct {
	ProjectID int64 `path:"project"`
	Body      models.ProjectInvitation
}) (*singleBody[models.ProjectInvitation], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	in.Body.ProjectID = in.ProjectID // URL wins over body
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	// DoCreate committed; a failed mail must not undo the invitation, the
	// inviter still gets invite_url to share by hand.
	if err := in.Body.SendInvitationMail(); err != nil {
		log.Errorf("Could not send the invitation mail for project %d: %s", in.ProjectID, err)
	}
	return &singleBody[models.ProjectInvitation]{Body: &in.Body}, nil
}

func projectInvitationsDelete(ctx context.Context, in *struct {
	ProjectID    int64 `path:"project"`
	InvitationID int64 `path:"invitation"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.ProjectInvitation{ID: in.InvitationID, ProjectID: in.ProjectID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

// withInvitee runs fn for the authenticated user in a transaction.
func withInvitee(ctx context.Context, fn func(s *xorm.Session, u *user.User) error) error {
	a, err := authFromCtx(ctx)
	if err != nil {
		return err
	}
	u, err := user.GetFromAuth(a)
	if err != nil {
		return translateDomainError(err)
	}
	s := db.NewSession()
	defer s.Close()
	if err := fn(s, u); err != nil {
		_ = s.Rollback()
		return translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return translateDomainError(err)
	}
	return nil
}

func projectInvitationsReceived(ctx context.Context, _ *struct{}) (*receivedProjectInvitationsBody, error) {
	out := &receivedProjectInvitationsBody{}
	err := withInvitee(ctx, func(s *xorm.Session, u *user.User) (err error) {
		out.Body, err = models.ListReceivedProjectInvitations(s, u)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func projectInvitationsAccept(ctx context.Context, in *struct {
	InvitationID int64 `path:"invitation"`
}) (*singleBody[joinedProject], error) {
	out := &joinedProject{}
	err := withInvitee(ctx, func(s *xorm.Session, u *user.User) error {
		p, err := models.AcceptProjectInvitation(s, u, in.InvitationID)
		if err != nil {
			return err
		}
		out.ProjectID = p.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[joinedProject]{Body: out}, nil
}

func projectInvitationsDecline(ctx context.Context, in *struct {
	InvitationID int64 `path:"invitation"`
}) (*emptyBody, error) {
	err := withInvitee(ctx, func(s *xorm.Session, u *user.User) error {
		return models.DeclineProjectInvitation(s, u, in.InvitationID)
	})
	if err != nil {
		return nil, err
	}
	return &emptyBody{}, nil
}

func projectInvitationsClaim(ctx context.Context, in *struct {
	Body projectInvitationTokenBody
}) (*singleBody[joinedProject], error) {
	out := &joinedProject{}
	err := withInvitee(ctx, func(s *xorm.Session, u *user.User) error {
		p, err := models.ClaimProjectInvitation(s, u, in.Body.Token)
		if err != nil {
			return err
		}
		out.ProjectID = p.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &singleBody[joinedProject]{Body: out}, nil
}

func projectInvitationsPreview(_ context.Context, in *struct {
	Body projectInvitationTokenBody
}) (*singleBody[models.ProjectInvitationPreview], error) {
	s := db.NewSession()
	defer s.Close()
	preview, err := models.PreviewProjectInvitation(s, in.Body.Token)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.ProjectInvitationPreview]{Body: preview}, nil
}
