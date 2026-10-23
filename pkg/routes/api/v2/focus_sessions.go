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
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
)

type focusSessionListBody struct {
	Body Paginated[*models.FocusSession]
}

// RegisterFocusSessionRoutes wires the pomodoro / focus timer sessions and
// their stats onto the Huma API.
func RegisterFocusSessionRoutes(api huma.API) {
	tags := []string{"focus-sessions"}

	Register(api, huma.Operation{
		OperationID: "focus-sessions-list",
		Summary:     "List focus sessions",
		Description: "Returns the authenticated user's focus sessions, newest first. Sessions are private: only the user who recorded them sees them.",
		Method:      http.MethodGet,
		Path:        "/focus-sessions",
		Tags:        tags,
	}, focusSessionsList)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-stats",
		Summary:     "Get focus stats",
		Description: "Returns today's and all-time pomo counts and focused minutes, plus a per-day history of the last `days` days ending today. " +
			"Days are cut in `timezone`, which defaults to the user's timezone setting.",
		Method: http.MethodGet,
		Path:   "/focus-sessions/stats",
		Tags:   tags,
	}, focusSessionsStats)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-read",
		Summary:     "Get a focus session",
		Description: "Returns a single focus session. Only its owner may read it. Sends an ETag; pass it as If-None-Match on a later read to get a 304 Not Modified.",
		Method:      http.MethodGet,
		Path:        "/focus-sessions/{id}",
		Tags:        tags,
	}, focusSessionsRead)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-create",
		Summary:     "Record a focus session",
		Description: "Records a finished focus session for the authenticated user. end_time defaults to now and duration to end_time - start_time. Linking a task needs read access to it.",
		Method:      http.MethodPost,
		Path:        "/focus-sessions",
		Tags:        tags,
	}, focusSessionsCreate)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-update",
		Summary:     "Update a focus session",
		Description: "Replaces all of a focus session's fields — only the owner may update it. Use PATCH for a partial update.",
		Method:      http.MethodPut,
		Path:        "/focus-sessions/{id}",
		Tags:        tags,
	}, focusSessionsUpdate)

	Register(api, huma.Operation{
		OperationID: "focus-sessions-delete",
		Summary:     "Delete a focus session",
		Description: "Deletes a focus session. Only the owner may delete it.",
		Method:      http.MethodDelete,
		Path:        "/focus-sessions/{id}",
		Tags:        tags,
	}, focusSessionsDelete)
}

func init() { AddRouteRegistrar(RegisterFocusSessionRoutes) }

func focusSessionsList(ctx context.Context, in *struct {
	ListParams
	From   time.Time `query:"from" doc:"Only sessions starting at or after this time, RFC 3339."`
	To     time.Time `query:"to" doc:"Only sessions starting before this time, RFC 3339."`
	TaskID int64     `query:"task_id" doc:"Only sessions linked to this task."`
}) (*focusSessionListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	m := &models.FocusSession{TaskID: in.TaskID}
	if !in.From.IsZero() {
		m.From = &in.From
	}
	if !in.To.IsZero() {
		m.To = &in.To
	}
	result, _, total, err := handler.DoReadAll(ctx, m, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.FocusSession)
	if !ok {
		return nil, fmt.Errorf("focus sessions ReadAll returned unexpected type %T (expected []*models.FocusSession)", result)
	}
	return &focusSessionListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

type focusSessionReadBody struct {
	models.FocusSession
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The maximum permission the requesting user has on this session. Always 2 (admin), as only the owner can read it."`
}

func focusSessionsRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[focusSessionReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	fs := &models.FocusSession{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, fs, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &focusSessionReadBody{FocusSession: *fs, MaxPermission: models.Permission(maxPermission)}
	return conditionalReadResponse(&in.Params, body, fs.Updated, maxPermission)
}

func focusSessionsCreate(ctx context.Context, in *struct {
	Body models.FocusSession
}) (*singleBody[models.FocusSession], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.FocusSession]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func focusSessionsUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body focusSessionReadBody
}) (*singleBody[models.FocusSession], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	fs := &in.Body.FocusSession
	fs.ID = in.ID // URL wins over body
	if err := handler.DoUpdate(ctx, fs, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.FocusSession]{Body: fs}, nil
}

func focusSessionsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.FocusSession{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

type focusStatsBody struct {
	Body *models.FocusStats
}

// focusSessionsStats is scoped to the caller's own sessions, so it needs no
// resource permission beyond authentication (GetFocusStats refuses link shares).
func focusSessionsStats(ctx context.Context, in *struct {
	Days     int    `query:"days" default:"30" minimum:"1" maximum:"366" doc:"How many days of per-day history to return, ending today."`
	Timezone string `query:"timezone" doc:"IANA timezone (e.g. Europe/Berlin) to cut days in. Defaults to the user's timezone setting."`
}) (*focusStatsBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	var loc *time.Location
	if in.Timezone != "" {
		loc, err = time.LoadLocation(in.Timezone)
		if err != nil {
			return nil, huma.Error400BadRequest(fmt.Sprintf("unknown timezone %q", in.Timezone))
		}
	}

	s := db.NewSession()
	defer s.Close()

	stats, err := models.GetFocusStats(s, a, loc, in.Days, time.Now())
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &focusStatsBody{Body: stats}, nil
}
