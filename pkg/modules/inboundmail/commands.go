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

package inboundmail

import (
	"regexp"
	"strings"

	"code.vikunja.io/api/pkg/models"
)

// verbs maps every accepted spelling of a command onto its canonical name.
var verbs = map[string]string{
	"add": "add", "create": "add", "new": "add",
	"update": "update", "edit": "update", "change": "update",
	"done": "done", "complete": "done", "finish": "done",
	"undone": "undone", "reopen": "undone", "undo": "undone",
	"delete": "delete", "remove": "delete",
	"skip":    "skip",
	"comment": "comment",
	"find":    "find", "search": "find",
	"lists": "lists",
	"help":  "help",
}

// fields maps every accepted spelling of a field onto its canonical name.
var fields = map[string]string{
	"title": "title", "name": "title",
	"list": "list", "project": "list",
	"date": "date", "due": "date", "when": "date", "start": "date", "due date": "date",
	"end": "end", "until": "end",
	"duration": "duration", "length": "duration", "for": "duration",
	"repeat": "repeat", "repeats": "repeat", "recurrence": "repeat", "rrule": "repeat",
	"repeat from": "repeat_from", "repeat_from": "repeat_from",
	"priority": "priority",
	"labels":   "labels", "label": "labels", "tags": "labels", "tag": "labels",
	"notes": "notes", "description": "notes",
	"text": "text", "message": "text",
	"scope": "scope",
}

// freeText fields swallow the lines that follow them, so notes can span paragraphs.
var freeText = map[string]bool{"notes": true, "text": true}

// command is one instruction from a mail: a verb, its argument and its fields.
type command struct {
	Verb   string
	Arg    string
	Fields map[string]string
	// Order keeps the fields in the order they were written, for error messages.
	Order []string
	// Unknown collects lines that were neither a field nor a command.
	Unknown []string
}

func (c *command) set(field, value string) {
	if _, ok := c.Fields[field]; !ok {
		c.Order = append(c.Order, field)
	}
	c.Fields[field] = value
}

func (c *command) has(field string) bool {
	_, ok := c.Fields[field]
	return ok
}

var keyLineRe = regexp.MustCompile(`^\s*[-*]?\s*([A-Za-z][A-Za-z _]{0,20}?)\s*:\s?(.*)$`)

// stopMarkers end the part of a mail that is read: whatever follows is quoted or reference text.
var stopMarkers = []string{
	strings.ToLower(models.InboundCommandReferenceMarker),
	"-----original message-----",
	"________________________________",
}

var wroteRe = regexp.MustCompile(`(?i)^on .{4,200} wrote:\s*$`)

// cleanBody drops quoted replies, signatures and the command reference OpenTick sends along.
func cleanBody(body string) string {
	body = strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		stop := line == "-- " || wroteRe.MatchString(trimmed)
		for _, m := range stopMarkers {
			if strings.HasPrefix(lower, m) {
				stop = true
			}
		}
		if stop {
			break
		}
		if strings.HasPrefix(trimmed, ">") {
			continue
		}
		out = append(out, strings.TrimRight(line, " \t"))
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// parseCommands reads the commands out of a cleaned mail body. Each command starts
// with a "verb: argument" line (or a bare "lists" / "help"); the "field: value" lines
// after it belong to it. Notes and comment text may run over several lines.
func parseCommands(body string) []*command {
	var cmds []*command
	var cur *command
	lastField := ""
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if cur != nil && freeText[lastField] {
				cur.Fields[lastField] += "\n"
			}
			continue
		}
		key, value, isKey := splitKeyLine(trimmed)
		if verb, ok := verbs[key]; ok && (isKey || verb == "lists" || verb == "help") {
			cur = &command{Verb: verb, Arg: strings.TrimSpace(value), Fields: map[string]string{}}
			cmds = append(cmds, cur)
			lastField = ""
			continue
		}
		if field, ok := fields[key]; ok && isKey && cur != nil {
			cur.set(field, strings.TrimSpace(value))
			lastField = field
			continue
		}
		if cur == nil {
			continue
		}
		if freeText[lastField] {
			if cur.Fields[lastField] != "" {
				cur.Fields[lastField] += "\n"
			}
			cur.Fields[lastField] += trimmed
			continue
		}
		cur.Unknown = append(cur.Unknown, trimmed)
	}
	for _, c := range cmds {
		for f := range freeText {
			if v, ok := c.Fields[f]; ok {
				c.Fields[f] = strings.TrimSpace(v)
			}
		}
	}
	return cmds
}

// splitKeyLine splits "Key: value" into a lowercase key and the value. A line
// without a colon returns the whole lowercase line as key, for bare verbs.
func splitKeyLine(line string) (key, value string, isKey bool) {
	if m := keyLineRe.FindStringSubmatch(line); m != nil {
		return strings.ToLower(strings.TrimSpace(m[1])), m[2], true
	}
	return strings.ToLower(strings.TrimRight(strings.TrimLeft(line, "-* "), ".!")), "", false
}
