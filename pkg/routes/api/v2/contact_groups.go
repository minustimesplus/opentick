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

	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
	"xorm.io/xorm"
)

type contactGroupListBody struct {
	Body Paginated[*models.ContactGroup]
}

// RegisterContactGroupRoutes wires contact groups and tags onto the Huma API.
func RegisterContactGroupRoutes(api huma.API) {
	tags := []string{"contacts"}

	Register(api, huma.Operation{
		OperationID: "contact-groups-list",
		Summary:     "List contact groups",
		Description: "Returns the authenticated user's contact groups by name, each with its description, color and member count.",
		Method:      http.MethodGet,
		Path:        "/contact-groups",
		Tags:        tags,
	}, contactGroupsList)

	Register(api, huma.Operation{
		OperationID: "contact-groups-read",
		Summary:     "Get a contact group",
		Description: "Returns one contact group with its member count. To list its members, search contacts by group.",
		Method:      http.MethodGet,
		Path:        "/contact-groups/{id}",
		Tags:        tags,
	}, contactGroupsRead)

	Register(api, huma.Operation{
		OperationID:   "contact-groups-create",
		Summary:       "Create a contact group",
		Description:   "Creates a group. If one with the same name exists (ignoring case, accents and punctuation), it is reused and updated with the given description and color instead.",
		Method:        http.MethodPost,
		Path:          "/contact-groups",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, contactGroupsCreate)

	Register(api, huma.Operation{
		OperationID: "contact-groups-update",
		Summary:     "Update a contact group",
		Description: "Renames a group or changes its description or color. PUT replaces all three; use PATCH to change only some.",
		Method:      http.MethodPut,
		Path:        "/contact-groups/{id}",
		Tags:        tags,
	}, contactGroupsUpdate)

	Register(api, huma.Operation{
		OperationID: "contact-groups-delete",
		Summary:     "Delete a contact group",
		Description: "Deletes a group. Its contacts stay.",
		Method:      http.MethodDelete,
		Path:        "/contact-groups/{id}",
		Tags:        tags,
	}, contactGroupsDelete)

	Register(api, huma.Operation{
		OperationID: "contact-tags-list",
		Summary:     "List contact tags",
		Description: "Returns all of the authenticated user's contact tags by name, with how many contacts have each. Tags are created by tagging contacts.",
		Method:      http.MethodGet,
		Path:        "/contact-tags",
		Tags:        tags,
	}, contactTagsList)

	Register(api, huma.Operation{
		OperationID: "contact-tags-delete",
		Summary:     "Delete a contact tag",
		Description: "Deletes a tag and removes it from every contact.",
		Method:      http.MethodDelete,
		Path:        "/contact-tags/{id}",
		Tags:        tags,
	}, contactTagsDelete)
}

func init() { AddRouteRegistrar(RegisterContactGroupRoutes) }

func contactGroupsList(ctx context.Context, in *struct {
	ListParams
}) (*contactGroupListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.ContactGroup{}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.ContactGroup)
	if !ok {
		return nil, fmt.Errorf("contact groups ReadAll returned unexpected type %T (expected []*models.ContactGroup)", result)
	}
	return &contactGroupListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

type contactGroupReadBody struct {
	models.ContactGroup
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The maximum permission the requesting user has on this group. Always 2 (admin), as only the owner can read it."`
}

func contactGroupsRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[contactGroupReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	g := &models.ContactGroup{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, g, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &contactGroupReadBody{ContactGroup: *g, MaxPermission: models.Permission(maxPermission)}
	// The member count is not part of the row, so it has to be in the ETag.
	return conditionalReadResponseWithState(&in.Params, body, g.Updated, maxPermission, fmt.Sprint(g.Members))
}

func contactGroupsCreate(ctx context.Context, in *struct {
	Body models.ContactGroup
}) (*singleBody[models.ContactGroup], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.ContactGroup]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func contactGroupsUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body contactGroupReadBody
}) (*singleBody[models.ContactGroup], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	g := &in.Body.ContactGroup
	g.ID = in.ID // URL wins over body
	if err := handler.DoUpdate(ctx, g, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.ContactGroup]{Body: g}, nil
}

func contactGroupsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.ContactGroup{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

type contactTagListBody struct {
	Body struct {
		Tags []*models.ContactTag `json:"tags"`
	}
}

func contactTagsList(ctx context.Context, _ *struct{}) (*contactTagListBody, error) {
	tags, err := contactsTx(ctx, models.ListContactTags)
	if err != nil {
		return nil, err
	}
	out := &contactTagListBody{}
	out.Body.Tags = tags
	return out, nil
}

func contactTagsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	_, err := contactsTx(ctx, func(s *xorm.Session, a web.Auth) (bool, error) {
		return true, models.DeleteContactTag(s, a, in.ID)
	})
	if err != nil {
		return nil, err
	}
	return &emptyBody{}, nil
}
