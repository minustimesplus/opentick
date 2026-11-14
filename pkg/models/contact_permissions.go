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

// Contacts, their groups and tags are private to their owner: nobody else
// can see, change or delete them, and link shares have no access at all.

// CanCreate checks if the user can create a contact.
func (c *Contact) CanCreate(_ *xorm.Session, a web.Auth) (bool, error) {
	_, isShareAuth := a.(*LinkSharing)
	return !isShareAuth, nil
}

// CanRead checks if the user can read a contact. It loads the contact.
func (c *Contact) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	existing, err := c.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, 0, err
	}
	*c = *existing
	return true, int(PermissionAdmin), nil
}

// CanUpdate checks if the user can update a contact, its history, notes,
// photo, tags and groups.
func (c *Contact) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	existing, err := c.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, err
	}
	// Keep the incoming fields, but the owner can never change.
	c.UserID = existing.UserID
	return true, nil
}

// CanDelete checks if the user can delete a contact. It loads the contact,
// as Delete needs its photo.
func (c *Contact) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	existing, err := c.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, err
	}
	*c = *existing
	return true, nil
}

// loadIfOwner returns the stored contact, or nil if a does not own it.
func (c *Contact) loadIfOwner(s *xorm.Session, a web.Auth) (*Contact, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, nil
	}
	existing := &Contact{ID: c.ID}
	if err := existing.load(s); err != nil {
		return nil, err
	}
	if existing.UserID != a.GetID() {
		return nil, nil
	}
	return existing, nil
}

// LoadOwnedContact returns the contact with the given id if a owns it.
// It answers ErrContactDoesNotExist for other users' contacts too, so ids
// cannot be probed.
func LoadOwnedContact(s *xorm.Session, a web.Auth, id int64) (*Contact, error) {
	c := &Contact{ID: id}
	existing, err := c.loadIfOwner(s, a)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrContactDoesNotExist{ID: id}
	}
	return existing, nil
}

// CanCreate checks if the user can create a contact group.
func (g *ContactGroup) CanCreate(_ *xorm.Session, a web.Auth) (bool, error) {
	_, isShareAuth := a.(*LinkSharing)
	return !isShareAuth, nil
}

// CanRead checks if the user can read a contact group. It loads the group.
func (g *ContactGroup) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	existing, err := g.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, 0, err
	}
	*g = *existing
	return true, int(PermissionAdmin), nil
}

// CanUpdate checks if the user can update a contact group.
func (g *ContactGroup) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	existing, err := g.loadIfOwner(s, a)
	if err != nil || existing == nil {
		return false, err
	}
	g.UserID = existing.UserID
	return true, nil
}

// CanDelete checks if the user can delete a contact group.
func (g *ContactGroup) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	existing, err := g.loadIfOwner(s, a)
	return existing != nil, err
}

func (g *ContactGroup) loadIfOwner(s *xorm.Session, a web.Auth) (*ContactGroup, error) {
	if _, isShareAuth := a.(*LinkSharing); isShareAuth {
		return nil, nil
	}
	existing := &ContactGroup{}
	has, err := s.ID(g.ID).Get(existing)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrContactGroupDoesNotExist{ID: g.ID}
	}
	if existing.UserID != a.GetID() {
		return nil, nil
	}
	return existing, nil
}
