# HashiCorp Vault

[Русский](../ru/secret-storages.md)

TaskExec supports HashiCorp Vault KV v1/v2.
Open **Project → Key Store → Storages** and create a **HashiCorp Vault** connection.

## Connection

- **Server URL**: a URL reachable from the TaskExec server, such as `https://vault.example.com`.
  The server resolves credentials before dispatching work to a local or remote executor.
- **KV mount path**: the mount of the KV secrets engine, default `secret`.
  Enter the storage mount only; the path to a particular secret is configured separately.
  Do not paste the full API path, `/v1`, or the API's `/data` or `/metadata` segments.
- **KV version**: choose the version configured on that mount. The default is **2**;
  the client does not guess the version from failed requests.
- **Namespace**: optional, if your Vault deployment supports namespaces.
- **Read only**: enabled by default. TaskExec can read existing secrets and import references;
  it cannot write or delete provider values through this connection.

TLS certificate validation is always enabled. Under **Advanced**, optionally provide a custom
CA certificate (PEM), or install it in the server trust store. Redirects are rejected so credentials are not forwarded to another endpoint.
Connection errors, missing fields, invalid responses and denied access fail the operation;
TaskExec does not substitute an empty secret or fall back to a local value.

The token needs `read` on the selected paths. Path autocomplete also needs
`list`: for KV v2, on `<mount>/metadata/...`; for KV v1, on `<mount>/...`.
Writable connections additionally need `create`, `update` and `delete` on the data paths.
KV v2 writes use check-and-set (CAS) to detect concurrent changes.

## Example: secret `proxmox` in mount `secret`

If the Vault UI shows **Secrets → secret → proxmox → Paths** with:

```text
API path: /v1/secret/data/proxmox
CLI path: -mount="secret" "proxmox"
```

configure the TaskExec connection as follows:

| TaskExec field | Value | Meaning |
| --- | --- | --- |
| Server URL | `https://vault.example.com` | Your Vault server address; replace the example domain |
| KV mount path | `secret` | The `-mount` value from CLI path |
| KV version | `2` | This example uses KV v2 |

**`/v1/secret/data/proxmox` is incorrect in the KV mount path field.** For this example,
`secret/proxmox` and `secret/data` are also incorrect. The connection describes the entire
`secret` KV mount; select the `proxmox` document when configuring a key.
Mounts may have multiple segments: if CLI path says `-mount="team/kv"`, enter the full
`team/kv`. Do not assume that the mount is always the first segment of an API path.

TaskExec constructs the KV v2 read request itself:

```text
Server URL                   + /v1/ + mount  + /data/ + secret path
https://vault.example.com     + /v1/ + secret + /data/ + proxmox
= https://vault.example.com/v1/secret/data/proxmox
```

This follows the [KV v2 read API](https://developer.hashicorp.com/vault/api-docs/secret/kv/kv-v2#read-secret-version).
KV v1 has no `/data/` segment: the same mount and path yield `/v1/secret/proxmox`.
**KV version** is the secrets engine version, not a secret revision in **Version History**.

### Add a key

1. Save the connection; keep **Read only** enabled when reading an existing secret.
2. Open **Key Store → Keys → New Key** and give it a name, such as `test`.
3. Select the **Storage** source and your connection.
4. Enter `proxmox` in **Secret path**, without `secret/`, `/v1/`, `/data/` or `#field`.
5. Choose **String**, then select `dns_server` from the automatically loaded **Secret field** list.
   This creates `test → proxmox document → dns_server field`. To read the complete document,
   choose **Variable set**; this type needs no field mappings.

The form saves references and mappings only. It does not write to Vault, even when the
connection allows writes. SSH and login/password fields select document fields instead of
accepting credential values. Reference edits do not need **Override**. Field names load automatically
when selecting or changing the path and when opening a saved key. Typed paths are loaded after
a short pause; directories ending in `/` are not read as secrets. Existing mappings remain editable
when Vault is unavailable.

The former `proxmox#dns_server` format is rejected, with no automatic conversion. Configure
`proxmox` as the path and `dns_server` as the field mapping separately.

**Test authentication** validates login, not the existence of `secret/proxmox` or permission
to read it. Login can succeed with an incorrect KV mount path; access to the document is
checked when reading the key.

## Authentication

Select a method in the connection form. Its fields appear below the selector.
Configure the auth method, roles and policies on the Vault server beforehand.
**Authentication mount path** is the path after `auth/`, e.g. `approle` or `services/approle`;
it is independent of the **KV mount path** (`secret` by default).

| Method | Fields |
| --- | --- |
| Token | Existing Vault token |
| AppRole | Role ID and Secret ID; custom auth mount, default `approle` |
| Kubernetes | Vault role and service account JWT; custom auth mount, default `kubernetes` |
| JWT | Vault role and JWT; custom auth mount, default `jwt` |
| Certificate | PEM client certificate and unencrypted PEM private key; optional certificate role name; default mount `cert`; HTTPS required |

Each credential has its own source selector: **Database**, **Environment** or **File**.
Database values and source references are encrypted together using the existing access-key
keyring; retain the keyring with database backups. Saved values are masked. An empty edit
preserves a saved value only for the same method and source. Switching sources requires a
new value/reference; changing methods requires that method's credentials.

Environment variables and files belong to the **TaskExec server**, including when tasks use
remote runners. File paths must be absolute and inside `TASKEXEC_SECRETS_PATH` (or `dirs.secrets`).
For Kubernetes, mount a projected service account token inside this directory and supply its
path; the default service account path outside the directory is not implicitly allowed.
The Vault Kubernetes role must match the account, namespace and audience of that token.
JWT sources must supply fresh tokens issued for the configured role/audience; TaskExec does
not request JWTs from an identity provider. No interactive OIDC browser login is performed.

A successful login yields a temporary Vault token cached in server memory, scoped to the
connection and credentials. Concurrent operations share a login. While used, renewable tokens
are renewed after two-thirds of their lifetime; expired/nonrenewable tokens trigger another
login. There is no background renewal during idle periods. Restarting TaskExec loses the cache.
Source values are reread when preparing each operation, so file/environment rotation changes
the cached session. KV permission failures do not trigger repeated logins.

AppRole Secret IDs may expire or have a limited number of uses. Provide their rotation externally;
TaskExec does not generate Secret IDs. **Test authentication** can consume a Secret ID use,
especially for a new, unsaved connection. Single-use Secret IDs cannot support re-login after a
restart without a replacement. Enable **This AppRole does not require a Secret ID** only for
roles explicitly configured with `bind_secret_id=false` in Vault.

**Test authentication** checks a draft without saving it or writing secret data. Token mode uses
`auth/token/lookup-self` and needs permission for that endpoint. Other methods validate the login.
Success does not guarantee `read`/`list`/write permissions on every secret path.

## Keys and values

The secret path identifies a document relative to the KV mount. Choose fields separately
from the current list read from Vault. Only the path and mappings are stored. Every field
of one credential is read from the same document version.

| Key type | Required mapping | Optional mappings | Result |
| --- | --- | --- | --- |
| Variable set | None | None | Complete object, preserving nested values and types |
| String | Secret field | None | Selected scalar; numbers and booleans become strings |
| Login/password | Password | Username | Credentials usable like a local login/password key |
| SSH | Private key | Username, passphrase | Credentials usable like a local SSH key |

For `accounts/deploy` containing `user` and `pwd`, select username → `user`, password → `pwd`.
For SSH, arbitrary fields such as `pem`, `user` and `phrase` can be mapped. Omitted optional
mappings remain empty, as with local credentials. Mapped fields must exist; credential fields
must be strings and required credentials nonempty. String keys reject objects, arrays and null.

### Use keys in tasks through variable groups

1. Create `blabla`, using **Storage**, secret path `proxmox`, and type **Variable set**.
2. Open a variable group → **Keys** → **Add key**, and select `blabla`.
3. Choose the variable name (defaults to the key name), target, and either **Entire value**
   or a specific field. The form shows current values and the read time. **Refresh value**
   reads the source again. Only the key ID, name, target and selected field are saved;
   previews are never copied into the group's JSON or browser storage.
4. Attach the variable group to the task template.

Given `{"dns_server":"192.0.2.53","enabled":true}`, Ansible receives a real object:
`{{ blabla.dns_server }}` and `{{ blabla.enabled }}` work. A variable set is a named object,
not an array. For field names containing punctuation or dictionary method names such as
`items`, use `{{ blabla['items'] }}`.

Selecting `dns_server` with alias `dns` gives `{{ dns }}`. A String key bound as `blabla` gives
`{{ blabla }}`. Selected nested objects/arrays retain their types.

With a variable set and **Entire value**, TaskExec creates the base ENV containing the
whole JSON object plus an ENV for every top-level field. For alias `test` and document
`{"username":"demo","password":"example","options":{"enabled":true}}`:

| ENV | Content |
| --- | --- |
| `$test` | Entire JSON object; e.g. `printf '%s' "$test" \| jq -r .username` |
| `$test_username` | String value of `username` |
| `$test_password` | String value of `password` |
| `$test_options` | JSON object from `options` |

Names use `PREFIX_FIELD`, preserving case. Nested fields are not expanded recursively.
Selecting a single field creates only the base ENV. ENV previews and field selectors show
names only, without contents. Generated names must match `[A-Za-z_][A-Za-z0-9_]*` and be at
most 255 characters. Bind a field with an incompatible name separately under a valid alias.
A collision with another variable in the group aborts task preparation without adding a
partial expansion. Both values and field names are read again on every run.

The **Environment variables** target sends strings unchanged and JSON-encodes objects, arrays,
numbers, booleans and null. Ansible reads these with `lookup('env', 'NAME')`, adding `from_json`
for an object; shell uses `$NAME`. The extra-variable target uses JSON `--extra-vars` for Ansible,
`-var` for Terraform/OpenTofu and `name=value` arguments for scripts (structured values as JSON).

A key can have multiple aliases and field selections. Names must match `[A-Za-z_][A-Za-z0-9_]*`.
Duplicate names for the same target within a group are rejected, including plain and secret
variables. Across groups, the last key binding with the same name and target wins. SSH and
login/password keys are used by resource credential selectors, not variable bindings.
Referenced keys cannot be deleted or changed to an incompatible type.

Live previews require permission to manage project resources. Saving references does not
require a remote read. Each task reads values again during preparation: rotation needs no
group edit. Missing documents/fields and access errors fail preparation instead of reusing
stale values. For remote runners, the server resolves values during preparation and sends them
through the runner's task channel. Playbooks can still print their own values; use Ansible
`no_log: true` on sensitive steps to keep those values out of task output.

Deleting a key through the UI removes only its TaskExec reference, including for writable
connections. The value in Vault is retained.

For integrations, the API retains a separate write mode: without `reference_only`, values
can be written to connections with **Read only** disabled; updates need `override_secret: true`.
An empty path on such a create allocates `taskexec/<random-id>`. Writes preserve unrelated
document fields. API deletion without `reference_only=true` removes the remote field or
document for a regular key in a writable connection. KV v2 soft-deletes the current version
and preserves history. Synchronized keys and read-only connections delete only the reference.
A deleted KV v2 path with history must be restored or replaced with a new path.

Deleting a connection
never deletes remote secrets and is refused while keys or variable groups still reference it.

## Mapping key values in a variable group

Use **+** beside **Access keys** at the top of the group form. Select a **String** or
**Variable set** key and a unique prefix, such as `test`. Prefixes are case-sensitive,
start with an ASCII letter or `_`, and contain only ASCII letters, digits and `_`.
One key may have several prefixes. Sources do not export task variables by themselves.

On **Variables** or **Secrets**, name the target variable and insert a reference using
`{}` or by typing `{{`. Suggestions show current field paths without secret contents.
The refresh button beside a source reloads its fields.

| Name | Form value | Result |
| --- | --- | --- |
| `dns_server` | `{{ test.dns_server }}` | Selected key field |
| `DATABASE_PASSWORD` | `{{ test.password }}` | Password under an arbitrary ENV name |
| `DATABASE_URL` | `postgres://{{ test.username }}:{{ test.password }}@db` | Composed string |
| `settings` | `{{ test }}` | Whole value with its original types |
| `first_dns` | `{{ test["dns.servers"][0] }}` | Array element in a field containing a dot |

Expressions work in table, JSON and YAML modes, including nested objects and arrays.
Quote expressions in JSON/YAML: `port: "{{ test.port }}"`. A whole-value reference preserves
numbers, booleans, objects, arrays and null. Within a composed string, non-string values
are JSON-encoded. ENV values are strings, with non-string values serialized as JSON;
these mappings do not automatically create child variables. Use the **Keys** tab to export
a complete variable set with the existing `NAME_FIELD` expansion.

Plain `test.password` stays literal. Only prefixes configured in the group are resolved;
other Ansible expressions are left intact. Remote secret contents are never evaluated
recursively. Operators, filters and function calls are not supported.

Saving stores sources and expressions without reading or writing Vault. Literal secrets
remain encrypted locally. Each task reads current values during preparation and sends
them through its secret channel. Missing fields or source errors stop preparation.
Referenced keys cannot be deleted. Removing or renaming a prefix requires fixing its
remaining expressions first.

## API

Use `/api/project/{project_id}/secret_storages` for GET/POST and append `/{storage_id}` for
GET/PUT/DELETE. PUT includes the matching `id` and `project_id`.
Connection parameters are `params.url`, `params.mount`, `params.namespace`, `params.kv_version`,
`params.auth_method` (`token`, `approle`, `kubernetes`, `jwt`, `cert`), `params.auth_mount`,
`params.auth_role`, and the AppRole-only boolean `params.without_secret_id`.
`credentials` maps `token`, `role_id`, `secret_id`, `jwt`, `client_cert`, `client_key` and the optional
`ca_cert` to `{ "source": "database|env|file", "value": "..." }`. Only fields applicable to the
selected method are accepted. GET returns `configured: true` and masks database values; source
references remain editable. `clear: true` explicitly removes an optional credential. The public
`params` object cannot contain secrets. Connection and credential updates are transactional.

POST `/api/project/{project_id}/secret_storages/test` accepts the same draft; include its `id`
when testing edits that preserve saved credentials. It requires Manage project resources.

The connection type is `vault`; the default authentication method is `token`.
Create/update responses never echo submitted credential values.

Protocol references: [Vault KV v1](https://developer.hashicorp.com/vault/api-docs/secret/kv/kv-v1),
[Vault KV v2](https://developer.hashicorp.com/vault/api-docs/secret/kv/kv-v2).

### Editing references through the API

POST/PUT `/api/project/{project_id}/keys[/<key_id>]` with `reference_only: true` saves a
reference without reading or writing Vault. Supply `type` (`object`, `string`, `ssh` or `login_password`),
`source_storage_type: "vault"`, `source_storage_id`, `source_storage_key` (path only) and
`source_mapping`: `{"value":"dns_server"}` for String, `{"login":"user","password":"pwd"}`
for login/password, or `{"private_key":"pem","login":"user","passphrase":"phrase"}` for SSH.
Object keys have empty mappings; optional mappings may be omitted. PUT requires
matching `id` and `project_id`; `override_secret` and `generate_ssh_key` must be `false`, and
secret value fields must be empty. The storage and secret path are required for references.
Type constraints for keys used in host credential mappings still apply. DELETE on the same
resource with `?reference_only=true` removes only the reference. The UI uses these modes;
API write mode is described above.

### Fields, previews and variable bindings API

The **Secret path** field loads the first level of the storage. Choose a directory ending
in `/` or type a path: after `/`, its children are loaded and filtered by the typed name.
Suggestions require Vault `list` permission on directories: `secret/metadata/...` for KV v2,
`secret/...` for KV v1. Without `list`, enter the path manually; `read` permission is checked
separately when loading fields.

- POST `/api/project/{project_id}/secret_storages/{storage_id}/paths`, body `{"path":""}`,
  returns the first level: `{"paths":["apps/","proxmox"]}`. Pass `{"path":"apps/"}` to list
  that directory; results are full relative paths such as `apps/db`. No recursive traversal
  or value reads take place. A missing directory returns an empty list.
- POST `/api/project/{project_id}/secret_storages/{storage_id}/fields`, body `{"path":"proxmox"}`,
  returns `fields: [{"name":"dns_server","type":"string"}, ...]` without values.
- POST `/api/project/{project_id}/keys/{key_id}/preview` returns `value`, `type`, `read_at`.
  This explicitly reads a current string/object value; credentials and internal keys are rejected.
- POST `/api/project/{project_id}/keys/{key_id}/fields` returns field paths only:
  `{"paths":["",".username",".password"]}`. The empty path selects the entire key.
- All these endpoints require resource-management permission and send `Cache-Control: no-store`.
- Variable-group POST/PUT accepts `key_bindings`, for example
  `[{"key_id":12,"name":"blabla","type":"var","field":null}]`.
  `type` is `var` or `env`; null `field` selects the entire value, `"dns_server"` selects an object field.
  Normal group GET returns references without resolved values. Project backups save bindings
  by key name and restore them with remapped IDs.
- For expressions, group POST/PUT accepts `key_sources`, for example
  `[{"key_id":12,"prefix":"test"}]`. References remain strings inside `json` and `env`:
  `{"dns":"{{ test.dns_server }}"}`. Secrets-tab expressions are passed separately:
  `"secret_expressions":[{"name":"PASSWORD","type":"env","expression":"{{ test.password }}"}]`.
  Sources are also backed up by key name and restored with remapped IDs.
