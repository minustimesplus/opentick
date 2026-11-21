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

// Package cli is a command-line client for an OpenTick server's MCP tools,
// for scripts and AI agents:
//
//	otk search_contacts --school "sciences po" --location reims --match any
//	otk search_contacts --location reims --ids-only | otk add_to_group "Sciences Po" --contact-ids - --dry-run
//	otk call add_to_group '{"group":"Sciences Po","contact_ids":[1,2]}'
//
// Flags map to the tool's JSON-schema parameters (--contact-ids → contact_ids).
// Array parameters can be repeated; `-` reads the value from stdin. Output is
// JSON on stdout; errors go to stderr as {"error": "..."} with exit code 1.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"
)

// IO is what Run reads and writes; tests substitute all of it.
type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Getenv func(string) string
}

// StdIO is the process's own streams and environment.
func StdIO() IO {
	return IO{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Getenv: os.Getenv,
	}
}

type globalFlags struct {
	server string
	token  string
	config string
}

type runner struct {
	prog  string
	io    IO
	flags globalFlags
	stdin *stdinOnce
}

const requestTimeout = 2 * time.Minute

// Run executes one CLI invocation; prog is how the user calls it (for help texts).
func Run(ctx context.Context, prog string, args []string, streams IO) int {
	if streams.Getenv == nil {
		streams.Getenv = func(string) string { return "" }
	}
	r := &runner{
		prog:  prog,
		io:    streams,
		stdin: &stdinOnce{r: streams.Stdin},
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	out, err := r.dispatch(ctx, args)
	if err != nil {
		enc := json.NewEncoder(streams.Stderr)
		enc.SetEscapeHTML(false)
		if encErr := enc.Encode(map[string]string{"error": err.Error()}); encErr != nil {
			fmt.Fprintln(streams.Stderr, err)
		}
		return 1
	}
	if out != nil {
		enc := json.NewEncoder(streams.Stdout)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return 1
		}
	}
	return 0
}

func (r *runner) dispatch(ctx context.Context, args []string) (any, error) {
	args, err := r.parseGlobal(args)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, r.help(ctx, "")
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "help", "-h", "--help":
		topic := ""
		if len(rest) > 0 {
			topic = rest[0]
		}
		return r.helpTopic(ctx, topic)
	case "login":
		return r.login(ctx, rest)
	case "logout":
		return r.logout()
	case "config":
		return r.config(rest)
	case "tools":
		c, err := r.connect(ctx)
		if err != nil {
			return nil, err
		}
		defer c.close()
		return r.allTools(ctx, c)
	case "call":
		return r.call(ctx, rest)
	}
	c, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.close()
	t, err := c.tool(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if len(rest) == 1 && (rest[0] == "--help" || rest[0] == "-h") {
		return describeTool(r.prog, t), nil
	}
	toolArgs, err := parseArgs(t, rest, r.stdin, r.prog)
	if err != nil {
		return nil, err
	}
	return c.call(ctx, t, toolArgs)
}

// parseGlobal takes --server, --token and --config from before the command.
func (r *runner) parseGlobal(args []string) ([]string, error) {
	for len(args) > 0 {
		name, value, hasValue := strings.Cut(strings.TrimPrefix(args[0], "--"), "=")
		var dst *string
		switch {
		case !strings.HasPrefix(args[0], "--"):
			return args, nil
		case name == "server" || name == "url":
			dst = &r.flags.server
		case name == "token":
			dst = &r.flags.token
		case name == "config":
			dst = &r.flags.config
		default:
			return args, nil
		}
		args = args[1:]
		if !hasValue {
			if len(args) == 0 {
				return nil, fmt.Errorf("--%s needs a value", name)
			}
			value, args = args[0], args[1:]
		}
		*dst = value
	}
	return args, nil
}

func (r *runner) settings() (settings, error) {
	return resolve(r.flags, r.io.Getenv)
}

func (r *runner) connect(ctx context.Context) (*client, error) {
	st, err := r.settings()
	if err != nil {
		return nil, err
	}
	return connect(ctx, st)
}

func (r *runner) call(ctx context.Context, rest []string) (any, error) {
	if len(rest) == 0 {
		return nil, fmt.Errorf("usage: %s call <tool> '<json arguments>'", r.prog)
	}
	raw := "{}"
	if len(rest) > 1 {
		raw = rest[1]
	}
	if raw == "-" {
		text, err := r.stdin.get()
		if err != nil {
			return nil, err
		}
		raw = text
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var args map[string]any
	if err := dec.Decode(&args); err != nil || args == nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	c, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.close()
	t, err := c.tool(ctx, rest[0])
	if err != nil {
		return nil, err
	}
	return c.call(ctx, t, args)
}

func (r *runner) login(ctx context.Context, rest []string) (any, error) {
	if _, err := r.parseGlobal(rest); err != nil {
		return nil, err
	}
	st, err := r.settings()
	if err != nil {
		return nil, err
	}
	if st.server == "" {
		return nil, fmt.Errorf("usage: %s login --server https://tasks.example.com [--token tk_…]", r.prog)
	}
	if st.token == "" {
		fmt.Fprint(r.io.Stderr, "API token (Settings → AI Features → create a token): ")
		line, err := bufio.NewReader(r.io.Stdin).ReadString('\n')
		if err != nil && line == "" {
			return nil, errors.New("no token given")
		}
		st.token = strings.TrimSpace(line)
		st.tokenSource = "stdin"
	}
	c, err := connect(ctx, st)
	if err != nil {
		return nil, err
	}
	defer c.close()
	tools, err := c.listTools(ctx)
	if err != nil {
		return nil, err
	}
	if err := writeConfig(st.file, fileConfig{
		Server: st.server,
		Token:  st.token,
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"server":      st.server,
		"endpoint":    c.endpoint,
		"tools":       len(tools),
		"config_file": st.file,
	}, nil
}

func (r *runner) logout() (any, error) {
	st, err := r.settings()
	if err != nil {
		return nil, err
	}
	cfg, err := readConfig(st.file)
	if err != nil {
		return nil, err
	}
	cfg.Token = ""
	if err := writeConfig(st.file, cfg); err != nil {
		return nil, err
	}
	return map[string]any{
		"logged_out":  true,
		"config_file": st.file,
	}, nil
}

func (r *runner) config(rest []string) (any, error) {
	st, err := r.settings()
	if err != nil {
		return nil, err
	}
	if len(rest) > 0 {
		if rest[0] != "set" || len(rest) != 3 || (rest[1] != "server" && rest[1] != "token") {
			return nil, fmt.Errorf("usage: %s config [set server|token <value>]", r.prog)
		}
		cfg, err := readConfig(st.file)
		if err != nil {
			return nil, err
		}
		if rest[1] == "server" {
			cfg.Server = rest[2]
		} else {
			cfg.Token = rest[2]
		}
		if err := writeConfig(st.file, cfg); err != nil {
			return nil, err
		}
		return r.config(nil)
	}
	endpoint, _ := endpointFor(st.server)
	return map[string]any{
		"config_file":   st.file,
		"server":        st.server,
		"server_source": st.serverSource,
		"endpoint":      endpoint,
		"token":         maskToken(st.token),
		"token_source":  st.tokenSource,
	}, nil
}

func (r *runner) allTools(ctx context.Context, c *client) (any, error) {
	tools, err := c.listTools(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]any, 0, len(tools))
	for _, t := range tools {
		if t.Catalog && t.InputSchema == nil {
			full, err := c.tool(ctx, t.Name)
			if err != nil {
				return nil, err
			}
			t = full
		}
		out = append(out, describeTool(r.prog, t))
	}
	return out, nil
}

func (r *runner) helpTopic(ctx context.Context, topic string) (any, error) {
	if topic == "" {
		return nil, r.help(ctx, "")
	}
	c, err := r.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.close()
	t, err := c.tool(ctx, topic)
	if err != nil {
		return nil, err
	}
	return describeTool(r.prog, t), nil
}

func describeTool(prog string, t *toolInfo) map[string]any {
	usage := prog + " " + t.Name
	for _, p := range t.Positional {
		usage += " <" + p + ">"
	}
	usage += " [--flag value ...]"
	out := map[string]any{
		"name":         t.Name,
		"description":  t.Description,
		"usage":        usage,
		"input_schema": t.InputSchema,
	}
	if len(t.Positional) > 0 {
		out["positional"] = t.Positional
	}
	if t.Catalog {
		out["catalog"] = true
	}
	return out
}

func (r *runner) help(ctx context.Context, _ string) error {
	w := r.io.Stdout
	p := r.prog
	fmt.Fprintf(w, "OpenTick — command line for the server's MCP tools\n\n")
	fmt.Fprintf(w, "Usage:\n")
	fmt.Fprintf(w, "  %s login --server URL [--token TOKEN]   save the server and an API token\n", p)
	fmt.Fprintf(w, "  %s config [set server|token VALUE]      show or change the saved settings\n", p)
	fmt.Fprintf(w, "  %s <tool> [args] [--flag value]         run a tool, print JSON\n", p)
	fmt.Fprintf(w, "  %s call <tool> '<json>'                  run a tool with JSON arguments (- reads stdin)\n", p)
	fmt.Fprintf(w, "  %s help <tool>                          show a tool's parameters as JSON schema\n", p)
	fmt.Fprintf(w, "  %s tools                                every tool with its schema, as JSON\n\n", p)
	fmt.Fprintf(w, "Settings: --server/--token/--config before the command, or %s, %s and %s.\n", envServer, envToken, envConfig)
	fmt.Fprintf(w, "Flags map to parameters (--contact-ids → contact_ids); repeat array flags or pass - to read ids from stdin.\n\n")

	st, err := r.settings()
	if err == nil && st.server != "" && st.token != "" {
		c, err := connect(ctx, st)
		if err != nil {
			fmt.Fprintf(w, "Tools: unavailable (%s)\n", err)
			return nil
		}
		defer c.close()
		tools, err := c.listTools(ctx)
		if err != nil {
			fmt.Fprintf(w, "Tools: unavailable (%s)\n", err)
			return nil
		}
		printTools(w, "Tools:", tools, false)
		printTools(w, "More actions:", tools, true)
	} else {
		fmt.Fprintf(w, "Run `%s login` to list the tools your token can use.\n\n", p)
	}

	fmt.Fprintf(w, "Examples:\n")
	fmt.Fprintf(w, "  %s distinct_values school --contains \"sciences po\"\n", p)
	fmt.Fprintf(w, "  %s search_contacts --school \"sciences po\" --location reims --match any\n", p)
	fmt.Fprintf(w, "  %s search_contacts --location reims --ids-only | %s add_to_group \"Sciences Po\" --contact-ids - --dry-run\n", p, p)
	fmt.Fprintf(w, "  %s update_contact 42 --set company=BCG --set title=Consultant\n", p)
	fmt.Fprintf(w, "  %s tasks_list --filter \"done = false\" --per-page 20\n", p)
	return nil
}

func printTools(w io.Writer, title string, tools []*toolInfo, catalog bool) {
	var rows []*toolInfo
	width := 0
	for _, t := range tools {
		if t.Catalog == catalog {
			rows = append(rows, t)
			width = max(width, len(t.Name))
		}
	}
	if len(rows) == 0 {
		return
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Name < rows[j].Name })
	fmt.Fprintln(w, title)
	for _, t := range rows {
		short, _, _ := strings.Cut(t.Description, ". ")
		fmt.Fprintf(w, "  %-*s  %s\n", width, t.Name, strings.TrimSuffix(short, "."))
	}
	fmt.Fprintln(w)
}
