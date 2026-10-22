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
	"context"
	"testing"
	"time"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/files"
	"code.vikunja.io/api/pkg/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task 51 is trashed in the fixtures (project 1, owned by user 1).
var task51DeletedAt = time.Date(2018, 12, 1, 1, 12, 4, 0, time.UTC)

// trashTasks soft-deletes tasks the way Task.Delete does, without its side effects.
func trashTasks(t *testing.T, ids ...int64) {
	t.Helper()
	s := db.NewSession()
	defer s.Close()
	for _, id := range ids {
		_, err := s.ID(id).Delete(&Task{})
		require.NoError(t, err)
	}
	require.NoError(t, s.Commit())
}

func trashedTaskIDs(tasks []*Task) []int64 {
	ids := make([]int64, 0, len(tasks))
	for _, t := range tasks {
		ids = append(ids, t.ID)
	}
	return ids
}

func TestGetTrashedTasks(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("only writable, non-archived projects", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		// 18: project 9, read-only for user 1. 19: project 10, writable.
		// 35: project 21, archived.
		trashTasks(t, 18, 19, 35)

		s := db.NewSession()
		defer s.Close()
		tasks, total, err := GetTrashedTasks(s, u1, "", 1, 50)
		require.NoError(t, err)

		// Most recently deleted first
		assert.Equal(t, []int64{19, 51}, trashedTaskIDs(tasks))
		assert.Equal(t, int64(2), total)
		for _, task := range tasks {
			assert.False(t, task.DeletedAt.IsZero())
			assert.NotEmpty(t, task.Identifier)
		}
	})
	t.Run("search", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		trashTasks(t, 19)

		s := db.NewSession()
		defer s.Close()
		tasks, total, err := GetTrashedTasks(s, u1, "soft deleted", 1, 50)
		require.NoError(t, err)
		assert.Equal(t, []int64{51}, trashedTaskIDs(tasks))
		assert.Equal(t, int64(1), total)
	})
	t.Run("pagination", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		trashTasks(t, 19)

		s := db.NewSession()
		defer s.Close()
		tasks, total, err := GetTrashedTasks(s, u1, "", 2, 1)
		require.NoError(t, err)
		assert.Equal(t, []int64{51}, trashedTaskIDs(tasks))
		assert.Equal(t, int64(2), total)
	})
	t.Run("other user does not see it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		tasks, total, err := GetTrashedTasks(s, &user.User{ID: 2}, "", 1, 50)
		require.NoError(t, err)
		assert.Empty(t, tasks)
		assert.Zero(t, total)
	})
	t.Run("link share", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, _, err := GetTrashedTasks(s, &LinkSharing{ID: 2, ProjectID: 1, Permission: PermissionWrite}, "", 1, 50)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestRestoreTrashedTask(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("delete and restore round trip", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		require.NoError(t, (&Task{ID: 1}).Delete(s, u1))
		require.NoError(t, s.Commit())
		db.AssertMissing(t, "task_buckets", map[string]interface{}{"task_id": 1})

		s = db.NewSession()
		defer s.Close()
		restored, err := RestoreTrashedTask(s, u1, 1)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
		events.DispatchPending(context.Background(), s)
		events.AssertDispatched(t, &TaskUpdatedEvent{})

		assert.True(t, restored.DeletedAt.IsZero())
		db.AssertExists(t, "tasks", map[string]interface{}{"id": 1, "deleted_at": nil}, false)
		// The views get their bucket and position rows back
		db.AssertExists(t, "task_buckets", map[string]interface{}{"task_id": 1}, false)
		db.AssertExists(t, "task_positions", map[string]interface{}{"task_id": 1}, false)

		s = db.NewSession()
		defer s.Close()
		task := &Task{ID: 1}
		require.NoError(t, task.ReadOne(s, u1))
		assert.Equal(t, "task #1", task.Title)

		result, _, _, err := (&TaskCollection{ProjectID: 1}).ReadAll(s, u1, "", 1, 50)
		require.NoError(t, err)
		assert.Contains(t, trashedTaskIDs(result.([]*Task)), int64(1))

		// Not reported as deleted to CalDAV any more
		deletions, err := GetDeletedTasksSince(s, 1, time.Time{})
		require.NoError(t, err)
		assert.NotContains(t, trashedTaskIDs(deletions), int64(1))

		tasks, _, err := GetTrashedTasks(s, u1, "", 1, 50)
		require.NoError(t, err)
		assert.Equal(t, []int64{51}, trashedTaskIDs(tasks))
	})
	t.Run("task that is not trashed", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, err := RestoreTrashedTask(s, u1, 1)
		require.Error(t, err)
		assert.True(t, IsErrTaskDoesNotExist(err))
	})
	t.Run("nonexistent task", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, err := RestoreTrashedTask(s, u1, 9999)
		require.Error(t, err)
		assert.True(t, IsErrTaskDoesNotExist(err))
	})
	t.Run("no access to the project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		_, err := RestoreTrashedTask(s, &user.User{ID: 2}, 51)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
	t.Run("read-only access to the project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		trashTasks(t, 18)
		s := db.NewSession()
		defer s.Close()
		_, err := RestoreTrashedTask(s, u1, 18)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
	t.Run("archived project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		trashTasks(t, 36)
		s := db.NewSession()
		defer s.Close()
		_, err := RestoreTrashedTask(s, u1, 36)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
	})
}

func TestDeleteTrashedTask(t *testing.T) {
	u1 := &user.User{ID: 1}

	t.Run("normal", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		files.InitTestFileFixtures(t)
		s := db.NewSession()
		defer s.Close()
		require.NoError(t, DeleteTrashedTask(s, u1, 51))
		require.NoError(t, s.Commit())

		db.AssertMissing(t, "tasks", map[string]interface{}{"id": 51})
		db.AssertMissing(t, "label_tasks", map[string]interface{}{"task_id": 51})
		db.AssertMissing(t, "task_reminders", map[string]interface{}{"task_id": 51})

		s = db.NewSession()
		defer s.Close()
		project, err := GetProjectSimpleByID(s, 1)
		require.NoError(t, err)
		assert.True(t, project.TasksPurgedUntil.Equal(task51DeletedAt), "got %s", project.TasksPurgedUntil)
	})
	t.Run("task that is not trashed", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := DeleteTrashedTask(s, u1, 1)
		require.Error(t, err)
		assert.True(t, IsErrTaskDoesNotExist(err))
		require.NoError(t, s.Commit())
		db.AssertExists(t, "tasks", map[string]interface{}{"id": 1}, false)
	})
	t.Run("no access to the project", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		s := db.NewSession()
		defer s.Close()
		err := DeleteTrashedTask(s, &user.User{ID: 2}, 51)
		require.Error(t, err)
		assert.True(t, IsErrGenericForbidden(err))
		require.NoError(t, s.Commit())
		db.AssertExists(t, "tasks", map[string]interface{}{"id": 51}, false)
	})
}

func TestEmptyTrash(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	files.InitTestFileFixtures(t)
	trashTasks(t, 18, 19)

	s := db.NewSession()
	defer s.Close()
	deleted, err := EmptyTrash(s, &user.User{ID: 1})
	require.NoError(t, err)
	require.NoError(t, s.Commit())

	assert.Equal(t, 2, deleted)
	db.AssertMissing(t, "tasks", map[string]interface{}{"id": 51})
	db.AssertMissing(t, "tasks", map[string]interface{}{"id": 19})
	// Read-only for user 1, so it stays in the owner's trash
	db.AssertExists(t, "tasks", map[string]interface{}{"id": 18}, false)
}

func TestSyncTokenMissesPurgedDeletions(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	files.InitTestFileFixtures(t)

	check := func(since time.Time) bool {
		s := db.NewSession()
		defer s.Close()
		misses, err := SyncTokenMissesPurgedDeletions(s, 1, since)
		require.NoError(t, err)
		return misses
	}

	recent := time.Now().Add(-time.Hour)
	assert.False(t, check(recent))
	assert.True(t, check(time.Now().Add(-31*24*time.Hour)), "older than the retention period")

	s := db.NewSession()
	defer s.Close()
	require.NoError(t, (&Task{ID: 1}).Delete(s, &user.User{ID: 1}))
	require.NoError(t, s.Commit())

	s = db.NewSession()
	defer s.Close()
	require.NoError(t, DeleteTrashedTask(s, &user.User{ID: 1}, 1))
	require.NoError(t, s.Commit())

	// The token predates the purged deletion, so its 404 can't be reported any more
	assert.True(t, check(recent))
	assert.False(t, check(time.Now().Add(time.Second)))
}

// Trashed tasks must not leak through the paths that reach tasks through
// another table.
func TestTrashedTaskLeaks(t *testing.T) {
	u1 := &user.User{ID: 1}

	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	_, err := s.Insert(&TaskRelation{TaskID: 1, OtherTaskID: 51, RelationKind: RelationKindRelated, CreatedByID: 1})
	require.NoError(t, err)
	_, err = s.Insert(&Favorite{EntityID: 51, UserID: 1, Kind: FavoriteKindTask})
	require.NoError(t, err)
	_, err = s.Exec("UPDATE tasks SET uid = ? WHERE id = ?", "trashed-uid-51", 51)
	require.NoError(t, err)
	require.NoError(t, s.Commit())

	s = db.NewSession()
	defer s.Close()

	t.Run("related tasks", func(t *testing.T) {
		task := &Task{ID: 1}
		require.NoError(t, task.ReadOne(s, u1))
		for _, related := range task.RelatedTasks {
			assert.NotContains(t, trashedTaskIDs(related), int64(51))
		}
	})
	t.Run("by index", func(t *testing.T) {
		_, err := GetTaskByProjectAndIndex(s, 1, 34)
		require.Error(t, err)
		assert.True(t, IsErrTaskDoesNotExist(err))
	})
	t.Run("by uid", func(t *testing.T) {
		tasks, err := GetTasksByUIDs(s, []string{"trashed-uid-51"}, u1)
		if err == nil {
			assert.Empty(t, tasks)
		}
	})
	t.Run("favorites", func(t *testing.T) {
		result, _, _, err := (&TaskCollection{ProjectID: FavoritesPseudoProject.ID}).ReadAll(s, u1, "", 1, 50)
		require.NoError(t, err)
		assert.NotContains(t, trashedTaskIDs(result.([]*Task)), int64(51))
	})
	t.Run("search and filters", func(t *testing.T) {
		result, _, _, err := (&TaskCollection{ProjectID: 1, Filter: "done = false"}).ReadAll(s, u1, "soft deleted", 1, 50)
		require.NoError(t, err)
		assert.Empty(t, result.([]*Task))
	})
	t.Run("comments and attachments", func(t *testing.T) {
		comment := &TaskComment{TaskID: 51}
		can, _, err := comment.CanRead(s, u1)
		assert.False(t, can)
		require.Error(t, err)

		attachment := &TaskAttachment{TaskID: 51}
		can, _, err = attachment.CanRead(s, u1)
		assert.False(t, can)
		require.Error(t, err)
	})
	t.Run("update", func(t *testing.T) {
		can, err := (&Task{ID: 51}).CanUpdate(s, u1)
		assert.False(t, can)
		require.Error(t, err)
		assert.True(t, IsErrTaskDoesNotExist(err))
	})
}
