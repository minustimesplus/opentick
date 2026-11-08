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
	"encoding/json"

	"code.vikunja.io/api/pkg/db"
	"code.vikunja.io/api/pkg/notifications"
	"code.vikunja.io/api/pkg/user"

	"github.com/ThreeDotsLabs/watermill/message"
)

type sharedActivity int

const (
	sharedActivityCreated sharedActivity = iota
	sharedActivityDone
	sharedActivityDeleted
)

// SendSharedProjectActivityNotification notifies the members of a shared
// project who asked for it (Settings > Collaborate) when someone else adds,
// completes or deletes a task there.
type SendSharedProjectActivityNotification struct {
	activity sharedActivity
}

// Name defines the name for the listener.
func (l *SendSharedProjectActivityNotification) Name() string {
	switch l.activity {
	case sharedActivityCreated:
		return "task.created.shared.notification.send"
	case sharedActivityDone:
		return "task.completed.shared.notification.send"
	case sharedActivityDeleted:
		return "task.deleted.shared.notification.send"
	}
	return ""
}

func (l *SendSharedProjectActivityNotification) wants(u *user.User) bool {
	switch l.activity {
	case sharedActivityCreated:
		return u.NotifySharedTaskCreated
	case sharedActivityDone:
		return u.NotifySharedTaskDone
	case sharedActivityDeleted:
		return u.NotifySharedTaskDeleted
	}
	return false
}

// Handle is executed when the event the listener listens on is fired.
func (l *SendSharedProjectActivityNotification) Handle(msg *message.Message) (err error) {
	// All three events share the same payload shape.
	event := &TaskCreatedEvent{}
	if err := json.Unmarshal(msg.Payload, event); err != nil {
		return err
	}
	if event.Task == nil || event.Doer == nil {
		return nil
	}

	s := db.NewSession()
	defer s.Close()

	project, err := GetProjectSimpleByID(s, event.Task.ProjectID)
	if IsErrProjectDoesNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	uids, err := getUserIDsWithProjectAccess(s, project.ID)
	if err != nil {
		return err
	}
	if len(uids) < 2 {
		return nil
	}

	// Subscribers already get the regular created/deleted notification.
	var subscribers []*SubscriptionWithUser
	switch l.activity {
	case sharedActivityCreated:
		subscribers, err = GetSubscriptionsForEntity(s, SubscriptionEntityTask, event.Task.ID)
	case sharedActivityDeleted:
		subscribers, err = GetSubscriptionsForDeletedTask(s, event.Task)
	case sharedActivityDone:
		// Completing has no regular notification to deduplicate against.
	}
	if err != nil {
		return err
	}
	skip := map[int64]bool{event.Doer.ID: true}
	for _, sub := range subscribers {
		skip[sub.UserID] = true
	}

	users, err := user.GetUsersByIDs(s, uids)
	if err != nil {
		return err
	}
	event.Task.setIdentifier(project)

	for _, u := range users {
		if skip[u.ID] || !l.wants(u) {
			continue
		}
		var n notifications.Notification
		switch l.activity {
		case sharedActivityCreated:
			n = &TaskCreatedNotification{Doer: event.Doer, Task: event.Task, Project: project}
		case sharedActivityDone:
			n = &SharedTaskDoneNotification{Doer: event.Doer, Task: event.Task, Project: project}
		case sharedActivityDeleted:
			n = &TaskDeletedNotification{Doer: event.Doer, Task: event.Task}
		}
		if err := notifications.Notify(u, n, s); err != nil {
			_ = s.Rollback()
			return err
		}
	}

	return s.Commit()
}
