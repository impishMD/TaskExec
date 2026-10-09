# Configuration

**English** | [Русский](../ru/configuration.md)

TaskExec reads JSON/YAML configuration and `TASKEXEC_*` environment variables. Environment
variables take precedence. Use `taskexec setup` to generate a configuration file,
then `taskexec server --config /path/to/config.json`. `--no-config` enables environment-only operation.

| Setting | Purpose |
| --- | --- |
| `TASKEXEC_DB_DIALECT` | `sqlite`, `postgres` or `mysql` |
| `TASKEXEC_DB_HOST` | SQLite file or database host |
| `TASKEXEC_PORT` | HTTP port, default 3000 |
| `TASKEXEC_WEB_ROOT` | Public URL, including any path prefix |
| `TASKEXEC_TMP_PATH` | Working directory, default `/tmp/taskexec` |
| `TASKEXEC_ACCESS_KEY_ENCRYPTION` | Base64 encryption key for stored credentials |
| `TASKEXEC_METRICS_ENABLED` | Enable Prometheus metrics |

See the [complete generated reference](reference/configuration.md),
[CLI commands](reference/cli.md) and [schema](../../config.schema.yaml).
Keep the encryption key with backups; replacing it prevents decrypting existing secrets.
Set the public URL when using a reverse proxy. Authentication settings include local
accounts, LDAP and OpenID Connect; configure providers using the reference.

## General server settings

Administrators can open **Management (gear) → General settings** and enable
**Use remote runners by default**. This server-wide setting is stored in the database
and survives container recreation. It defaults to off: untagged tasks run on the server.
When enabled, untagged tasks use active remote runners marked as default; matching
project runners take priority over global runners. An explicit runner tag in a template
or inventory always selects a remote runner, regardless of this setting.

Saving applies to newly created tasks without restarting the server. Tasks already
queued in the running server keep their execution mode, and assigned tasks remain on
their runners. If the server recovers an unassigned task from the database after a restart,
it uses the current default. If matching runners are offline or busy, the task waits.
If no enabled runner matches, dispatch fails. There is no automatic fallback to local execution.

Remote execution is configured through the UI. The setting is included in full database
backups, not project JSON exports.
Automation can use the administrator-only `GET /api/settings` and `PUT /api/settings`
endpoints with `{"use_remote_runner": true}` or `{"use_remote_runner": false}`.

## Telegram alerts

Open **Management (gear) → Alerts → Telegram** to save the default bot token
issued by BotFather. Global settings are available to administrators.

In **Project → Dashboard → Settings**, open **Configure alerts → Telegram**.
Enable Telegram, enter the destination Chat ID and save. Telegram starts disabled
for every project. The bot needs permission to send messages to that chat.
Template options that suppress success or failure alerts still apply.

**Use the global bot token** is selected by default. Clear it to save a separate
bot token for this project; select it again to remove the override. Saved tokens
are never returned by the API or displayed. An empty replacement field keeps an
existing token. Use **Remove token** to delete the global token explicitly.
Project alert settings require permission to update the project.

**Test notification** sends a message using the current form values without saving them.
In global settings, enter a separate test Chat ID; it is not stored. In project settings,
the test uses the selected global or project bot and the destination shown in the form.
The result appears below the fields. Sending a test does not run a task.

Channel settings are stored separately in the database, with per-project enablement
and destinations. Tokens use the server's access-key encryption configuration;
keep this key with your database backups. `vault check`, `vault rekey`, and rekey
credential backups include alert tokens. Changes apply without restarting the server.
Project JSON exports do not include these credentials or channel settings.

## Backups

Back up the configuration, encryption keys and database together. Stop a SQLite instance
before copying its database, or use a consistent SQLite backup. For PostgreSQL/MySQL,
use the database's native backup tooling. Project JSON export is useful for transferring
project definitions but does not replace a complete instance backup.

## Runners and access permissions

- **Project → Runners** manages runners belonging to that project. Creation, editing,
  deletion, cache clearing and registration token rotation require **Manage project
  resources**. Members can read the list; runner credentials are not included in it.
  To dispatch tasks remotely, enable **Use remote runners by default** in **General settings** or select a runner
  tag in the template/inventory. Matching project runners take priority over global
  runners; tasks without a tag use runners marked as default.
- **Management → Roles** manages global roles and requires administrator access.
  **Project → Team → Roles** manages roles belonging to that project and requires
  **Manage project users** to make changes. Assign a role from the project's team page.
- **Template → Permissions** grants additional template permissions to a custom
  global or project role. Editing these grants requires **Manage project users**.
  Grants add to the role's existing permissions; they do not revoke them.
- A role's slug is immutable and unique across the instance. Built-in slugs are reserved.
  Remove a role's user assignments and template grants before deleting it.

## Task integrations

- [HashiCorp Vault](secret-storages.md): KV v1/v2 secret references and credential mapping.
- [Terraform/OpenTofu HTTP backend](terraform-state.md): states, history and locks stored in the database.
- [Workflows](workflows.md): task graphs, approvals and execution conditions.
- [Docker execution](containers.md#docker-task-executor): task isolation in containers.

## Ansible summaries

Ansible tasks store host recap counters, task errors and execution phases in the
main database. Open a completed task and select **Summary**. This works for local
execution and remote runners. Host counters come from `PLAY RECAP`; ignored/rescued
errors remain in the error list even when the final host status is successful.

The parser supports Ansible's default text callback, including colored output and
multiline JSON/YAML failure details. It preserves Ansible's `no_log` censorship.
Custom stdout callbacks may not produce a compatible recap. Runs ending before a
recap show an empty-state explanation. Error previews are
limited to 1,000 characters; the full emitted output remains in the task log.

Deleting a task deletes its summary. Instance database backups include summaries and task logs;
project JSON exports do not include execution history.

## Deployment limits

Run one TaskExec server with local, remote or Docker task execution. Authentication supports
local accounts, LDAP, OpenID Connect and TOTP. Audit events are stored in the database.
