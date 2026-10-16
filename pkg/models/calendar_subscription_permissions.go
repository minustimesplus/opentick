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

// Calendar subscriptions are private to their owner: nobody else can see,
// change or delete them, and link shares have no access at all.

// CanCreate checks if the user can subscribe to a calendar.
func (cs *CalendarSubscription) CanCreate(_ *xorm.Session, a web.Auth) (bool, error) {
	_, isShareAuth := a.(*LinkSharing)
	return !isShareAuth, nil
}

// CanRead checks if the user can read a calendar subscription and its events.
// It loads the subscription.
func (cs *CalendarSubscription) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	existing, err := cs.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, 0, err
	}
	*cs = *existing
	return true, int(PermissionAdmin), nil
}

// CanUpdate checks if the user can update a calendar subscription.
func (cs *CalendarSubscription) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	existing, err := cs.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, err
	}
	// Keep the incoming fields, but the owner can never change.
	cs.UserID = existing.UserID
	return true, nil
}

// CanDelete checks if the user can delete a calendar subscription.
func (cs *CalendarSubscription) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	existing, err := cs.loadIfOwner(s, a)
	return existing != nil, err
}

// loadIfOwner returns the stored subscription, or nil if a does not own it.
func (cs *CalendarSubscription) loadIfOwner(s *xorm.Session, a web.Auth) (*CalendarSubscription, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, nil
	}

	existing := &CalendarSubscription{ID: cs.ID}
	if err := existing.load(s); err != nil {
		return nil, err
	}
	if existing.UserID != a.GetID() {
		return nil, nil
	}
	return existing, nil
}
