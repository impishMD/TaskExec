# Development

**English** | [Русский](../ru/development.md)

Requirements: Go from `go.mod`, Node.js 24, npm, Make; Docker for container checks.

```sh
make deps
make build
make test
make check
bash tools/test-binary.sh ./bin/jeh v$(cat VERSION)
make container
bash tools/test-container.sh impishmd/jeh:v$(cat VERSION) v$(cat VERSION) server
```

`web/` contains Vue 2/Vuetify UI; `api/public/` is generated and embedded in the Go binary.
`cli/`, `api/`, `services/`, `db/` and `util/` contain the backend. `pro/` holds open-source
interface stubs. The module path is `github.com/impishMD/jeh`.

Run `npm --prefix web run serve` for frontend development with an API server on port 3000.
`make docs` regenerates configuration and CLI references. Commit generated references with
source changes; `make docs-check` checks synchronization and local Markdown links.
Do not commit databases, secrets, `node_modules`, `bin` or `dist`.
CI runs Go race tests, frontend tests/lint, documentation checks, and native container
smoke tests for both Linux architectures. [Release process](releasing.md).
