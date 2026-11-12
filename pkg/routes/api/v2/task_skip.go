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
	"code.vikunja.io/api/pkg/models"

	"github.com/danielgtaylor/huma/v2"
)

// TaskSkipResult is the outcome of skipping one occurrence of a repeating task.
type TaskSkipResult struct {
	Deleted bool         `json:"deleted" doc:"Whether the skipped occurrence was the last of its series, so the task went to the trash."`
	Task    *models.Task `json:"task" doc:"The task, moved on to its next occurrence (or as it was, when deleted)."`
}

// RegisterTaskSkipRoutes wires skipping one occurrence of a repeating task onto the Huma API.
func RegisterTaskSkipRoutes(api huma.API) {
	Register(api, huma.Operation{
		OperationID: "tasks-skip-occurrence",
		Summary:     "Skip one occurrence of a repeating task",
		Description: "Deletes only the current occurrence of a repeating task: the task moves on to its next date the way completing it would, but is not recorded as done. When it was the last occurrence of its series the task goes to the trash and deleted is true. To delete the task with all future occurrences, delete the task itself. Needs write access; a task that doesn't repeat is a 400.",
		Method:      http.MethodPost,
		Path:        "/tasks/{task}/skip-occurrence",
		Tags:        []string{"tasks"},
	}, tasksSkipOccurrence)
}

func init() { AddRouteRegistrar(RegisterTaskSkipRoutes) }

func tasksSkipOccurrence(ctx context.Context, in *struct {
	TaskID int64 `path:"task" doc:"The numeric id of the repeating task."`
}) (*struct{ Body TaskSkipResult }, error) {
	a, err := authFromCtx(ctx)
	if err != nil {
		return nil, err
	}
	s := db.NewSession()
	defer s.Close()

	can, err := (&models.Task{ID: in.TaskID}).CanUpdate(s, a)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if !can {
		_ = s.Rollback()
		return nil, huma.Error403Forbidden("forbidden")
	}
	task, deleted, err := models.SkipTaskOccurrence(s, a, in.TaskID)
	if err != nil {
		_ = s.Rollback()
		return nil, translateDomainError(err)
	}
	if err := s.Commit(); err != nil {
		return nil, translateDomainError(err)
	}
	return &struct{ Body TaskSkipResult }{Body: TaskSkipResult{Deleted: deleted, Task: task}}, nil
}
