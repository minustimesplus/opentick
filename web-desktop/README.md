# OpenTick desktop

Electron shell around the frontend in `../web`. The bundled frontend is served
from a loopback server that proxies `/api/*` to the Vikunja server entered on
the login screen (stored in `~/.config/OpenTick/config.json`).

## Releases and updates

`.github/workflows/desktop-release.yml` publishes a GitHub release on every
push to `main` with `.deb` + AppImage (Linux x64/arm64), an NSIS `.exe`
(Windows x64) and `.dmg` + `.zip` (macOS Intel/Apple Silicon), plus the
`latest*.yml` update feeds. The app checks for new releases on start and every
4 hours, downloads them in the background, and shows an update button in the
nav rail. AppImages replace themselves; `.deb` installs ask for your password
via `pkexec`.

Builds are unsigned. Windows SmartScreen and macOS Gatekeeper warn on first
launch, and macOS auto-updates won't install until the app is signed with an
Apple Developer ID.

While the repository is private, the updater needs a GitHub token with read
access to its contents in `~/.config/OpenTick/update-token`.

## Development

```sh
npm ci
npm run build:web   # builds ../web into ./app
npm start
npm run dist        # .deb + AppImage in ./dist
```
