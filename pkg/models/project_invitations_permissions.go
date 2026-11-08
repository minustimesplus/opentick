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

// CanCreate checks if the user may invite people to the project.
func (pi *ProjectInvitation) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	return pi.canManage(s, a)
}

// CanDelete checks if the user may revoke the invitation. The invitation must
// belong to the project in the path.
func (pi *ProjectInvitation) CanDelete(s *xorm.Session, a web.Auth) (bool, error) {
	exists, err := s.Where("id = ? AND project_id = ?", pi.ID, pi.ProjectID).Exist(&ProjectInvitation{})
	if err != nil {
		return false, err
	}
	if !exists {
		return false, ErrProjectInvitationDoesNotExist{}
	}
	return pi.canManage(s, a)
}

// canManage: only project admins (and instance admins) manage invitations,
// the same as direct shares.
func (pi *ProjectInvitation) canManage(s *xorm.Session, a web.Auth) (bool, error) {
	return (&ProjectUser{ProjectID: pi.ProjectID}).canDoProjectUser(s, a)
}
