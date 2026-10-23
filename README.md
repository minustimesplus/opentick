# OpenTick

> A clean, open source implementation of TickTick, built on [Vikunja](https://vikunja.io).

[![Features implemented](https://img.shields.io/badge/TickTick_features-57%25-yellow)](FEATURES.md)

See [FEATURES.md](FEATURES.md) for what is and isn't implemented yet.

OpenTick pairs a TickTick-style web frontend (lists, calendar, Eisenhower matrix,
subscribed ICS calendars) with the Vikunja Go API as its backend. You host it
yourself, and your data stays with you.

> [!NOTE]
> OpenTick is developed with LLM-assisted coding tools in various parts of the codebase.

## Table of contents

- [Installing](#installing)
	- [Docker](#docker)
	- [Desktop app (Linux)](#desktop-app-linux)
- [Building from source](#building-from-source)
- [Development setup](#development-setup)
- [Contributing](#contributing)
- [License](#license)
	- [Unsplash Images](#unsplash-images)

## Installing

### Docker

Build the image from the repository and run it:

```sh
git clone https://github.com/minustimesplus/vikunja-ticktick.git opentick
cd opentick
docker build -t opentick .
docker run -d --name opentick \
  -p 3456:3456 \
  -v opentick-db:/db \
  -v opentick-files:/app/vikunja/files \
  opentick
```

OpenTick is then available at <http://localhost:3456>. The image uses SQLite by
default; PostgreSQL and MySQL are supported too. Configure the server with
`VIKUNJA_*` environment variables or a `config.yml` (see
[`config.yml.sample`](config.yml.sample) for every option).

### Hosted

The hosted instance runs on Heroku with Neon, S3 and Clerk sign in. See
[docs/hosted.md](docs/hosted.md).

### Desktop app (Linux)

Every push to `main` publishes a GitHub release with a `.deb` and an AppImage.
Download one from the [releases page](https://github.com/minustimesplus/vikunja-ticktick/releases),
install it, and enter the URL of your OpenTick server on the login screen.
The app updates itself in the background.

## Building from source

Requirements:

- Go (version in [`go.mod`](go.mod))
- Node.js 24 and npm
- [Mage](https://magefile.org/)

```sh
# 1. Build the frontend into web/dist, which the Go binary embeds
cd web
npm ci
npm run build
cd ..

# 2. Build the server binary
mage build

# 3. Run it
./vikunja
```

The resulting binary contains both the API and the frontend and serves them on
port 3456.

To build the desktop app instead, see [`web-desktop/README.md`](web-desktop/README.md):

```sh
cd web-desktop
npm ci
npm run build:web   # builds ../web into ./app
npm run dist        # .deb + AppImage in ./dist
```

## Development setup

The quickest way to get every tool (Go, Node.js, Mage, linters) is
[devenv](https://devenv.sh/) (`devenv shell`) or [mise](https://mise.jdx.dev/)
(`mise install`). Otherwise install the requirements from
[Building from source](#building-from-source) manually.

Run the backend and frontend in two terminals:

```sh
# Terminal 1: API on http://localhost:3456
mage build && ./vikunja

# Terminal 2: frontend dev server on http://localhost:5180
cd web
npm ci
npm run dev
```

The dev server proxies `/api` to `http://localhost:3456`. Point it at another
backend with `VIKUNJA_API=https://your-server npm run dev`.

Useful commands:

```sh
mage -l                      # list all backend tasks
mage test:feature            # backend feature tests
mage test:web                # backend web/API tests
mage test:filter "TestName"  # a single backend test
mage lint:fix                # lint the backend
cd web && npm run typecheck  # type-check the frontend
```

Plain `go test` does not work; always go through `mage test:*`.

## Contributing

Contributions go through [GitHub issues](https://github.com/minustimesplus/vikunja-ticktick/issues).
Open an issue for bug reports, feature ideas, or before starting on a change so
we can agree on it first.

## License

OpenTick is licensed under [AGPL‑3.0‑or‑later](LICENSE), the same license as
Vikunja, which it is built on. You can use, modify, and self-host it freely; if
you distribute it or run a modified version as a network service, you must
make your source code available under the same license.

### Unsplash Images

Background images from Unsplash are distributed under the [Unsplash License](https://unsplash.com/license). The license requires giving credit to the photographer and Unsplash. See [Unsplash’s terms](https://unsplash.com/terms) for more information.
