# Development

**English** | [Русский](../ru/development.md)

## Local preview

Once `.env` is configured using the [quick start](../../README.md#quick-start), run:

```sh
docker compose -f compose.yaml -f compose.dev.yaml up -d --build
```

Open <http://localhost:3000>. The `taskexec:dev` image builds from your working tree and
reuses the TaskExec Compose database/configuration volumes. Repeat after changes.
For a JEH installation, [migrate its volumes first](migration.md#existing-jeh-installation).
This command does not publish images or create releases.

For frontend hot reload with the container API on port 3000:

```sh
npm --prefix web ci
npm --prefix web run serve -- --host 127.0.0.1 --port 8080
```

Open <http://localhost:8080>.

## Build and checks

Requirements: Go from `go.mod`, Node.js 24, npm, Make; Docker for container checks.

```sh
make deps
make build
make test
make check
bash tools/test-binary.sh ./bin/taskexec v$(cat VERSION)
make container
bash tools/test-container.sh impishmd/taskexec:v$(cat VERSION) v$(cat VERSION) server
```

`web/` contains Vue 2/Vuetify UI; `api/public/` is generated and embedded in the Go binary.
`cli/`, `api/`, `services/`, `db/` and `util/` contain the backend. The Go module path is `github.com/impishMD/taskexec`.

Run `npm --prefix web run serve` for frontend development with an API server on port 3000.
`make docs` regenerates configuration and CLI references. Commit generated references with
source changes; `make docs-check` checks synchronization and local Markdown links.
Do not commit databases, secrets, `node_modules`, `bin` or `dist`.
CI runs Go race tests, frontend tests/lint, documentation checks, and native container
smoke tests for both Linux architectures. [Release process](releasing.md).

## Context help in the UI

Use the shared help pattern: a `?` button in the header reveals icons beside
parameters; clicking an icon opens its explanation. In `EditDialog`, enable
`help-button` and place `HelpHint` beside the relevant parameter. For standalone
pages, use the `HelpContext` mixin and `HelpToggle` component. Help state belongs
to that page or dialog and resets when the dialog closes.

Only show the toggle when help content exists. Agree on new text and placements
separately; avoid explanations that merely repeat obvious field behavior.

## Project icons

Project settings support built-in symbols and PNG, JPEG or WebP uploads up to
5 MiB. Removing the icon restores initials. The browser resizes images to
128×128 (96×96 or 64×64 when needed) and stores a PNG data URL in `project.icon`.
Built-in symbols store their `mdi-*` name. Project backups include the icon;
no separate file storage is needed.
