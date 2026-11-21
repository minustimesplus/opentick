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
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

func toolNameFor(operationID string) string {
	if o, ok := toolOverrides[operationID]; ok && o.name != "" {
		return o.name
	}
	return strings.ReplaceAll(operationID, "-", "_")
}

// toolOverride renames a tool, replaces its description and names its positional CLI arguments.
type toolOverride struct {
	name        string
	description string
	positional  []string
}

// The contact tools keep the Personal CRM agent's names and wording, so prompts and scripts written for it carry over.
var toolOverrides = map[string]toolOverride{
	"contacts-search": {
		name:        "search_contacts",
		description: "Find contacts by structured filters and/or free text. Each result says why it matched (e.g. \"school: Sciences Po (Master, 2019–2021)\"), so you can check edge cases before acting. Tip: call distinct_values first to discover spelling variants of a school, company or location. Pass ids_only to get every matching id for a bulk write.",
		positional:  []string{"text"},
	},
	"contacts-read": {
		name:        "get_contact",
		description: "Everything about one contact: fields, experience, education, notes, tags and groups.",
		positional:  []string{"id"},
	},
	"contacts-distinct-values": {
		name:        "distinct_values",
		description: "List the distinct values of a field across all contacts, with how many contacts have each. Use it to find every spelling of a school, company or city before searching.",
		positional: []string{
			"field",
			"contains",
		},
	},
	"contact-groups-list": {
		name:        "list_groups",
		description: "All groups with id, name, description, color and member count.",
	},
	"contact-tags-list": {
		name:        "list_tags",
		description: "All tags with id, name and how many contacts have each.",
	},
	"contact-groups-create": {
		name:        "create_group",
		description: "Create a group. If one with the same name (ignoring case) exists, it is reused and updated.",
		positional:  []string{"name"},
	},
	"contact-groups-update": {
		name:        "update_group",
		description: "Rename a group or change its description or color. Takes the group id from list_groups; only the fields you pass change.",
		positional:  []string{"id"},
	},
	"contacts-add-to-group": {
		name:        "add_to_group",
		description: "Add contacts to a group. Reports how many were added, how many were already members, and unknown ids. Use dry_run to preview.",
		positional:  []string{"group"},
	},
	"contacts-remove-from-group": {
		name:        "remove_from_group",
		description: "Remove contacts from a group. The contacts themselves stay. Reports how many were removed, how many were not members, and unknown ids. Use dry_run to preview.",
		positional:  []string{"group"},
	},
	"contacts-tag": {
		name:        "tag_contacts",
		description: "Add a tag to contacts (the tag is created if needed). Reports how many were tagged, how many already had it, and unknown ids. Use dry_run to preview.",
		positional:  []string{"tag"},
	},
	"contacts-untag": {
		name:        "untag_contacts",
		description: "Remove a tag from contacts. Reports how many were untagged, how many did not have it, and unknown ids. Use dry_run to preview.",
		positional:  []string{"tag"},
	},
	"contacts-update": {
		name:        "update_contact",
		description: "Set fields on a contact: full_name, headline, company, title, location, email, phone, linkedin_url, birthday (YYYY-MM-DD or MM-DD) and about. Only the fields you pass change; an empty string clears a field. Edited fields are locked so LinkedIn enrichment won't overwrite them, the same as editing in the app.",
		positional:  []string{"id"},
	},
	"contacts-create": {
		name:        "create_contact",
		description: "Create a contact. Needs a name, LinkedIn URL or email. Refuses if one with the same LinkedIn URL or email already exists (the error names it) unless allow_duplicate is set.",
		positional:  []string{"full_name"},
	},
	"contacts-notes-create": {
		name:        "add_note",
		description: "Add a note to a contact.",
		positional: []string{
			"id",
			"body",
		},
	},
	"contacts-experiences-create": {
		name:        "add_experience",
		description: "Add a job to a contact's experience. Needs a company or a title; leave end_date out for a current job. Locks the list against LinkedIn re-imports, like a hand edit.",
		positional:  []string{"id"},
	},
	"contacts-educations-create": {
		name:        "add_education",
		description: "Add a school to a contact's education. Locks the list against LinkedIn re-imports, like a hand edit.",
		positional:  []string{"id"},
	},
}

type toolTier int

const (
	catalogTool toolTier = iota
	typedTool
)

// Anything not listed stays off MCP; credentials, account, webhooks, shares, file transfer and admin routes are deliberately absent.
// Contacts have no delete tool on purpose.
var exposedOperations = map[string]toolTier{
	"buckets-create":                  catalogTool,
	"buckets-delete":                  catalogTool,
	"buckets-list":                    catalogTool,
	"buckets-update":                  catalogTool,
	"contact-groups-create":           typedTool,
	"contact-groups-list":             typedTool,
	"contact-groups-update":           typedTool,
	"contact-tags-list":               typedTool,
	"contacts-add-to-group":           typedTool,
	"contacts-create":                 typedTool,
	"contacts-distinct-values":        typedTool,
	"contacts-educations-create":      typedTool,
	"contacts-experiences-create":     typedTool,
	"contacts-notes-create":           typedTool,
	"contacts-read":                   typedTool,
	"contacts-remove-from-group":      typedTool,
	"contacts-search":                 typedTool,
	"contacts-tag":                    typedTool,
	"contacts-untag":                  typedTool,
	"contacts-update":                 typedTool,
	"filters-create":                  catalogTool,
	"filters-delete":                  catalogTool,
	"filters-read":                    catalogTool,
	"filters-update":                  catalogTool,
	"labels-create":                   typedTool,
	"labels-delete":                   typedTool,
	"labels-list":                     typedTool,
	"labels-read":                     typedTool,
	"labels-update":                   typedTool,
	"notifications-delete-all":        catalogTool,
	"notifications-list":              catalogTool,
	"notifications-mark-all-read":     catalogTool,
	"notifications-mark-read":         catalogTool,
	"project-tasks-list":              catalogTool,
	"project-teams-create":            catalogTool,
	"project-teams-delete":            catalogTool,
	"project-teams-list":              catalogTool,
	"project-teams-update":            catalogTool,
	"project-time-entries-list":       catalogTool,
	"project-users-create":            catalogTool,
	"project-users-delete":            catalogTool,
	"project-users-list":              catalogTool,
	"project-users-update":            catalogTool,
	"project-view-buckets-tasks-list": catalogTool,
	"project-view-tasks-list":         catalogTool,
	"project-views-create":            catalogTool,
	"project-views-delete":            catalogTool,
	"project-views-list":              catalogTool,
	"project-views-read":              catalogTool,
	"project-views-update":            catalogTool,
	"projects-create":                 typedTool,
	"projects-delete":                 typedTool,
	"projects-duplicate":              catalogTool,
	"projects-list":                   typedTool,
	"projects-read":                   typedTool,
	"projects-update":                 typedTool,
	"projects-users-search":           catalogTool,
	"reactions-create":                catalogTool,
	"reactions-delete":                catalogTool,
	"reactions-list":                  catalogTool,
	"task-assignees-bulk":             catalogTool,
	"task-assignees-create":           typedTool,
	"task-assignees-delete":           typedTool,
	"task-assignees-list":             typedTool,
	"task-attachments-delete":         catalogTool,
	"task-attachments-list":           catalogTool,
	"task-bucket-update":              catalogTool,
	"task-comments-create":            typedTool,
	"task-comments-delete":            typedTool,
	"task-comments-list":              typedTool,
	"task-comments-read":              typedTool,
	"task-comments-update":            typedTool,
	"task-labels-bulk-replace":        catalogTool,
	"task-labels-create":              catalogTool,
	"task-labels-delete":              catalogTool,
	"task-labels-list":                catalogTool,
	"task-time-entries-list":          catalogTool,
	"tasks-bulk-create":               catalogTool,
	"tasks-bulk-update":               catalogTool,
	"tasks-create":                    typedTool,
	"tasks-delete":                    typedTool,
	"tasks-duplicate":                 catalogTool,
	"tasks-list":                      typedTool,
	"tasks-mark-read":                 catalogTool,
	"tasks-position-update":           catalogTool,
	"tasks-read":                      typedTool,
	"tasks-read-by-index":             catalogTool,
	"tasks-skip-occurrence":           typedTool,
	"tasks-relations-create":          catalogTool,
	"tasks-relations-delete":          catalogTool,
	"tasks-update":                    typedTool,
	"teams-create":                    catalogTool,
	"teams-delete":                    catalogTool,
	"teams-list":                      catalogTool,
	"teams-members-add":               catalogTool,
	"teams-members-remove":            catalogTool,
	"teams-members-toggle-admin":      catalogTool,
	"teams-read":                      catalogTool,
	"teams-update":                    catalogTool,
	"time-entries-create":             catalogTool,
	"time-entries-delete":             catalogTool,
	"time-entries-list":               catalogTool,
	"time-entries-read":               catalogTool,
	"time-entries-timer-stop":         catalogTool,
	"time-entries-update":             catalogTool,
	"users-search":                    typedTool,
}

// ExposedToolNames are the tool names the allow-list can produce. Exported so tests can check it against the live v2 routes.
func ExposedToolNames() []string {
	names := make([]string, 0, len(exposedOperations))
	for id := range exposedOperations {
		names = append(names, toolNameFor(id))
	}
	return names
}

func exposure(operationID string, op *huma.Operation) (typed bool, ok bool) {
	tier, exposed := exposedOperations[operationID]
	if !exposed {
		return false, false
	}
	if op.RequestBody != nil {
		if _, schema := bodyMedia(op); schema == nil {
			return false, false
		}
	}
	return tier == typedTool, true
}
func bodyMedia(op *huma.Operation) (contentType string, schema *huma.Schema) {
	if op.RequestBody == nil {
		return "", nil
	}
	for _, ct := range []string{
		"application/json",
		"application/merge-patch+json",
	} {
		if m := op.RequestBody.Content[ct]; m != nil && m.Schema != nil {
			return ct, m.Schema
		}
	}
	return "", nil
}
