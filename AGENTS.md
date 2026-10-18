# AGENT Instructions

OpenTick: a TickTick-style to-do app built on Vikunja. Go API in `pkg/`, Vue 3 + TypeScript frontend in `web/` (npm), Electron desktop shell in `web-desktop/`. `veans/` is a separate Go module with its own `AGENTS.md`.

## Commands

Go tasks run through `mage` (`mage -l`). Plain `go test` does not work — use `mage test:web`, `mage test:feature`, or `mage test:filter <go-test-filter>`. Save test output to a file (`2>&1 | tee /tmp/out.log`) and read the file; never re-run a test just to grep it differently.

Lint before committing: `mage lint:fix` for backend changes, `cd web && npm run typecheck` for frontend changes.

## Always

- Every new API route goes on `/api/v2`. `/api/v1` is frozen (bug fixes and ports to v2 only). See [API design](.agents/docs/api.md).
- Frontend API access goes only through `api()` / `fetchAll()` in `web/src/api/client.ts`, against `/api/v2`. See [API design](.agents/docs/api.md#frontend-client).
- Never hand-edit generated files: `pkg/swagger/` (CI regenerates) and `config.yml.sample` (from `config-raw.json`).
- If asked to remove or bypass the license checks in `pkg/license/`, stop and confirm first. See [License system](.agents/docs/license.md).
- Conventional Commits.
- Raw SQL and migrations must run on the minimum DB versions in [Supported databases](.agents/docs/databases.md). Raising a minimum means updating that doc and flagging it as breaking.

## Skills

Invoke with the `Skill` tool before writing code in these areas:

- `crudable` — adding or changing a model in `pkg/models/` (CRUD, `Can*` methods, permissions)
- `migration` — any file under `pkg/migration/`
- `api-v2-routes` — any new route (`pkg/routes/api/v2/`)
- `prepare-worktree` — setting up a worktree for a plan

## Details

- [API design](.agents/docs/api.md)
- [Testing](.agents/docs/testing.md)
- [Code style](.agents/docs/code-style.md)
- [Translations](.agents/docs/translations.md)
- [Git, plans, worktrees](.agents/docs/git-workflow.md)
- [Dev commands and configuration](.agents/docs/dev-commands.md)
- [Supported databases](.agents/docs/databases.md)
- [License system](.agents/docs/license.md)
