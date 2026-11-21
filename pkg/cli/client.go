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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"code.vikunja.io/api/pkg/version"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// The CLI is an MCP client of the server's own endpoint, so its commands are
// exactly the tools the server exposes to the token, and can never drift from them.

const (
	statusUnreachable = -1
	toolFindAction    = "find_action"
	toolDoAction      = "do_action"
	positionalMetaKey = "opentick/positional"
)

// toolInfo is one command: a typed MCP tool or a catalog action reached through do_action.
type toolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Positional  []string       `json:"positional,omitempty"`
	Catalog     bool           `json:"catalog,omitempty"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
}

type client struct {
	endpoint string
	session  *mcp.ClientSession
	http     *recordingTransport
	tools    []*toolInfo
}

// recordingTransport adds the token and keeps the last failed response, whose
// status and body explain a failure better than the SDK's wrapped error.
type recordingTransport struct {
	token string
	base  http.RoundTripper

	mu      sync.Mutex
	failure string
	status  int
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("Authorization", "Bearer "+t.token)
	req.Header.Set("User-Agent", "opentick-cli/"+version.Version)
	res, err := t.base.RoundTrip(req)
	if err != nil {
		t.mu.Lock()
		t.status, t.failure = statusUnreachable, err.Error()
		t.mu.Unlock()
		return res, err
	}
	if res.StatusCode < 400 || req.Method != http.MethodPost {
		return res, nil
	}
	body, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
	_ = res.Body.Close()
	res.Body = io.NopCloser(bytes.NewReader(body))
	t.mu.Lock()
	t.status = res.StatusCode
	t.failure = strings.TrimSpace(string(body))
	t.mu.Unlock()
	return res, nil
}

func (t *recordingTransport) reset() {
	t.mu.Lock()
	t.status, t.failure = 0, ""
	t.mu.Unlock()
}

// explain prefers the failed HTTP response; without one the error is the server's JSON-RPC error or a network failure.
func (t *recordingTransport) explain(endpoint string, err error) error {
	t.mu.Lock()
	status, failure := t.status, t.failure
	t.mu.Unlock()
	switch status {
	case statusUnreachable:
		return fmt.Errorf("cannot reach %s: %s", endpoint, failure)
	case 0:
		var rpcErr *jsonrpc.Error
		if errors.As(err, &rpcErr) {
			return errors.New(rpcErr.Message)
		}
		return fmt.Errorf("cannot reach %s: %w", endpoint, err)
	}
	var msg struct {
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if json.Unmarshal([]byte(failure), &msg) == nil {
		switch {
		case msg.Detail != "":
			failure = msg.Detail
		case msg.Message != "":
			failure = msg.Message
		}
	}
	hint := ""
	switch status {
	case http.StatusUnauthorized:
		hint = " (the token is missing, invalid or expired: create one under Settings → AI Features and run `login`)"
	case http.StatusForbidden:
		hint = " (the token needs the mcp access permission)"
	case http.StatusNotFound:
		hint = " (is this an OpenTick server?)"
	}
	return fmt.Errorf("%s answered %d: %s%s", endpoint, status, failure, hint)
}

func connect(ctx context.Context, st settings) (*client, error) {
	endpoint, err := endpointFor(st.server)
	if err != nil {
		return nil, err
	}
	if st.token == "" {
		return nil, fmt.Errorf("no token configured: run `login` or set %s", envToken)
	}
	rt := &recordingTransport{
		token: st.token,
		base:  http.DefaultTransport,
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint:             endpoint,
		HTTPClient:           &http.Client{Transport: rt},
		MaxRetries:           -1,
		DisableStandaloneSSE: true,
	}
	c := mcp.NewClient(&mcp.Implementation{
		Name:    "opentick-cli",
		Version: version.Version,
	}, nil)
	session, err := c.Connect(ctx, transport, nil)
	if err != nil {
		return nil, rt.explain(endpoint, err)
	}
	return &client{
		endpoint: endpoint,
		session:  session,
		http:     rt,
	}, nil
}

func (c *client) close() { _ = c.session.Close() }

// listTools returns the typed tools followed by the catalog actions.
func (c *client) listTools(ctx context.Context) ([]*toolInfo, error) {
	if c.tools != nil {
		return c.tools, nil
	}
	var out []*toolInfo
	hasCatalog := false
	c.http.reset()
	for t, err := range c.session.Tools(ctx, nil) {
		if err != nil {
			return nil, c.http.explain(c.endpoint, err)
		}
		switch t.Name {
		case toolFindAction:
			hasCatalog = true
			continue
		case toolDoAction:
			continue
		}
		schema, err := asObject(t.InputSchema)
		if err != nil {
			return nil, fmt.Errorf("tool %s: %w", t.Name, err)
		}
		out = append(out, &toolInfo{
			Name:        t.Name,
			Description: t.Description,
			Positional:  stringList(t.Meta[positionalMetaKey]),
			InputSchema: schema,
		})
	}
	if hasCatalog {
		actions, err := c.findActions(ctx, map[string]any{})
		if err != nil {
			return nil, err
		}
		out = append(out, actions...)
	}
	c.tools = out
	return out, nil
}

func (c *client) findActions(ctx context.Context, args map[string]any) ([]*toolInfo, error) {
	res, err := c.callRaw(ctx, toolFindAction, args)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Actions []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Positional  []string       `json:"positional"`
			InputSchema map[string]any `json:"input_schema"`
		} `json:"actions"`
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("unexpected find_action result: %w", err)
	}
	out := make([]*toolInfo, 0, len(parsed.Actions))
	for _, a := range parsed.Actions {
		out = append(out, &toolInfo{
			Name:        a.Name,
			Description: a.Description,
			Positional:  a.Positional,
			Catalog:     true,
			InputSchema: a.InputSchema,
		})
	}
	return out, nil
}

var errUnknownTool = errors.New("unknown tool")

// tool resolves a command name; catalog actions get their full schema from find_action.
func (c *client) tool(ctx context.Context, name string) (*toolInfo, error) {
	name = strings.ReplaceAll(name, "-", "_")
	tools, err := c.listTools(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tools {
		if t.Name != name {
			continue
		}
		if t.Catalog && t.InputSchema == nil {
			full, err := c.findActions(ctx, map[string]any{"action": name})
			if err != nil {
				return nil, err
			}
			if len(full) == 1 {
				t.InputSchema = full[0].InputSchema
				t.Positional = full[0].Positional
			}
		}
		return t, nil
	}
	return nil, fmt.Errorf("%w: %s (the token may not allow it; see `help`)", errUnknownTool, name)
}

func (c *client) call(ctx context.Context, t *toolInfo, args map[string]any) (any, error) {
	if t.Catalog {
		return c.callRaw(ctx, toolDoAction, map[string]any{
			"action":    t.Name,
			"arguments": args,
		})
	}
	return c.callRaw(ctx, t.Name, args)
}

func (c *client) callRaw(ctx context.Context, name string, args map[string]any) (any, error) {
	c.http.reset()
	res, err := c.session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		return nil, c.http.explain(c.endpoint, err)
	}
	text := resultText(res)
	if res.IsError {
		return nil, errors.New(text)
	}
	if res.StructuredContent != nil {
		return res.StructuredContent, nil
	}
	// Not every tool answers with JSON; such text is printed as a JSON string.
	if !json.Valid([]byte(text)) {
		return text, nil
	}
	var out any
	dec := json.NewDecoder(strings.NewReader(text))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

func resultText(res *mcp.CallToolResult) string {
	var parts []string
	for _, c := range res.Content {
		if t, ok := c.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func asObject(v any) (map[string]any, error) {
	if m, ok := v.(map[string]any); ok {
		return m, nil
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func stringList(v any) []string {
	items, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
