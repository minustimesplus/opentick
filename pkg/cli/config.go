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
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	envServer = "OPENTICK_URL"
	envToken  = "OPENTICK_TOKEN"
	envConfig = "OPENTICK_CONFIG"
	mcpPath   = "/api/v2/mcp"
)

// fileConfig is what login stores. The token is an API token with mcp:access.
type fileConfig struct {
	Server string `json:"server,omitempty"`
	Token  string `json:"token,omitempty"`
}

// settings are the effective server and token, with where each came from.
type settings struct {
	file         string
	server       string
	serverSource string
	token        string
	tokenSource  string
}

func defaultConfigPath(getenv func(string) string) string {
	if p := getenv(envConfig); p != "" {
		return p
	}
	if dir := getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "opentick", "cli.json")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "opentick-cli.json")
	}
	return filepath.Join(dir, "opentick", "cli.json")
}

func readConfig(path string) (fileConfig, error) {
	var cfg fileConfig
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("%s is not valid JSON: %w", path, err)
	}
	return cfg, nil
}

func writeConfig(path string, cfg fileConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// The file holds a bearer token.
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

// resolve layers flags over the environment over the config file.
func resolve(g globalFlags, getenv func(string) string) (settings, error) {
	st := settings{file: g.config}
	if st.file == "" {
		st.file = defaultConfigPath(getenv)
	}
	cfg, err := readConfig(st.file)
	if err != nil {
		return st, err
	}
	pick := func(flag, env, file string) (string, string) {
		switch {
		case flag != "":
			return flag, "flag"
		case env != "":
			return env, "env"
		case file != "":
			return file, "file"
		}
		return "", ""
	}
	st.server, st.serverSource = pick(g.server, getenv(envServer), cfg.Server)
	st.token, st.tokenSource = pick(g.token, getenv(envToken), cfg.Token)
	return st, nil
}

// endpointFor accepts the server's base URL or the full MCP endpoint, as shown in the settings.
func endpointFor(server string) (string, error) {
	s := strings.TrimSpace(server)
	if s == "" {
		return "", fmt.Errorf("no server configured: run `login --server https://tasks.example.com` or set %s", envServer)
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	s = strings.TrimRight(s, "/")
	if strings.HasSuffix(s, mcpPath) {
		return s, nil
	}
	return s + mcpPath, nil
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return strings.Repeat("*", len(token))
	}
	return token[:3] + "…" + token[len(token)-4:]
}
