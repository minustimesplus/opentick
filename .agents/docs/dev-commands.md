# Dev commands and configuration

- `mage -l` lists targets; `npm run` in `web/` lists scripts.
- `npm run dev` in `web/` serves the frontend on port 5180 and proxies `/api` to `http://localhost:3456` (override with `VIKUNJA_API`).
- `npm run build` in `web/` writes `web/dist`, which `web/embed.go` embeds into the Go binary.
- Scaffolding: `mage dev:make-migration <StructName>` (prompts if omitted), `dev:make-event`, `dev:make-listener`, `dev:make-notification`. `make-listener` also adds the `events.RegisterListener(...)` call; a listener only fires if registered there.
- Invoke the `migration` skill before touching `pkg/migration/`.

## Configuration

`config.yml.sample` is generated from `config-raw.json` via `mage generate:config-yaml`. Edit the JSON, then regenerate. Environment variables override config file settings.
