# Configuration

**English** | [Русский](../ru/configuration.md)

JEH reads JSON/YAML configuration and `JEH_*` environment variables. Environment
variables take precedence. Use `jeh setup` to generate a configuration file,
then `jeh server --config /path/to/config.json`. `--no-config` enables environment-only operation.

| Setting | Purpose |
| --- | --- |
| `JEH_DB_DIALECT` | `sqlite`, `postgres` or `mysql` |
| `JEH_DB_HOST` | SQLite file or database host |
| `JEH_PORT` | HTTP port, default 3000 |
| `JEH_WEB_ROOT` | Public URL, including any path prefix |
| `JEH_TMP_PATH` | Working directory, default `/tmp/jeh` |
| `JEH_ACCESS_KEY_ENCRYPTION` | Base64 encryption key for stored credentials |
| `JEH_METRICS_ENABLED` | Enable Prometheus metrics |

See the [complete generated reference](reference/configuration.md),
[CLI commands](reference/cli.md) and [schema](../../config.schema.yaml).
Keep the encryption key with backups; replacing it prevents decrypting existing secrets.
Set the public URL when using a reverse proxy. Authentication settings include local
accounts, LDAP and OpenID Connect; configure providers using the reference.

## Backups

Back up the configuration, encryption keys and database together. Stop a SQLite instance
before copying its database, or use a consistent SQLite backup. For PostgreSQL/MySQL,
use the database's native backup tooling. Project JSON export is useful for transferring
project definitions but does not replace a complete instance backup.

## Feature scope

JEH v0.0.1 builds the open-source implementation present in this repository.
The `pro` package contains interface stubs, not upstream proprietary implementations.
Options marked **upstream Pro/Enterprise** in the generated reference are retained for
source compatibility and do not enable those features. Workflows, active-active HA,
custom roles and Docker/Kubernetes executors are among the gated features. No subscription
is sold or activated through JEH; no private upstream module is fetched during builds.
