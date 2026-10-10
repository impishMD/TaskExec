# TaskExec Helm chart

A chart for one TaskExec server, based on [Semaphore UI Charts](https://github.com/semaphoreui/charts/tree/92a563854accec1175a5e0cd48e8cd6219567d65/stable/semaphore).
It supports SQLite or an external PostgreSQL/MySQL database, persistent storage,
Ingress/TLS, OIDC, ServiceAccounts, custom CA certificates and additional containers.
Kubernetes 1.26+ and Helm 3 or 4 are required.

## Install

Create a Secret named `taskexec-credentials` in the release namespace with
`TASKEXEC_ADMIN_PASSWORD` and `TASKEXEC_ACCESS_KEY_ENCRYPTION` (a base64-encoded 32-byte key).
Then install:

```sh
helm repo add taskexec https://impishmd.github.io/TaskExec
helm repo update
helm upgrade --install taskexec taskexec/taskexec --version 1.0.2 \
  --namespace taskexec --create-namespace \
  --set secrets.existingSecret=taskexec-credentials --wait --timeout 5m
kubectl -n taskexec port-forward service/taskexec 3000:3000
```

Open `http://localhost:3000`; the initial username is `admin`. To use a checkout,
replace `taskexec/taskexec --version 1.0.2` with `./charts/taskexec`.
Full installation guides: [English](https://github.com/impishMD/TaskExec/blob/develop/docs/en/helm.md) ·
[Русский](https://github.com/impishMD/TaskExec/blob/develop/docs/ru/helm.md).

## OIDC

Create `taskexec-oidc` with a `client-secret` key, and add these values:

```yaml
general:
  host: https://tasks.example.com
oidc:
  enable: true
  providers:
    keycloak:
      displayName: Keycloak
      providerUrl: https://sso.example.com/realms/main
      clientId: taskexec
      existingSecret: taskexec-oidc
      clientSecretKey: client-secret
      scopes: [openid, profile, email]
      usernameClaim: preferred_username
      nameClaim: name
      emailClaim: email
```

Register `https://tasks.example.com/api/auth/oidc/keycloak/redirect` as the client's callback URL.
`providerUrl` is the issuer URL; discovery is automatic. The callback is generated from
`general.host`, including its subpath. `redirectUrl` can override it.

Each provider has its own `existingSecret`. `clientIdKey` reads the client ID from that Secret
instead of `clientId`. Client secrets are read from mounted files; they are never rendered into
the ConfigMap. `oidc.enable: false` disables all chart-configured OIDC providers.
`general.passwordLoginDisable: true` disables password sign-in; enable it after testing SSO.

For providers without discovery, replace `providerUrl` with `endpoint` containing
`issuerUrl`, `authUrl`, `tokenUrl`, `jwksUrl`, optional `userInfoUrl` and `algorithms`.
Provider options also include `returnViaState` (default `true`), `requireVerifiedEmail`,
`order`, `color` and `icon`. Provider IDs must remain stable after users have signed in.

## Values

[values.yaml](values.yaml) lists defaults; [values.schema.json](values.schema.json) validates inputs.

| Setting | Purpose |
| --- | --- |
| `secrets.existingSecret` | Required bootstrap/encryption Secret |
| `secrets.accesskeyEncryptionKey` | Encryption key entry; default `TASKEXEC_ACCESS_KEY_ENCRYPTION` |
| `admin.username/fullname/email` | Initial administrator identity |
| `admin.existingSecret/passwordKey` | Admin password; defaults to the main Secret and `TASKEXEC_ADMIN_PASSWORD` |
| `general.host` | Public URL, including any subpath |
| `general.passwordLoginDisable` | Disable password sign-in |
| `general.nonAdminCanCreateProject` | Allow project creation by non-admins |
| `database.type` | `sqlite` (default), `postgres` or `mysql` |
| `database.host/port/name/options` | External database connection |
| `database.existingSecret/usernameKey/passwordKey` | Database credentials; defaults to the main Secret and `TASKEXEC_DB_USER` / `TASKEXEC_DB_PASS` |
| `oidc.enable/providers` | OIDC providers and Secret mappings |
| `image.repository/tag/digest` | Server image; default `ghcr.io/impishmd/taskexec:v1.1.0`; digest takes precedence |
| `image.pullSecrets` | List of registry Secret names |
| `service` / `ingress` | HTTP Service, Ingress and TLS |
| `persistence` | PVC size/class, existing claim and retention |
| `volumePermissions.enabled` | Optional root init container for imported volumes |
| `serviceAccount` | Create or reference a ServiceAccount; API token automount is disabled |
| `customCertificates` | Append a CA bundle from an existing Secret or ConfigMap |
| `labels/annotations/podAnnotations` | Resource labels and workload annotations |
| `dnsConfig/nodeSelector/tolerations/affinity` | DNS and scheduling settings |
| `extraInitContainers/extraSidecarContainers` | Additional workload containers |
| `extraEnvVariables` | Non-secret environment values |
| `extraEnvSecrets` | Individual env mappings `{secret: name, key: entry}` |
| `envFromSecrets/envFromConfigMaps/extraEnvFrom` | Additional environment sources |
| `extraVolumes/extraVolumeMounts` | Extra mounted files |
| `config.forwarded_env_vars` | Environment names forwarded to tasks |

## Operation

Exactly one server replica is supported, with `Recreate` updates. The PVC stores
`config/config.json` and the SQLite database. It is retained on uninstall by default;
reuse it through `persistence.existingClaim`. External databases still need persistent configuration.
The chart does not deploy database servers, Redis or remote runners.

Keep the encryption key unchanged and back up the database, PVC configuration and Secret together.
Configuration changes roll the pod. Updating external Secrets used through environment variables
requires a pod restart; OIDC files are updated by Kubernetes and read on each login.
A custom CA bundle is assembled at startup and needs a restart after CA rotation.
Local tasks share the server's resource limits; use `resources` or remote runners as appropriate.

Use `helm test taskexec -n taskexec --logs` to check Service connectivity.
The chart is distributed under Apache-2.0; see [LICENSE](LICENSE) and [NOTICE](NOTICE).
