# AI assistants: MCP server and command line

OpenTick exposes your tasks, projects, labels and people (contacts) to AI
assistants through an MCP server, and to scripts through a command line client
(`otk`) that offers exactly the same tools.

- MCP endpoint: `https://<your server>/api/v2/mcp` (streamable HTTP). The
  settings page (Settings → AI Features) shows the exact URL.
- Authentication: an API token with the `mcp:access` permission, created under
  Settings → AI Features, sent as `Authorization: Bearer tk_…`. Claude's custom
  connectors and Claude Code can sign in with OAuth instead.

## Server setup

- Set `service.publicurl` (`VIKUNJA_SERVICE_PUBLICURL`) to the address clients
  use, including a non-standard port: `https://box.tailnet.ts.net:8443/`. OAuth
  discovery and the endpoint shown in the settings are built from it.
- Serve OpenTick at the root of that host. OAuth discovery
  (`/.well-known/oauth-protected-resource`, `/.well-known/oauth-authorization-server`)
  is looked up at the host root, so a sub-path deployment cannot sign in
  with OAuth (tokens still work).
- A reverse proxy must pass the `Authorization` header through.

Check the setup from any machine:

```sh
# 401 with a WWW-Authenticate header pointing at the resource metadata
curl -si -X POST https://box.tailnet.ts.net:8443/api/v2/mcp -H 'Content-Type: application/json' -d '{}' | grep -i www-authenticate
curl -s https://box.tailnet.ts.net:8443/.well-known/oauth-protected-resource/api/v2/mcp
```

## Connecting assistants

### Claude Code

With a token:

```sh
claude mcp add --transport http opentick https://box.tailnet.ts.net:8443/api/v2/mcp \
  --header "Authorization: Bearer tk_…"
```

Or without one: `claude mcp add --transport http opentick https://box.tailnet.ts.net:8443/api/v2/mcp`,
then run `/mcp` in a session and choose Authenticate. Your browser opens
OpenTick's consent page; pick what Claude may access and click Allow. Claude
Code runs on your machine, so this works on a private network such as Tailscale.

### Claude desktop app and claude.ai

Add a custom connector (Customize → Connectors → Add custom connector) with the
endpoint URL and leave the OAuth fields empty; Claude opens OpenTick to sign in.
Claude connects from Anthropic's servers, not from your computer, so the server
must be reachable from the internet over HTTPS, for example with Tailscale Funnel
(which allows ports 443, 8443 and 10000). A private Tailscale address does not work
for connectors; use Claude Code instead.

### Codex and other clients

```sh
export OPENTICK_MCP_TOKEN=tk_…
codex mcp add opentick --url https://box.tailnet.ts.net:8443/api/v2/mcp --bearer-token-env-var OPENTICK_MCP_TOKEN
```

Other clients: use the streamable HTTP transport with the endpoint and send
`Authorization: Bearer tk_…`. The server answers with plain JSON (no event
stream) and also accepts clients that only send `Accept: application/json`.

## Tools

Tools are generated from the v2 API, so a token only sees the tools its
permissions allow. Frequent ones are first-class tools; the rest are found with
`find_action` and run with `do_action`.

People (the Personal CRM tools, same names and behaviour):

| Tool | What it does |
|------|--------------|
| `search_contacts` | Structured filters (name, org, school, company, title, location, tag, group, not_group, not_tag, has, missing) and free text, `match` all/any, with the reasons each contact matched. `ids_only` returns every matching id. |
| `get_contact` | Everything about one contact. |
| `distinct_values` | Every spelling of a school, company, location, title, degree, field, tag or group, with counts. |
| `list_groups`, `list_tags` | Groups and tags with member counts. |
| `create_group`, `update_group` | Create (or reuse by name) and change groups. |
| `add_to_group`, `remove_from_group`, `tag_contacts`, `untag_contacts` | Bulk writes. Take `dry_run` and report `changed`, `unchanged` and `unknown_ids`. |
| `create_contact`, `update_contact` | Create (refuses duplicates unless `allow_duplicate`) and edit contacts. Edited fields are locked against LinkedIn imports. |
| `add_note`, `add_experience`, `add_education` | Add history and notes. |

There is deliberately no tool that deletes contacts.

Tasks and projects: `tasks_list`, `tasks_read`, `tasks_create`, `tasks_update`,
`tasks_delete`, `projects_*`, `labels_*`, `task_comments_*`, `task_assignees_*`
and more through `find_action`.

## Command line (`otk`)

`otk` is an MCP client of your server: its commands are the tools your token
may use, read from the server each time, so the command line and the MCP
server never drift apart. It is also built into the server binary as
`opentick cli`.

### Install

```sh
go build -o otk ./cmd/otk     # small standalone binary, no server code
sudo install otk /usr/local/bin/
```

### Configure

```sh
otk login --server https://box.tailnet.ts.net:8443   # asks for the token, checks it, saves both
otk config                                          # shows the server, the masked token and where each comes from
otk config set token tk_…
```

The settings live in `~/.config/opentick/cli.json` (mode 0600). `OPENTICK_URL`,
`OPENTICK_TOKEN` and `OPENTICK_CONFIG` override the file, and `--server`,
`--token` and `--config` before the command override both. The server may be
given as the base URL or the full `/api/v2/mcp` endpoint.

### Use

```sh
otk help                        # commands and every tool the token can use
otk help search_contacts        # a tool's description, positional arguments and JSON schema
otk tools                       # every tool with its schema, as JSON

otk distinct_values school --contains "sciences po"
otk search_contacts --school "sciences po" --location reims --match any
otk search_contacts --location reims --ids-only | otk tag_contacts Reims --contact-ids - --dry-run
otk add_to_group "Sciences Po" --contact-ids 12 --contact-ids 15,16 --create
otk update_contact 42 --set company=BCG --set title=Consultant
otk add_note 42 "Met at the alumni dinner"

otk tasks_list --filter "done = false" --per-page 20
otk tasks_create 3 --title "Write the docs" --due-date 2026-12-01T09:00:00Z
otk call tasks_update '{"task": 7, "done": true}'
echo '{"location": ["reims"]}' | otk call search_contacts -
```

- Flags map to the tool's parameters: `--contact-ids` → `contact_ids`.
  `--flag=value` works too; booleans take no value (`--dry-run`, `--no-create`).
- Array parameters can be repeated; integer arrays also take `1,2,3`.
- `-` reads the value from stdin. For id lists it accepts the output of
  `--ids-only`, a search result, a list response (`{"items": [...]}`), a JSON
  array, or ids separated by newlines, spaces or commas.
- Some tools take positional arguments (`otk get_contact 42`); `help <tool>`
  lists them. Tools without their own use their path parameters in order.
- `--set field=value` sets one parameter, like the Personal CRM's command line.
- Catalog actions (`teams_list`, `project_views_list`, …) are run directly by
  name; `otk` goes through `do_action` for you.

Output is JSON on stdout. Errors are `{"error": "…"}` on stderr with exit code 1,
including the server's message and a hint for 401 (bad or expired token) and
403 (token without `mcp:access`).
