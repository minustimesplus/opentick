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

package cmd

import (
	"context"
	"os"

	"code.vikunja.io/api/pkg/cli"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(cliCmd)
}

var cliCmd = &cobra.Command{
	Use:   "cli",
	Short: "Run a server's MCP tools from the command line (for scripts and AI agents)",
	Long: `A client for the MCP tools of an OpenTick server, local or remote. It talks to
the server's /api/v2/mcp endpoint with an API token, so it offers exactly the
tools the token may use. Run "opentick cli help" for the commands and tools.
The standalone build of the same program is cmd/otk.`,
	// Tool flags are only known once the server lists its tools.
	DisableFlagParsing: true,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		os.Exit(cli.Run(ctx, "opentick cli", args, cli.StdIO()))
	},
}
