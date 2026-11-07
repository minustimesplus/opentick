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

package mcp

import (
	"maps"
	"slices"
	"sort"
	"strings"

	"code.vikunja.io/api/pkg/config"
	"code.vikunja.io/api/pkg/models"
	apiv2 "code.vikunja.io/api/pkg/routes/api/v2"
)

// ConnectionInfo reports which token permissions the tools of this module can actually use.
func (m *Module) ConnectionInfo() apiv2.ConnectionSettings {
	routes := map[string]models.APITokenRoute{
		"mcp": {"access": {Path: RoutePrefix, Method: "ANY"}},
	}
	typed := models.APIPermissions{}
	for group, permissions := range models.GetAPITokenRoutes() {
		for permission := range permissions {
			token := &models.APIToken{APIPermissions: models.APIPermissions{group: {permission}}}
			for _, t := range m.order {
				if !token.CanUseRoute(t.echoPath, t.op.Method) {
					continue
				}
				if routes[group] == nil {
					routes[group] = models.APITokenRoute{}
				}
				routes[group][permission] = &models.RouteDetail{
					Path:   t.echoPath,
					Method: t.op.Method,
				}
				if t.typed && !slices.Contains(typed[group], permission) {
					typed[group] = append(typed[group], permission)
				}
			}
		}
	}
	for _, permissions := range typed {
		sort.Strings(permissions)
	}
	return apiv2.ConnectionSettings{
		Endpoint: strings.TrimRight(config.ServicePublicURL.GetString(), "/") + RoutePrefix,
		Routes:   routes,
		Presets: apiv2.TokenPresets{
			ReadOnly: map[string][]string{"*": {"read_one", "read_all"}},
			Typed:    typed,
			Full:     map[string]string{"*": "*"},
		},
	}
}

// PermissionsForPreset expands an access preset (typed, read_only, full) the
// same way the MCP settings UI does: against the permissions exposed tools can
// use, with `*` as a group or permission wildcard. The result always includes
// mcp:access. ok is false for an unknown preset.
func (m *Module) PermissionsForPreset(preset string) (permissions models.APIPermissions, ok bool) {
	info := m.ConnectionInfo()
	var wanted map[string][]string
	switch preset {
	case "typed":
		wanted = info.Presets.Typed
	case "read_only":
		wanted = info.Presets.ReadOnly
	case "full":
		wanted = map[string][]string{}
		for group, permission := range info.Presets.Full {
			wanted[group] = []string{permission}
		}
	default:
		return nil, false
	}

	out := models.APIPermissions{}
	for group, perms := range wanted {
		targets := []string{group}
		if group == "*" {
			targets = slices.Collect(maps.Keys(info.Routes))
		}
		for _, target := range targets {
			for available := range info.Routes[target] {
				if slices.Contains(perms, "*") || slices.Contains(perms, available) {
					if !slices.Contains(out[target], available) {
						out[target] = append(out[target], available)
					}
				}
			}
		}
	}
	out["mcp"] = []string{"access"}
	for _, perms := range out {
		sort.Strings(perms)
	}
	return out, true
}
