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
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/builder"
	"xorm.io/xorm"
)

// The trash holds soft-deleted tasks. A user sees the trashed tasks of every
// non-archived project they can write to, the same permission deleting a task
// needs. Only real users have a trash; link shares are refused.

func trashedTasksCond(s *xorm.Session, a web.Auth) (builder.Cond, error) {
	if _, isLinkShare := a.(*LinkSharing); isLinkShare {
		return nil, ErrGenericForbidden{}
	}

	u, err := user.GetFromAuth(a)
	if err != nil {
		return nil, err
	}

	access, err := getProjectAccessForUser(s, u.ID)
	if err != nil {
		return nil, err
	}

	writable := []int64{}
	for _, id := range access.sortedIDs {
		if p, _ := access.permission(id); p >= PermissionWrite {
			writable = append(writable, id)
		}
	}

	return builder.And(
		builder.NotNull{"tasks.deleted_at"},
		builder.In("tasks.project_id",
			builder.Select("id").From("projects").Where(builder.And(
				builder.In("id", writable),
				builder.Eq{"is_archived": false},
			)),
		),
	), nil
}

// GetTrashedTasks returns the trashed tasks the user can see, most recently deleted first.
func GetTrashedTasks(s *xorm.Session, a web.Auth, search string, page, perPage int) (tasks []*Task, totalItems int64, err error) {
	cond, err := trashedTasksCond(s, a)
	if err != nil {
		return nil, 0, err
	}
	if search != "" {
		cond = builder.And(cond, db.MultiFieldSearch([]string{"tasks.title", "tasks.description"}, search))
	}

	tasks = []*Task{}
	query := s.Unscoped().Where(cond).OrderBy("tasks.deleted_at DESC, tasks.id DESC")
	limit, start := getLimitFromPageIndex(page, perPage)
	if limit > 0 {
		query = query.Limit(limit, start)
	}
	if err = query.Find(&tasks); err != nil {
		return nil, 0, err
	}

	totalItems, err = s.Unscoped().Where(cond).Count(&Task{})
	if err != nil {
		return nil, 0, err
	}

	projectIDs := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		projectIDs = append(projectIDs, t.ProjectID)
	}
	projects, err := GetProjectsMapByIDs(s, projectIDs)
	if err != nil {
		return nil, 0, err
	}
	for _, t := range tasks {
		t.setIdentifier(projects[t.ProjectID])
	}

	return tasks, totalItems, nil
}

// getTrashedTask loads one trashed task and checks the user may restore or purge it.
func getTrashedTask(s *xorm.Session, a web.Auth, taskID int64) (*Task, error) {
	cond, err := trashedTasksCond(s, a)
	if err != nil {
		return nil, err
	}

	task := &Task{}
	exists, err := s.Unscoped().Where(builder.Eq{"tasks.id": taskID}).Get(task)
	if err != nil {
		return nil, err
	}
	if !exists || task.DeletedAt.IsZero() {
		return nil, ErrTaskDoesNotExist{ID: taskID}
	}

	visible, err := s.Unscoped().Where(builder.And(cond, builder.Eq{"tasks.id": taskID})).Exist(&Task{})
	if err != nil {
		return nil, err
	}
	if !visible {
		return nil, ErrGenericForbidden{}
	}

	return task, nil
}

// RestoreTrashedTask moves a task out of the trash and back into its project's views.
func RestoreTrashedTask(s *xorm.Session, a web.Auth, taskID int64) (*Task, error) {
	task, err := getTrashedTask(s, a, taskID)
	if err != nil {
		return nil, err
	}

	views, err := lockProjectViewsForPositionUpdate(s, task.ProjectID)
	if err != nil {
		return nil, err
	}

	// xorm's deleted tag can't clear deleted_at, so this needs raw SQL.
	if _, err = s.Exec("UPDATE tasks SET deleted_at = NULL WHERE id = ?", task.ID); err != nil {
		return nil, err
	}
	task.DeletedAt = time.Time{}

	// Bumping updated makes CalDAV sync-collection report the task again.
	if _, err = s.ID(task.ID).Cols("updated").Update(task); err != nil {
		return nil, err
	}

	// Soft delete removed the task's bucket and position rows.
	if err = insertTaskBucketsAndPositions(s, views, []*Task{task}, a, true, nil); err != nil {
		return nil, err
	}

	project, err := GetProjectSimpleByID(s, task.ProjectID)
	if err != nil {
		return nil, err
	}
	task.setIdentifier(project)

	doer, err := user.GetFromAuth(a)
	if err != nil {
		return nil, err
	}
	events.DispatchOnCommit(s, &TaskUpdatedEvent{
		Task: task,
		Doer: doer,
	})

	return task, updateProjectLastUpdated(s, &Project{ID: task.ProjectID})
}

// DeleteTrashedTask permanently deletes one task from the trash.
func DeleteTrashedTask(s *xorm.Session, a web.Auth, taskID int64) error {
	task, err := getTrashedTask(s, a, taskID)
	if err != nil {
		return err
	}
	return purgeTrashedTasks(s, []*Task{task})
}

// EmptyTrash permanently deletes every trashed task the user can see and
// returns how many were removed.
func EmptyTrash(s *xorm.Session, a web.Auth) (int, error) {
	cond, err := trashedTasksCond(s, a)
	if err != nil {
		return 0, err
	}

	tasks := []*Task{}
	if err = s.Unscoped().Where(cond).Find(&tasks); err != nil {
		return 0, err
	}

	return len(tasks), purgeTrashedTasks(s, tasks)
}

// purgeTrashedTasks hard-deletes soft-deleted tasks and advances each project's
// tasks_purged_until, so CalDAV sync tokens that could have reported those
// deletions force a full resync instead of silently missing them.
func purgeTrashedTasks(s *xorm.Session, tasks []*Task) error {
	purgedUntil := make(map[int64]time.Time)
	for _, t := range tasks {
		if err := hardDeleteTask(s, t); err != nil {
			return err
		}
		if t.DeletedAt.After(purgedUntil[t.ProjectID]) {
			purgedUntil[t.ProjectID] = t.DeletedAt
		}
	}

	for projectID, until := range purgedUntil {
		_, err := s.
			Where(builder.And(
				builder.Eq{"id": projectID},
				builder.Or(
					builder.IsNull{"tasks_purged_until"},
					builder.Lt{"tasks_purged_until": until},
				),
			)).
			Cols("tasks_purged_until").
			NoAutoTime().
			Update(&Project{TasksPurgedUntil: until})
		if err != nil {
			return err
		}
	}

	return nil
}

// SyncTokenMissesPurgedDeletions reports whether a CalDAV sync token minted at
// since can no longer see every deletion in the project, because tasks deleted
// at or after it were already purged from the trash.
func SyncTokenMissesPurgedDeletions(s *xorm.Session, projectID int64, since time.Time) (bool, error) {
	// Covers purges from before tasks_purged_until existed.
	if retention := TaskDeleteRetention(); retention > 0 && time.Since(since) > retention {
		return true, nil
	}

	project, err := GetProjectSimpleByID(s, projectID)
	if err != nil {
		return false, err
	}
	return !project.TasksPurgedUntil.IsZero() && !since.After(project.TasksPurgedUntil), nil
}
