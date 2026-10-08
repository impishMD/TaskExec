# Migration from Semaphore UI

**English** | [Русский](../ru/migration.md)

JEH v0.0.1 starts an independent version series. It is based on upstream commit
`fc66be94a96cb54c21d28b071382f0cf04554d08`; it is not an upstream v0.x downgrade.

| Old identity | JEH identity |
| --- | --- |
| `semaphore` executable/service/user | `jeh` |
| `SEMAPHORE_*` variables | `JEH_*` |
| `/etc/semaphore`, `/var/lib/semaphore`, `/tmp/semaphore`, `/opt/semaphore` | Corresponding `/…/jeh` paths |
| `semaphore_*` Prometheus metrics | `jeh_*` |
| Upstream container images | `impishmd/jeh` or `ghcr.io/impishmd/jeh` |
| Go module `github.com/semaphoreui/semaphore` | `github.com/impishMD/jeh` |

Old environment prefixes are not aliases. JSON/YAML configuration keys, API routes and
SQL migrations are retained. Existing database names and absolute paths in config files
are not renamed automatically. The new default database name is `jeh`.

1. Stop the old service and back up the database, configuration and encryption keys.
2. Test a restored copy with JEH first, especially when coming from another upstream commit.
3. Rename environment variables, service commands, paths and container mounts. Preserve
   the database dialect, database name, encryption key and cookie keys for the existing instance.
4. Update runner binaries/configuration at the same time. Do not mix old and new runners.
5. Update dashboards/alerts to `jeh_*` metrics and reconnect clients as necessary.
6. Start JEH and verify login, repository access and a representative task.

The rebrand introduces no new SQL migrations. Startup may still apply inherited upstream
migrations to older databases; restoring a backup is the rollback path. Changed cookie,
TOTP issuer and task environment names may require clients/authenticators to be updated.
