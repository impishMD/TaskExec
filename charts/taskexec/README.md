# TaskExec Helm chart

Deploys one TaskExec server with a persistent volume, Service and optional Ingress.
Kubernetes 1.26+ and Helm 3 or 4 are required. Chart and application versions are independent;
`image.tag` defaults to `v<appVersion>` and `image.digest` takes precedence.

## Installation

Create a namespace and a Secret named `taskexec-credentials` in it. The Secret must contain
`TASKEXEC_ADMIN_PASSWORD` and `TASKEXEC_ACCESS_KEY_ENCRYPTION` (a base64-encoded 32-byte key).
Use your Secret manager or the instructions in the installation guides. Keep this key unchanged
through upgrades and retain it with your backups.

```sh
helm repo add taskexec https://impishmd.github.io/TaskExec
helm repo update
helm upgrade --install taskexec taskexec/taskexec --version 1.0.0 \
  --namespace taskexec --create-namespace \
  --set existingSecret=taskexec-credentials --wait --timeout 5m
kubectl -n taskexec port-forward service/taskexec-taskexec 3000:3000
```

Open `http://localhost:3000`. The initial username is `admin`; its password comes from the Secret.
To install from a checkout, replace `taskexec/taskexec --version 1.0.0` with `./charts/taskexec`.
The repository URL is available after the first chart release and GitHub Pages deployment.

Complete guides: [English](https://github.com/impishMD/TaskExec/blob/develop/docs/en/helm.md) ·
[Русский](https://github.com/impishMD/TaskExec/blob/develop/docs/ru/helm.md).

## Values

See [values.yaml](values.yaml) for every setting; [values.schema.json](values.schema.json) validates inputs.

| Setting | Default | Purpose |
| --- | --- | --- |
| `existingSecret` | required | Bootstrap password, encryption key and external database credentials |
| `config.webRoot` | empty | Public URL, including a subpath when used |
| `config.admin` | `admin`, `Administrator`, `admin@localhost` | Initial administrator |
| `config.database.dialect` | `sqlite` | `sqlite`, `postgres` or `mysql` |
| `config.database.host/port/name/options` | empty / automatic / `taskexec` / `{}` | External database; no database subchart is installed |
| `image.repository` | `ghcr.io/impishmd/taskexec` | Server image; Docker Hub alternative: `impishmd/taskexec` |
| `image.tag/digest` | app version / empty | Explicit image version or immutable digest |
| `imagePullSecrets` | `[]` | Registry credentials |
| `service.type/port` | `ClusterIP` / `3000` | Internal or externally exposed HTTP Service |
| `ingress` | disabled | Ingress class, hosts, paths, annotations and TLS Secrets |
| `persistence.size` | `10Gi` | Configuration and SQLite data volume |
| `persistence.storageClass` | `null` | Default class; `""` selects no class; a name selects that class |
| `persistence.existingClaim` | empty | Use a PVC already in the release namespace |
| `persistence.retain` | `true` | Retain chart-created PVC on Helm uninstall and Argo CD deletion/pruning |
| `volumePermissions.enabled` | `false` | Root init container to repair imported volume ownership |
| `resources` | requests: 100m CPU / 256Mi RAM; limit: 1Gi RAM | Shared by the server and locally executed tasks |
| `podSecurityContext` | UID/GID 1001, fsGroup 1001 | Non-root server; no Kubernetes API token is mounted |
| `extraEnv` | `{}` | Additional non-secret string values; built-in settings cannot be overridden |
| `extraEnvFrom` | `[]` | Additional Secret/ConfigMap references |
| `extraVolumes/extraVolumeMounts` | `[]` | Additional files, such as database/Vault CA certificates |
| `nodeSelector/tolerations/affinity` | empty | Pod placement |

External PostgreSQL/MySQL requires `TASKEXEC_DB_USER` and `TASKEXEC_DB_PASS` in `existingSecret`.
The database must already exist. ConfigMap values override duplicate keys from `extraEnvFrom`
and `existingSecret`; explicitly required Secret keys take precedence over all `envFrom` entries.
Use configuration fields for chart-managed settings. Kubernetes Secret changes require a pod restart.

## Operation

- Exactly one replica is supported, with `Recreate` updates and a short outage on upgrades.
- Both `/var/lib/taskexec/config` and the SQLite database use the PVC. External databases still
  require it for configuration, including session keys.
- Admin bootstrap settings apply only to initial setup; subsequent password changes use TaskExec.
- Back up the database, PVC configuration and encryption Secret together before upgrading.
  Database migrations may prevent an image-only rollback.
- Uninstall keeps the PVC by default. Reinstall with `persistence.existingClaim=<retained-pvc>`.
  This option does not copy data or migrate between SQLite and an external database.
- The server image includes automation tools; local tasks consume the pod's resources.
  Existing remote runners can connect to the server. This chart does not deploy runners.
- The filesystem is writable for tool installation. Temporary task files under `/tmp` survive
  container restarts but are removed when the pod is replaced. Installed tool versions outside
  the data PVC may need downloading again after replacement.
- `volumePermissions.enabled` requires a policy permitting a root init container. It only mounts
  the data volume and receives no application credentials.

Check connectivity with `helm test taskexec -n taskexec --logs`. See the repository's
[release guide](https://github.com/impishMD/TaskExec/blob/develop/docs/en/releasing.md) for publishing.
