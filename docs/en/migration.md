# Migration to TaskExec

**English** | [Русский](../ru/migration.md)

## Existing JEH installation

Use the following mapping when moving an existing JEH installation to TaskExec.
Preserve the database and encryption keys.

| Previous identity | TaskExec identity |
| --- | --- |
| `jeh` executable, service and system user | `taskexec` |
| `JEH_*` environment variables | `TASKEXEC_*` |
| Compose project/service `jeh` | `taskexec` |
| Compose volumes `jeh_jeh-config`, `jeh_jeh-data` | `taskexec_taskexec-config`, `taskexec_taskexec-data` |
| `/etc/jeh`, `/var/lib/jeh`, `/tmp/jeh`, `/opt/jeh`, `/home/jeh` | Corresponding `/…/taskexec` paths |
| Ansible `jeh_vars` and `JEH_*` task environment | `taskexec_vars` and `TASKEXEC_*` |
| `jeh_*` Prometheus metrics | `taskexec_*` |
| Go module `github.com/impishMD/jeh` | `github.com/impishMD/taskexec` |
| Image `impishmd/jeh` | Local `taskexec:dev`; release images `impishmd/taskexec` / `ghcr.io/impishmd/taskexec` |

1. Build the new image with `docker compose -f compose.yaml -f compose.dev.yaml build taskexec`.
2. Stop the old service. Back up its database/data volume, configuration volume and `.env`.
   Keep a consistent SQLite backup while the service is stopped.
3. Rename `.env` keys to `TASKEXEC_*`, preserving their values, especially
   `TASKEXEC_ACCESS_KEY_ENCRYPTION`. Preserve database credentials and the existing database
   name; do not create a fresh database just because the new default name is `taskexec`.
4. Copy both volumes to the new names, preserving ownership (container UID 1001).
   Do not run `down -v` or remove the old volumes. Starting with empty new volumes creates
   a separate installation and does not migrate existing accounts or projects.
5. In the copied configuration, update absolute paths under the old `/…/jeh` directories
   to their new `/…/taskexec` mounts. Preserve cookie keys, encryption keys and other
   configuration values. For SQLite, point `sqlite.host` at the copied database file.
6. Start `docker compose -f compose.yaml -f compose.dev.yaml up -d --no-build`.
   Sign in again: the browser authentication cookie is named `taskexec`.
7. Verify users/projects, credentials, alert settings and a representative task. Update
   runners, task scripts/Ansible variables and Prometheus queries to the new names.

Keep the original volumes and backup until verification is complete. To roll back, stop
the new service and restore the previous image, environment and original volumes together.
Do not start both installations on the same writable database.

Old environment prefixes are not aliases. Configure database paths explicitly.


## Migration from Semaphore UI

| Old identity | TaskExec identity |
| --- | --- |
| `semaphore` executable/service/user | `taskexec` |
| `SEMAPHORE_*` variables | `TASKEXEC_*` |
| Ansible `semaphore_vars` | `taskexec_vars` |
| `/etc/semaphore`, `/var/lib/semaphore`, `/tmp/semaphore`, `/opt/semaphore` | Corresponding `/…/taskexec` paths |
| `semaphore_*` Prometheus metrics | `taskexec_*` |
| Upstream container images | `impishmd/taskexec` or `ghcr.io/impishmd/taskexec` |
| Go module `github.com/semaphoreui/semaphore` | `github.com/impishMD/taskexec` |

Old environment prefixes are not aliases.  Existing database names and absolute paths in config files
are not renamed automatically. The new default database name is `taskexec`.

1. Stop the old service and back up the database, configuration and encryption keys.
2. Test a restored copy with TaskExec first, especially when coming from another upstream commit.
3. Rename environment variables, service commands, paths and container mounts. Preserve
   the database dialect, database name, encryption key and cookie keys for the existing instance.
4. Update runner binaries/configuration at the same time. Do not mix old and new runners.
5. Update dashboards/alerts to `taskexec_*` metrics and reconnect clients as necessary.
6. Start TaskExec and verify login, repository access and a representative task.

TaskExec applies database migrations at startup. To roll back, restore the database backup
and matching application version together. Verify client authentication and task environment settings.
