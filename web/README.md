# TaskExec web interface

Vue 2 and Vuetify frontend for TaskExec.

```sh
npm ci
npm run serve
npm run test:unit
npm run lint -- --no-fix
npm run build
```

Development proxies `/api` to port 3000. Production output is written to `../api/public`
and embedded in the Go binary. API documentation is synchronized before builds.
[Development guide](../docs/en/development.md) · [Русский](../docs/ru/development.md).
