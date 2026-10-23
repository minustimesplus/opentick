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
	"code.vikunja.io/api/pkg/web"
	"xorm.io/xorm"
)

// Focus sessions are private to the user who recorded them: nobody else can
// see, change or delete them, and link shares have no access at all. Linking a
// session to a task needs read access to that task.

// CanCreate checks if the user can record a focus session.
func (fs *FocusSession) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return false, nil
	}
	return canLinkFocusSessionTask(s, a, fs.TaskID)
}

// CanRead checks if the user can read a focus session. It loads the session.
func (fs *FocusSession) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	existing, err := fs.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, 0, err
	}
	*fs = *existing
	return true, int(PermissionAdmin), nil
}

// CanUpdate checks if the user can update a focus session, including read
// access to a newly linked task.
func (fs *FocusSession) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	existing, err := fs.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, err
	}
	// Keep the incoming fields, but the owner can never change.
	fs.UserID = existing.UserID
	// The task may have been shared away since: keeping an existing link is fine.
	if fs.TaskID == existing.TaskID {
		return true, nil
	}
	return canLinkFocusSessionTask(s, a, fs.TaskID)
}

// CanDelete checks if the user can delete a focus session.
func (fs *FocusSession) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	existing, err := fs.loadIfOwner(s, a)
	return existing != nil, err
}

// loadIfOwner returns the stored session, or nil if a does not own it.
func (fs *FocusSession) loadIfOwner(s *xorm.Session, a web.Auth) (*FocusSession, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, nil
	}

	existing := &FocusSession{ID: fs.ID}
	if err := existing.load(s); err != nil {
		return nil, err
	}
	if existing.UserID != a.GetID() {
		return nil, nil
	}
	return existing, nil
}

func canLinkFocusSessionTask(s *xorm.Session, a web.Auth, taskID int64) (bool, error) {
	if taskID == 0 {
		return true, nil
	}
	task, err := GetTaskByIDSimple(s, taskID)
	if err != nil {
		return false, err
	}
	can, _, err := task.CanRead(s, a)
	return can, err
}
