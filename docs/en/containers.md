# Containers and runners

**English** | [Русский](../ru/containers.md)

## Images

| Role | Version tag | Stable alias | Entry point |
| --- | --- | --- | --- |
| Server | `v0.0.1` | `latest` | Server setup and startup |
| Runner | `v0.0.1-runner` | `latest-runner` | Remote runner |
| Job | `v0.0.1-job` | `latest-job` | Execution tools, `sh` |
| Helper | `v0.0.1-helper` | `latest-helper` | Git/SSH helper, `sh` |

Every tag is available at `impishmd/jeh` and `ghcr.io/impishmd/jeh` for
`linux/amd64` and `linux/arm64`. OCI labels link GHCR packages to the repository.
Server and runner use UID 1001; mounted directories must be writable by that UID.

Use [compose.yaml](../../compose.yaml) with [.env.example](../../.env.example).
The default SQLite volumes are `/etc/jeh` (configuration) and `/var/lib/jeh` (data).
Do not remove volumes when upgrading. The initial admin environment variables create
an account only on first setup. Change an existing password with `jeh user change-by-login`.
For PostgreSQL/MySQL, configure `JEH_DB_DIALECT`, `JEH_DB_HOST`, `JEH_DB_USER`,
`JEH_DB_PASS` and `JEH_DB`. Secret files are supported by variables such as
`JEH_DB_PASS_FILE` and `JEH_ADMIN_PASSWORD_FILE`.

## Remote runner

Create a runner registration token in the administrator interface, enable remote runners
on the server (`JEH_USE_REMOTE_RUNNER=true`), then start:

```sh
docker run -d --name jeh-runner --restart unless-stopped \
  -e JEH_WEB_ROOT=https://jeh.example.com \
  -e JEH_RUNNER_REGISTRATION_TOKEN="$RUNNER_REGISTRATION_TOKEN" \
  -v jeh-runner-data:/var/lib/jeh \
  impishmd/jeh:v0.0.1-runner
```

Keep the runner data volume: it stores the registered runner token.
Job/helper images are provided for compatible executor implementations; their publication
does not enable proprietary Docker/Kubernetes executors in this open-source build.
