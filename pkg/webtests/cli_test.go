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

package webtests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"code.vikunja.io/api/pkg/cli"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type cliRun struct {
	code   int
	stdout string
	stderr string
}

// runCLI runs the CLI against a real HTTP server, the way a script on another machine would.
func runCLI(t *testing.T, server, token, stdin string, args ...string) cliRun {
	t.Helper()
	env := map[string]string{
		"OPENTICK_URL":    server,
		"OPENTICK_TOKEN":  token,
		"OPENTICK_CONFIG": t.TempDir() + "/cli.json",
	}
	var stdout, stderr bytes.Buffer
	code := cli.Run(context.Background(), "otk", args, cli.IO{
		Stdin:  strings.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
		Getenv: func(k string) string { return env[k] },
	})
	return cliRun{
		code:   code,
		stdout: stdout.String(),
		stderr: stderr.String(),
	}
}

func (r cliRun) json(t *testing.T) map[string]any {
	t.Helper()
	require.Equal(t, 0, r.code, "stderr: %s", r.stderr)
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(r.stdout), &out), r.stdout)
	return out
}

func TestCLI_AgainstServer(t *testing.T) {
	c := newAllScopesMCPClient(t)
	srv := httptest.NewServer(c.e)
	t.Cleanup(srv.Close)
	token := c.token

	t.Run("lists the same tools as MCP, including catalog actions", func(t *testing.T) {
		run := runCLI(t, srv.URL, token, "", "tools")
		require.Equal(t, 0, run.code, run.stderr)
		var tools []map[string]any
		require.NoError(t, json.Unmarshal([]byte(run.stdout), &tools))
		names := map[string]bool{}
		for _, tool := range tools {
			names[tool["name"].(string)] = true
			assert.NotNil(t, tool["input_schema"], tool["name"])
		}
		for name := range c.toolNames() {
			if name != "find_action" && name != "do_action" {
				assert.True(t, names[name], name)
			}
		}
		for _, a := range findActions(t, c, map[string]any{}) {
			assert.True(t, names[a["name"].(string)], a["name"])
		}
	})

	t.Run("help shows a tool's schema", func(t *testing.T) {
		out := runCLI(t, srv.URL, token, "", "help", "add-to-group").json(t)
		assert.Equal(t, "add_to_group", out["name"])
		assert.Equal(t, []any{"group"}, out["positional"])
		assert.Contains(t, out["input_schema"].(map[string]any)["properties"], "contact_ids")
	})

	t.Run("search, then a dry-run bulk tag fed from stdin", func(t *testing.T) {
		search := runCLI(t, srv.URL, token, "", "search_contacts", "--location", "london", "--location", "new york", "--match", "any", "--ids-only")
		ids := search.json(t)["ids"].([]any)
		require.NotEmpty(t, ids)

		out := runCLI(t, srv.URL, token, search.stdout, "tag_contacts", "VIP", "--contact-ids", "-", "--contact-ids", "999999", "--dry-run").json(t)
		assert.Equal(t, true, out["dry_run"])
		assert.InDelta(t, len(ids), out["changed"], 0)
		assert.Equal(t, []any{float64(999999)}, out["unknown_ids"])

		tags := runCLI(t, srv.URL, token, "", "list_tags").json(t)
		for _, tag := range tags["tags"].([]any) {
			assert.NotEqual(t, "VIP", tag.(map[string]any)["name"], "a dry run must not create the tag")
		}
	})

	t.Run("positional arguments and --set", func(t *testing.T) {
		out := runCLI(t, srv.URL, token, "", "update_contact", "1", "--set", "title=Countess").json(t)
		assert.Equal(t, "Countess", out["title"])
		out = runCLI(t, srv.URL, token, "", "add_note", "1", "Wrote the first program").json(t)
		assert.Equal(t, "Wrote the first program", out["body"])
	})

	t.Run("tasks through typed tools and the catalog", func(t *testing.T) {
		out := runCLI(t, srv.URL, token, "", "tasks_list", "--per-page", "2").json(t)
		assert.NotEmpty(t, out["items"])
		out = runCLI(t, srv.URL, token, "", "call", "project_views_list", `{"project":1}`).json(t)
		assert.NotEmpty(t, out["items"])
	})

	t.Run("errors are JSON on stderr", func(t *testing.T) {
		run := runCLI(t, srv.URL, token, "", "search_contacts", "--bogus", "1")
		assert.Equal(t, 1, run.code)
		assert.Contains(t, run.stderr, `"error"`)
		assert.Contains(t, run.stderr, "otk help search_contacts")

		run = runCLI(t, srv.URL, token, "", "delete_contact", "1")
		assert.Equal(t, 1, run.code)
		assert.Contains(t, run.stderr, "unknown tool")

		run = runCLI(t, srv.URL, "tk_0000000000000000000000000000000000000000", "", "list_tags")
		assert.Equal(t, 1, run.code)
		assert.Contains(t, run.stderr, "401")
	})
}
