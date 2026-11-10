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
	"code.vikunja.io/api/pkg/user"
	"code.vikunja.io/api/pkg/web"

	"xorm.io/xorm"
)

// bindAgentConfigUser points the config at the authenticated user; link shares have none.
func (c *AgentConfig) bindAgentConfigUser(a web.Auth) (bool, error) {
	u, err := user.GetFromAuth(a)
	if err != nil || u == nil || u.ID <= 0 {
		return false, nil //nolint:nilerr // a link share simply has no agent config
	}
	c.UserID = u.ID
	return true, nil
}

// CanRead lets every real user read their own agent config.
func (c *AgentConfig) CanRead(_ *xorm.Session, a web.Auth) (bool, int, error) {
	can, err := c.bindAgentConfigUser(a)
	return can, int(PermissionAdmin), err
}

// CanUpdate lets every real user change their own agent config.
func (c *AgentConfig) CanUpdate(_ *xorm.Session, a web.Auth) (bool, error) {
	return c.bindAgentConfigUser(a)
}

// CanRead needs read access to the list.
func (p *ProjectAgentSettings) CanRead(s *xorm.Session, a web.Auth) (bool, int, error) {
	return (&Project{ID: p.ProjectID}).CanRead(s, a)
}

// CanUpdate needs write access to the list.
func (p *ProjectAgentSettings) CanUpdate(s *xorm.Session, a web.Auth) (bool, error) {
	if _, err := user.GetFromAuth(a); err != nil {
		return false, nil //nolint:nilerr // link shares can't own an agent
	}
	return (&Project{ID: p.ProjectID}).CanUpdate(s, a)
}

// CanCreate needs write access to the task, and a real user whose agent config is used.
func (d *AgentDispatch) CanCreate(s *xorm.Session, a web.Auth) (bool, error) {
	if u, err := user.GetFromAuth(a); err != nil || u == nil || u.ID <= 0 {
		return false, nil //nolint:nilerr // link shares have no agent config
	}
	return (&Task{ID: d.TaskID}).CanUpdate(s, a)
}
