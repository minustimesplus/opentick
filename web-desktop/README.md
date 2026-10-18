# Vikunja TickTick desktop

Electron shell around the frontend in `../web`. The bundled frontend is served
from a loopback server that proxies `/api/*` to the Vikunja server entered on
the login screen (stored in `~/.config/Vikunja TickTick/config.json`).

## Releases and updates

`.github/workflows/desktop-release.yml` publishes a GitHub release with a
`.deb`, an AppImage and `latest-linux.yml` on every push to `main`. The app
checks for new releases on start and every 4 hours, downloads them in the
background, and shows an update button in the nav rail. AppImages replace
themselves; `.deb` installs ask for your password via `pkexec`.

While the repository is private, the updater needs a GitHub token with read
access to its contents in `~/.config/Vikunja TickTick/update-token`.

## Development

```sh
npm ci
npm run build:web   # builds ../web into ./app
npm start
npm run dist        # .deb + AppImage in ./dist
```
