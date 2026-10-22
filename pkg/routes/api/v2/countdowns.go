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
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
)

type countdownListBody struct {
	Body Paginated[*models.Countdown]
}

// RegisterCountdownRoutes wires countdown CRUD onto the Huma API.
func RegisterCountdownRoutes(api huma.API) {
	tags := []string{"countdowns"}

	Register(api, huma.Operation{
		OperationID: "countdowns-list",
		Summary:     "List countdowns",
		Description: "Returns the authenticated user's countdowns, anniversaries and birthdays, ordered by date. Countdowns are private: only their owner sees them.",
		Method:      http.MethodGet,
		Path:        "/countdowns",
		Tags:        tags,
	}, countdownsList)

	Register(api, huma.Operation{
		OperationID: "countdowns-read",
		Summary:     "Get a countdown",
		Description: "Returns a single countdown. Only the owner may read it. Sends an ETag; pass it as If-None-Match on a later read to get a 304 Not Modified.",
		Method:      http.MethodGet,
		Path:        "/countdowns/{id}",
		Tags:        tags,
	}, countdownsRead)

	Register(api, huma.Operation{
		OperationID: "countdowns-create",
		Summary:     "Create a countdown",
		Description: "Creates a countdown owned by the authenticated user. The time of day of `date` is dropped.",
		Method:      http.MethodPost,
		Path:        "/countdowns",
		Tags:        tags,
	}, countdownsCreate)

	Register(api, huma.Operation{
		OperationID: "countdowns-update",
		Summary:     "Update a countdown",
		Description: "Replaces all of a countdown's fields — only the owner may update it. Use PATCH for a partial update.",
		Method:      http.MethodPut,
		Path:        "/countdowns/{id}",
		Tags:        tags,
	}, countdownsUpdate)

	Register(api, huma.Operation{
		OperationID: "countdowns-delete",
		Summary:     "Delete a countdown",
		Description: "Deletes a countdown. Only the owner may delete it.",
		Method:      http.MethodDelete,
		Path:        "/countdowns/{id}",
		Tags:        tags,
	}, countdownsDelete)
}

func init() { AddRouteRegistrar(RegisterCountdownRoutes) }

func countdownsList(ctx context.Context, in *ListParams) (*countdownListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.Countdown{}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.Countdown)
	if !ok {
		return nil, fmt.Errorf("countdowns ReadAll returned unexpected type %T (expected []*models.Countdown)", result)
	}
	return &countdownListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

type countdownReadBody struct {
	models.Countdown
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The maximum permission the requesting user has on this countdown. Always 2 (admin), as only the owner can read it."`
}

func countdownsRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[countdownReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	c := &models.Countdown{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, c, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &countdownReadBody{Countdown: *c, MaxPermission: models.Permission(maxPermission)}
	return conditionalReadResponse(&in.Params, body, c.Updated, maxPermission)
}

func countdownsCreate(ctx context.Context, in *struct {
	Body models.Countdown
}) (*singleBody[models.Countdown], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Countdown]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func countdownsUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body countdownReadBody
}) (*singleBody[models.Countdown], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	c := &in.Body.Countdown
	c.ID = in.ID // URL wins over body
	if err := handler.DoUpdate(ctx, c, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Countdown]{Body: c}, nil
}

func countdownsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.Countdown{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}
