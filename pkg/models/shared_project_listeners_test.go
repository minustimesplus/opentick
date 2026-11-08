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
	"encoding/json"
	"testing"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/events"
	"code.vikunja.io/api/pkg/notifications"
	"code.vikunja.io/api/pkg/user"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/stretchr/testify/require"
)

func TestTaskCompletedEvent(t *testing.T) {
	db.LoadAndAssertFixtures(t)
	s := db.NewSession()
	defer s.Close()
	task := &Task{ID: 1, Done: true}
	require.NoError(t, task.Update(s, &user.User{ID: 1}))
	require.NoError(t, s.Commit())
	events.DispatchPending(context.Background(), s)
	events.AssertDispatched(t, &TaskCompletedEvent{})
}

func TestSendSharedProjectActivityNotification(t *testing.T) {
	// Project 3 belongs to user3 and is shared with user1 and user2.
	handle := func(t *testing.T, activity sharedActivity, doerID int64) {
		payload, err := json.Marshal(&TaskCompletedEvent{
			Task: &Task{ID: 32, Title: "task #32", ProjectID: 3},
			Doer: &user.User{ID: doerID, Username: "doer"},
		})
		require.NoError(t, err)
		l := &SendSharedProjectActivityNotification{activity: activity}
		require.NoError(t, l.Handle(message.NewMessage(watermill.NewUUID(), payload)))
	}
	enable := func(t *testing.T, userID int64, col string) {
		s := db.NewSession()
		defer s.Close()
		_, err := s.Exec("UPDATE users SET "+col+" = ? WHERE id = ?", true, userID)
		require.NoError(t, err)
		require.NoError(t, s.Commit())
	}

	t.Run("notifies members who asked for it", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		enable(t, 2, "notify_shared_task_done")
		handle(t, sharedActivityDone, 3)
		notifications.AssertSent(t, &SharedTaskDoneNotification{})
	})
	t.Run("nobody asked", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		handle(t, sharedActivityDone, 3)
		notifications.AssertNotSent(t, &SharedTaskDoneNotification{})
	})
	t.Run("not the doer", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		enable(t, 2, "notify_shared_task_created")
		handle(t, sharedActivityCreated, 2)
		notifications.AssertNotSent(t, &TaskCreatedNotification{})
	})
	t.Run("deleted", func(t *testing.T) {
		db.LoadAndAssertFixtures(t)
		fakeNotifications(t)
		enable(t, 1, "notify_shared_task_deleted")
		handle(t, sharedActivityDeleted, 3)
		notifications.AssertSent(t, &TaskDeletedNotification{})
	})
}
