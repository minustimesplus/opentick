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

package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func toolFromSchema(t *testing.T, name, schema string, positional ...string) *toolInfo {
	t.Helper()
	var s map[string]any
	require.NoError(t, json.Unmarshal([]byte(schema), &s))
	return &toolInfo{
		Name:        name,
		Positional:  positional,
		InputSchema: s,
	}
}

const searchSchema = `{"type":"object","properties":{
	"text":{"type":"string"},
	"school":{"type":["array","null"],"items":{"type":"string"}},
	"match":{"type":"string","enum":["all","any"]},
	"limit":{"type":"integer"},
	"ids_only":{"type":"boolean"}}}`

const groupSchema = `{"type":"object","properties":{
	"group":{"type":"string"},
	"contact_ids":{"type":["array","null"],"items":{"type":"integer"}},
	"create":{"type":"boolean"},
	"dry_run":{"type":"boolean"}}}`

const updateSchema = `{"type":"object","properties":{
	"id":{"type":"integer"},
	"company":{"type":["string","null"]},
	"title":{"type":["string","null"]},
	"percent_done":{"type":["number","null"]}}}`

func parse(t *testing.T, tool *toolInfo, stdin string, args ...string) (map[string]any, error) {
	t.Helper()
	return parseArgs(tool, args, &stdinOnce{r: strings.NewReader(stdin)}, "otk")
}

func TestParseArgs_FollowSchemaTypes(t *testing.T) {
	search := toolFromSchema(t, "search_contacts", searchSchema, "text")
	got, err := parse(t, search, "", "free text", "--school", "a", "--school=b", "--match", "any", "--limit", "5", "--ids-only")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"text":     "free text",
		"school":   []any{"a", "b"},
		"match":    "any",
		"limit":    int64(5),
		"ids_only": true,
	}, got)

	group := toolFromSchema(t, "add_to_group", groupSchema, "group")
	got, err = parse(t, group, "", "G", "--contact-ids", "1", "--contact-ids", "2,3", "--no-create", "--dry-run=false")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"group":       "G",
		"contact_ids": []any{int64(1), int64(2), int64(3)},
		"create":      false,
		"dry_run":     false,
	}, got)

	update := toolFromSchema(t, "update_contact", updateSchema, "id")
	got, err = parse(t, update, "", "42", "--set", "company=BCG", "--set=title=Partner", "--percent-done", "0.5")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"id":           int64(42),
		"company":      "BCG",
		"title":        "Partner",
		"percent_done": 0.5,
	}, got)
}

func TestParseArgs_Errors(t *testing.T) {
	group := toolFromSchema(t, "add_to_group", groupSchema, "group")
	for _, args := range [][]string{
		{"--bogus", "1"},
		{"G", "extra"},
		{"--contact-ids", "x"},
		{"--group"},
		{"--set", "nope=1"},
	} {
		_, err := parse(t, group, "", args...)
		require.Error(t, err, "%v", args)
	}
	_, err := parse(t, group, "", "--bogus", "1")
	assert.ErrorContains(t, err, "otk help add_to_group")
}

func TestParseArgs_IDsFromStdin(t *testing.T) {
	group := toolFromSchema(t, "add_to_group", groupSchema, "group")
	for _, stdin := range []string{
		`{"total":3,"returned":3,"ids":[1,2,3]}`,
		`{"contacts":[{"id":1,"name":"a"},{"id":2},{"id":3}]}`,
		`[1,2,3]`,
		`["1","2","3"]`,
		"1\n2\n3\n",
		"1, 2 3",
	} {
		got, err := parse(t, group, stdin, "Reims", "--contact-ids", "-", "--dry-run")
		require.NoError(t, err, stdin)
		assert.Equal(t, []any{int64(1), int64(2), int64(3)}, got["contact_ids"], stdin)
	}
}

func TestIDsFrom_PaginatedList(t *testing.T) {
	assert.Equal(t, []any{json.Number("7"), json.Number("8")}, idsFrom(`{"items":[{"id":7,"title":"a"},{"id":8}],"page":1}`))
}

func TestEndpointFor(t *testing.T) {
	for in, want := range map[string]string{
		"https://tasks.example.com":               "https://tasks.example.com/api/v2/mcp",
		"https://tasks.example.com:8443/":         "https://tasks.example.com:8443/api/v2/mcp",
		"https://tasks.example.com/api/v2/mcp/":   "https://tasks.example.com/api/v2/mcp",
		"box.tailnet.ts.net:8443":                 "https://box.tailnet.ts.net:8443/api/v2/mcp",
		"http://localhost:3456":                   "http://localhost:3456/api/v2/mcp",
		"https://example.com/opentick/api/v2/mcp": "https://example.com/opentick/api/v2/mcp",
	} {
		got, err := endpointFor(in)
		require.NoError(t, err, in)
		assert.Equal(t, want, got, in)
	}
	_, err := endpointFor("")
	assert.Error(t, err)
}

func TestResolve_FlagsOverEnvOverFile(t *testing.T) {
	path := t.TempDir() + "/cli.json"
	require.NoError(t, writeConfig(path, fileConfig{
		Server: "https://file.example",
		Token:  "tk_file",
	}))
	env := map[string]string{envToken: "tk_env"}
	getenv := func(k string) string { return env[k] }

	st, err := resolve(globalFlags{config: path}, getenv)
	require.NoError(t, err)
	assert.Equal(t, "https://file.example", st.server)
	assert.Equal(t, "file", st.serverSource)
	assert.Equal(t, "tk_env", st.token)
	assert.Equal(t, "env", st.tokenSource)

	st, err = resolve(globalFlags{
		config: path,
		server: "https://flag.example",
		token:  "tk_flag",
	}, getenv)
	require.NoError(t, err)
	assert.Equal(t, "https://flag.example", st.server)
	assert.Equal(t, "tk_flag", st.token)
}
