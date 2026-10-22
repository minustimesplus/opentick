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
	"net/http"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/models"

	"github.com/danielgtaylor/huma/v2"
)

type trashListBody struct {
	Body Paginated[*models.Task]
}

type trashEmptyBody struct {
	Body struct {
		Deleted int `json:"deleted" doc:"How many tasks were permanently deleted."`
	}
}

func RegisterTaskTrashRoutes(api huma.API) {
	tags := []string{"tasks"}
	trashScope := " The trash holds the deleted tasks of every non-archived project the authenticated user can write to; link shares have no trash."

	Register(api, huma.Operation{
		OperationID: "tasks-trash-list",
		Summary:     "List trashed tasks",
		Description: "Returns deleted tasks, most recently deleted first. Each task carries deleted_at." + trashScope,
		Method:      http.MethodGet,
		Path:        "/tasks/trash",
		Tags:        tags,
	}, tasksTrashList)

	Register(api, huma.Operation{
		OperationID: "tasks-trash-restore",
		Summary:     "Restore a trashed task",
		Description: "Moves the task out of the trash, back into its project and that project's views. Returns the restored task." + trashScope,
		Method:      http.MethodPost,
		Path:        "/tasks/trash/{task}/restore",
		Tags:        tags,
		// Restoring puts back an existing task, it creates nothing.
		DefaultStatus: http.StatusOK,
	}, tasksTrashRestore)

	Register(api, huma.Operation{
		OperationID: "tasks-trash-delete",
		Summary:     "Delete a trashed task forever",
		Description: "Permanently deletes one task from the trash, with its comments, attachments, reminders and other related data. This cannot be undone." + trashScope,
		Method:      http.MethodDelete,
		Path:        "/tasks/trash/{task}",
		Tags:        tags,
	}, tasksTrashDelete)

	Register(api, huma.Operation{
		OperationID:   "tasks-trash-empty",
		Summary:       "Empty the trash",
		Description:   "Permanently deletes every task in the authenticated user's trash. This cannot be undone." + trashScope,
		Method:        http.MethodPost,
		Path:          "/tasks/trash/empty",
		Tags:          tags,
		DefaultStatus: http.StatusOK,
	}, tasksTrashEmpty)
}

func init() { AddRouteRegistrar(RegisterTaskTrashRoutes) }

func tasksTrashList(ctx context.Context, in *struct {
	ListParams
	Format string `query:"format" enum:"html,markdown" doc:"How rich-text fields are exchanged. See the API description."`
}) (*trashListBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	tasks, total, err := models.GetTrashedTasks(s, a, in.Q, in.Page, in.PerPage)
	if err != nil {
		return nil, translateDomainError(err)
	}
	convertTasksToMarkdown(ctx, tasks...)
	return &trashListBody{Body: NewPaginated(tasks, total, in.Page, in.PerPage)}, nil
}

func tasksTrashRestore(ctx context.Context, in *struct {
	ID     int64  `path:"task" doc:"The numeric id of the trashed task."`
	Format string `query:"format" enum:"html,markdown" doc:"How rich-text fields are exchanged. See the API description."`
}) (*singleBody[models.Task], error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	task, err := models.RestoreTrashedTask(s, a, in.ID)
	if err != nil {
		_ = s.Rollback()
		events.CleanupPending(s)
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		events.CleanupPending(s)
		return nil, translateDomainError(err)
	}
	events.DispatchPending(ctx, s)
	convertTasksToMarkdown(ctx, task)
	return &singleBody[models.Task]{Body: task}, nil
}

func tasksTrashDelete(ctx context.Context, in *struct {
	ID int64 `path:"task" doc:"The numeric id of the trashed task."`
}) (*emptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	if err := models.DeleteTrashedTask(s, a, in.ID); err != nil {
		_ = s.Rollback()
		events.CleanupPending(s)
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		events.CleanupPending(s)
		return nil, translateDomainError(err)
	}
	events.DispatchPending(ctx, s)
	return &emptyBody{}, nil
}

func tasksTrashEmpty(ctx context.Context, _ *struct{}) (*trashEmptyBody, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	deleted, err := models.EmptyTrash(s, a)
	if err != nil {
		_ = s.Rollback()
		events.CleanupPending(s)
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		events.CleanupPending(s)
		return nil, translateDomainError(err)
	}
	events.DispatchPending(ctx, s)
	out := &trashEmptyBody{}
	out.Body.Deleted = deleted
	return out, nil
}
