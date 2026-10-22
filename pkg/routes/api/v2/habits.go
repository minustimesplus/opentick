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
	"code.vikunja.io/api/pkg/models"
	"code.vikunja.io/api/pkg/web/handler"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/conditional"
)

type habitListBody struct {
	Body Paginated[*models.Habit]
}

// RegisterHabitRoutes wires habit CRUD, check-ins and stats onto the Huma API.
func RegisterHabitRoutes(api huma.API) {
	tags := []string{"habits"}

	Register(api, huma.Operation{
		OperationID: "habits-list",
		Summary:     "List habits",
		Description: "Returns the authenticated user's habits, archived ones included, each with today's value, current streak and the check-ins of the last 7 days. Habits are private: only their owner sees them.",
		Method:      http.MethodGet,
		Path:        "/habits",
		Tags:        tags,
	}, habitsList)

	Register(api, huma.Operation{
		OperationID: "habits-read",
		Summary:     "Get a habit",
		Description: "Returns a single habit with today's progress, as of today in the user's timezone. Only the owner may read it.",
		Method:      http.MethodGet,
		Path:        "/habits/{id}",
		Tags:        tags,
	}, habitsRead)

	Register(api, huma.Operation{
		OperationID: "habits-create",
		Summary:     "Create a habit",
		Description: "Creates a habit for the authenticated user. Daily habits always get all weekdays; weekdays habits need at least one.",
		Method:      http.MethodPost,
		Path:        "/habits",
		Tags:        tags,
	}, habitsCreate)

	Register(api, huma.Operation{
		OperationID: "habits-update",
		Summary:     "Update a habit",
		Description: "Replaces all of a habit's fields — only the owner may update it. Use PATCH for a partial update. Check-ins are kept and re-evaluated against the new schedule and goal.",
		Method:      http.MethodPut,
		Path:        "/habits/{id}",
		Tags:        tags,
	}, habitsUpdate)

	Register(api, huma.Operation{
		OperationID: "habits-delete",
		Summary:     "Delete a habit",
		Description: "Deletes a habit with all its check-ins. Only the owner may delete it. To stop tracking a habit but keep its history, archive it instead.",
		Method:      http.MethodDelete,
		Path:        "/habits/{id}",
		Tags:        tags,
	}, habitsDelete)

	Register(api, huma.Operation{
		OperationID: "habits-checkins-list",
		Summary:     "List a habit's check-ins",
		Description: "Returns the check-ins between from and to, both inclusive, oldest first. The range may span at most 366 days.",
		Method:      http.MethodGet,
		Path:        "/habits/{id}/checkins",
		Tags:        tags,
	}, habitsCheckinsList)

	Register(api, huma.Operation{
		OperationID: "habits-checkins-set",
		Summary:     "Check in to a habit",
		Description: "Sets how much of the habit was done on a day, replacing that day's previous value. A value of 0 removes the check-in. " +
			"Returns the habit with its updated progress. Days more than one day ahead of UTC are refused.",
		Method: http.MethodPut,
		Path:   "/habits/{id}/checkins/{date}",
		Tags:   tags,
	}, habitsCheckinsSet)

	Register(api, huma.Operation{
		OperationID: "habits-checkins-delete",
		Summary:     "Remove a check-in",
		Description: "Removes the check-in of a day. Succeeds even if there was none.",
		Method:      http.MethodDelete,
		Path:        "/habits/{id}/checkins/{date}",
		Tags:        tags,
	}, habitsCheckinsDelete)

	Register(api, huma.Operation{
		OperationID: "habits-stats",
		Summary:     "Get a habit's stats",
		Description: "Returns the current and longest streak, the completion rate and totals since the habit started, plus a calendar of one month. " +
			"Only days the habit is due on count towards streaks and the completion rate.",
		Method: http.MethodGet,
		Path:   "/habits/{id}/stats",
		Tags:   tags,
	}, habitsStats)
}

func init() { AddRouteRegistrar(RegisterHabitRoutes) }

func habitsList(ctx context.Context, in *struct {
	ListParams
	Today string `query:"today" doc:"The day today_value, current_streak and recent_checkins are relative to, YYYY-MM-DD — normally the client's local date. Defaults to today in the user's timezone setting."`
}) (*habitListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	result, _, total, err := handler.DoReadAll(ctx, &models.Habit{Today: in.Today}, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	items, ok := result.([]*models.Habit)
	if !ok {
		return nil, fmt.Errorf("habits ReadAll returned unexpected type %T (expected []*models.Habit)", result)
	}
	return &habitListBody{Body: NewPaginated(items, total, in.Page, in.PerPage)}, nil
}

type habitReadBody struct {
	models.Habit
	MaxPermission models.Permission `json:"max_permission" readOnly:"true" doc:"The maximum permission the requesting user has on this habit. Always 2 (admin), as only the owner can read it."`
}

func habitsRead(ctx context.Context, in *struct {
	ID int64 `path:"id"`
	conditional.Params
}) (*singleReadBody[habitReadBody], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	h := &models.Habit{ID: in.ID}
	maxPermission, err := handler.DoReadOne(ctx, h, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	body := &habitReadBody{Habit: *h, MaxPermission: models.Permission(maxPermission)}
	// Check-ins bump updated, but today's progress also changes when the day does.
	modified := h.Updated
	if today, err := models.ParseHabitDate(h.Today); err == nil && today.After(modified) {
		modified = today
	}
	return conditionalReadResponse(&in.Params, body, modified, maxPermission)
}

func habitsCreate(ctx context.Context, in *struct {
	Body models.Habit
}) (*singleBody[models.Habit], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoCreate(ctx, &in.Body, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Habit]{Body: &in.Body}, nil
}

// Body matches the read shape so AutoPatch's GET→PUT echo of max_permission validates.
func habitsUpdate(ctx context.Context, in *struct {
	ID   int64 `path:"id"`
	Body habitReadBody
}) (*singleBody[models.Habit], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	h := &in.Body.Habit
	h.ID = in.ID // URL wins over body
	if err := handler.DoUpdate(ctx, h, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Habit]{Body: h}, nil
}

func habitsDelete(ctx context.Context, in *struct {
	ID int64 `path:"id"`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	if err := handler.DoDelete(ctx, &models.Habit{ID: in.ID}, a); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

type habitCheckinsBody struct {
	Body []*models.HabitCheckin
}

func habitsCheckinsList(ctx context.Context, in *struct {
	ID   int64  `path:"id"`
	From string `query:"from" required:"true" doc:"First day of the range, YYYY-MM-DD."`
	To   string `query:"to" required:"true" doc:"Last day of the range (inclusive), YYYY-MM-DD."`
}) (*habitCheckinsBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	from, err := models.ParseHabitDate(in.From)
	if err != nil {
		return nil, translateDomainError(err)
	}
	to, err := models.ParseHabitDate(in.To)
	if err != nil {
		return nil, translateDomainError(err)
	}

	s := db.NewSession()
	defer s.Close()

	h := &models.Habit{ID: in.ID}
	can, _, err := h.CanRead(s, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	if !can {
		return nil, huma.Error403Forbidden("forbidden")
	}
	checkins, err := h.GetCheckins(s, from, to)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &habitCheckinsBody{Body: checkins}, nil
}

type habitCheckinInput struct {
	Value int `json:"value" minimum:"0" maximum:"100000" doc:"How much was done that day. 0 removes the check-in."`
}

func habitsCheckinsSet(ctx context.Context, in *struct {
	ID    int64  `path:"id"`
	Date  string `path:"date" doc:"The day to check in for, YYYY-MM-DD."`
	Today string `query:"today" doc:"The day the returned progress is relative to, YYYY-MM-DD — normally the client's local date. Defaults to today in the user's timezone setting."`
	Body  habitCheckinInput
}) (*singleBody[models.Habit], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	day, err := models.ParseHabitDate(in.Date)
	if err != nil {
		return nil, translateDomainError(err)
	}

	s := db.NewSession()
	defer s.Close()

	h := &models.Habit{ID: in.ID, Today: in.Today}
	can, err := h.CanUpdate(s, a)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if !can {
		_ = s.Rollback()
		return nil, huma.Error403Forbidden("forbidden")
	}
	if err := h.SetCheckin(s, a, day, in.Body.Value); err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &singleBody[models.Habit]{Body: h}, nil
}

func habitsCheckinsDelete(ctx context.Context, in *struct {
	ID   int64  `path:"id"`
	Date string `path:"date" doc:"The day to remove the check-in of, YYYY-MM-DD."`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	day, err := models.ParseHabitDate(in.Date)
	if err != nil {
		return nil, translateDomainError(err)
	}

	s := db.NewSession()
	defer s.Close()

	h := &models.Habit{ID: in.ID}
	can, err := h.CanUpdate(s, a)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if !can {
		_ = s.Rollback()
		return nil, huma.Error403Forbidden("forbidden")
	}
	if err := h.DeleteCheckin(s, day); err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &emptyBody{}, nil
}

type habitStatsBody struct {
	Body *models.HabitStats
}

func habitsStats(ctx context.Context, in *struct {
	ID    int64  `path:"id"`
	Today string `query:"today" doc:"The day streaks and the completion rate are counted up to, YYYY-MM-DD — normally the client's local date. Defaults to today in the user's timezone setting."`
	Month string `query:"month" doc:"The month to return the calendar of, YYYY-MM. Defaults to the month of today."`
}) (*habitStatsBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}

	s := db.NewSession()
	defer s.Close()

	h := &models.Habit{ID: in.ID}
	can, _, err := h.CanRead(s, a)
	if err != nil {
		return nil, translateDomainError(err)
	}
	if !can {
		return nil, huma.Error403Forbidden("forbidden")
	}
	stats, err := h.GetStats(s, a, in.Today, in.Month)
	if err != nil {
		return nil, translateDomainError(err)
	}
	return &habitStatsBody{Body: stats}, nil
}
