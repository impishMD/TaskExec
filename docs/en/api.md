# HTTP API

**English** | [Русский](../ru/api.md)

The REST API is served under `/api`. Interactive Swagger documentation is available
at `/swagger/` on your TaskExec instance. [api-docs.yml](../../api-docs.yml) is bundled
into every frontend build.

## Authentication

Create personal API tokens in your user profile. They act with the user's permissions.
Session authentication uses `POST /api/auth/login` with JSON
`{"auth":"username","password":"password"}` and the returned cookie.

```sh
curl http://localhost:3000/api/ping
curl -H "Authorization: Bearer $TASKEXEC_API_TOKEN" http://localhost:3000/api/projects
```

Project tokens are independent credentials for automation within a single project.
Create them under **Dashboard → Settings → API tokens**. Server administrators and
project owners can list, create, rotate and revoke them. Creator details are audit
snapshots: deleting, blocking or removing that user from the project does not disable
the token. The token remains valid until expiry or manual revocation; deleting the
project removes its tokens.

The secret is displayed only when created or rotated. The database stores its hash,
and subsequent API responses contain only token metadata. Keep tokens in your CI
secret store; do not commit them or include them in bug reports.

## Project token permissions

Project tokens use explicit permissions, independently of user roles. Select specific
templates, or explicitly allow all templates in the project.

| Permission | Access |
| --- | --- |
| `templates:read` | Allowed template names, types and survey field definitions |
| `tasks:run` | Start allowed templates |
| `tasks:read` | Status and basic metadata of runs of allowed templates |
| `tasks:logs` | Logs of runs of allowed templates; enable separately because logs can contain sensitive data |
| `tasks:stop` | Stop only runs started with this token |

The CI preset includes template discovery, running tasks and reading their status.
Log access is off by default. The default expiry in the form is 90 days; 30 days,
one year and no expiry are also available.

A token cannot manage project settings, credentials, variable groups, repositories,
users, permissions or other tokens. It cannot export backups, access other projects,
use WebSockets, start workflows or approve tasks. It is not a project administrator.

By default, a start request can supply only the selected template, a message and
values for survey fields declared by that template. Additional permissions can allow
overrides of `git_branch`, `playbook`, `inventory_id`, `arguments`, `environment` and
`params`. These can substantially change what a task executes. Enable only the
fields needed by the integration. Template restrictions still apply, and inventory
IDs must belong to the same project. `environment` and `secret` are JSON objects
encoded as strings; without the environment override, both accept only declared
survey fields, with type, choice and required-field validation. Omitted survey fields
use saved defaults. Fields declared as secret are stored through the task secret
mechanism even if submitted in `environment`.

## Calling the project API

Project tokens use the normal Bearer header, with these allowed paths under
`/api/project/{project_id}`:

| Method | Path | Permission |
| --- | --- | --- |
| GET | `/templates`, `/templates/{id}` | `templates:read` |
| POST | `/tasks` | `tasks:run` |
| GET | `/tasks`, `/tasks/last`, `/templates/{id}/tasks`, `/tasks/{id}` | `tasks:read` |
| GET | `/tasks/{id}/output` | `tasks:logs` |
| POST | `/tasks/{id}/stop` | `tasks:stop` |

Task lists return at most 100 rows, newest first; use `?before={last_id}` for the next
page. Task responses contain only ID, project/template IDs, status, timestamps,
version and the source token ID/name. Template responses omit credentials, variable
values and survey defaults. Supply exactly one of `template_id` and `template_name`
when starting a task. Unknown request fields are rejected. Stop requests require
JSON `{}` or `{"force":true}`.

```sh
curl -H "Authorization: Bearer $TASKEXEC_PROJECT_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"template_id":42,"message":"CI deployment"}' \
  https://taskexec.example.com/api/project/1/tasks
```

Token management endpoints require a session or a personal token belonging to a
server administrator or project owner:

- `GET /api/project/{project_id}/tokens` — list metadata, including expiry and last use.
- `POST /api/project/{project_id}/tokens` — issue a token with `name`, `scopes`,
  `template_ids` (or `all_templates: true`), optional `overrides` and optional RFC 3339 `expires_at`.
- `DELETE /api/project/{project_id}/tokens/{token_id}` — revoke a token; history remains.
- `POST /api/project/{project_id}/tokens/{token_id}/rotate` — issue a replacement with
  the same request fields as creation, atomically revoking the previous token.

After rotation, the old token stops working immediately. Queued runs are checked
again before execution and do not start with a revoked or expired token. Running
tasks continue. The replacement has its own ID, so it cannot stop runs started by
the previous token. Use a user account to stop those runs if necessary.

Creation, rotation, revocation and task actions are recorded in the audit log.
Task history identifies the token that launched the run.
