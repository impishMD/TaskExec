# Kubernetes and Helm

**English** | [Русский](../ru/helm.md)

The [TaskExec chart](../../charts/taskexec/README.md) deploys one server, a persistent volume
and a Service. Ingress/TLS and an external PostgreSQL or MySQL database are optional.
It requires Kubernetes 1.26+, Helm 3 or 4, and a storage class or an existing PVC.

The server includes automation tools. Tasks can run in the server pod or on separately
configured remote runners. The chart does not install runners, databases, Redis or an
Ingress controller. Multiple server replicas are not supported.

## Credentials

Create credentials once in the same namespace as the release. The chart references an
existing Secret, so credentials do not appear in Helm values or release manifests.
For GitOps, provision this Secret using your existing secret-management system.

For a manual installation:

```sh
kubectl create namespace taskexec
umask 077
bootstrap_dir=$(mktemp -d)
openssl rand -hex 24 | tr -d '\n' > "$bootstrap_dir/admin-password"
openssl rand -base64 32 | tr -d '\n' > "$bootstrap_dir/encryption-key"
kubectl -n taskexec create secret generic taskexec-credentials \
  --from-file=TASKEXEC_ADMIN_PASSWORD="$bootstrap_dir/admin-password" \
  --from-file=TASKEXEC_ACCESS_KEY_ENCRYPTION="$bootstrap_dir/encryption-key"
# Store both files in your password manager/backup before removing the temporary copy.
rm -r "$bootstrap_dir"
```

Keep the encryption key unchanged across pod replacement and Helm upgrades. Losing it
prevents TaskExec from decrypting saved credentials. The administrator password is used
at first setup only; change it later through TaskExec.

## Install

The published repository is available after a `chart-v…` release has deployed GitHub Pages.
For the initial release, publish the application image `v1.0.0` first; see [Releasing](releasing.md).

```yaml
# taskexec-values.yaml
existingSecret: taskexec-credentials
persistence:
  size: 10Gi
```

```sh
helm repo add taskexec https://impishmd.github.io/TaskExec
helm repo update
helm upgrade --install taskexec taskexec/taskexec --version 1.0.0 \
  --namespace taskexec -f taskexec-values.yaml --wait --timeout 5m
helm test taskexec -n taskexec --logs
kubectl -n taskexec port-forward service/taskexec-taskexec 3000:3000
```

Open <http://localhost:3000>. Sign in as `admin` with the password saved above.
To install directly from this repository, use `./charts/taskexec` instead of
`taskexec/taskexec --version 1.0.0`. Before an image is published, build and push your own server
image and set `image.repository` and `image.tag`, or load it into a local kind cluster.

The default image is `ghcr.io/impishmd/taskexec:v1.0.0`. You can use Docker Hub by setting
`image.repository: impishmd/taskexec`, or pin `image.digest` to a `sha256:…` digest.
Private registries require `imagePullSecrets` in the release namespace.

## Ingress and TLS

```yaml
config:
  webRoot: https://tasks.example.com/taskexec
ingress:
  enabled: true
  className: nginx
  hosts:
    - host: tasks.example.com
      paths:
        - path: /taskexec
          pathType: Prefix
  tls:
    - secretName: taskexec-tls
      hosts: [tasks.example.com]
```

Create the TLS Secret separately or configure certificate management through
`ingress.annotations`. The Ingress controller must preserve the path and support WebSockets.
Use `/` for a domain-root deployment and remove `/taskexec` from `config.webRoot`.
Probes and `helm test` automatically use the public URL's subpath. TLS terminates at Ingress;
connections from the Service to the pod use HTTP on port 3000.

## External database

Add `TASKEXEC_DB_USER` and `TASKEXEC_DB_PASS` to `taskexec-credentials`, create the database,
and grant the user permission to create/alter its schema. Select the database on first install:

```yaml
config:
  database:
    dialect: postgres
    host: postgres.database.svc.cluster.local
    port: 5432
    name: taskexec
    options:
      sslmode: require
```

For MySQL use `dialect: mysql` and port `3306`; configure TLS according to the
[database settings](reference/configuration.md). Values in `options` must be strings.
Do not include the port in `host`. Database credentials stay in the Secret.
Changing `dialect` or the server address does not transfer existing data.

The PVC remains necessary with an external database: it stores `config/config.json`,
including the session-signing keys. Use `extraVolumes` and `extraVolumeMounts` for CA files
or other required configuration files. `extraEnv` adds non-secret string configuration;
`extraEnvFrom` accepts additional Secret/ConfigMap references. Chart-managed environment
variables must be configured through their dedicated values.

## Storage, updates and backups

The PVC mounts at `/var/lib/taskexec`. SQLite uses `database.sqlite` and configuration
uses `config/config.json`. Use a storage class suitable for database workloads.
`persistence.storageClass: null` selects the cluster default; `""` requests no class.
`persistence.existingClaim` uses an existing PVC in the release namespace.

Before an upgrade, stop new task submissions, let running tasks finish, and back up the
database, persisted configuration and encryption Secret. For SQLite, stop the server or
use a consistent SQLite backup/snapshot. Updates use `Recreate` and cause a short outage.
Database migrations run on startup; `helm rollback` does not undo database migrations.
Restore a matching database/configuration backup when an older binary cannot use the new schema.

Configuration changes in Helm values roll the pod automatically. Updating the contents of
an external Secret does not; after intentional credential changes, restart the deployment:

```sh
kubectl -n taskexec rollout restart deployment/taskexec-taskexec
kubectl -n taskexec rollout status deployment/taskexec-taskexec
```

A normal uninstall retains the PVC. It is also marked against Argo CD pruning/deletion.
Storage-cluster failure or namespace deletion can still destroy it; retention is not a backup.
To reinstall against the retained volume:

```sh
helm upgrade --install taskexec taskexec/taskexec --version 1.0.0 -n taskexec \
  -f taskexec-values.yaml --set persistence.existingClaim=taskexec-taskexec --wait
```

When importing old files, preserve UID/GID 1001 ownership or use
`volumePermissions.enabled: true` for a one-time root init container. Keep it disabled
where the storage driver already implements `fsGroup` or cluster policy prohibits root.

Local tasks share the pod's CPU/memory limit; adjust `resources` for your workloads.
Temporary task files and tools installed outside the data PVC are not durable across
pod replacement. Remote runners are useful for keeping execution workloads separate.

## Argo CD

Create the credentials Secret first. A Helm Application can then use:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: taskexec
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://impishmd.github.io/TaskExec
    chart: taskexec
    targetRevision: 1.0.0
    helm:
      releaseName: taskexec
      valuesObject:
        existingSecret: taskexec-credentials
  destination:
    server: https://kubernetes.default.svc
    namespace: taskexec
  syncPolicy:
    syncOptions: [CreateNamespace=true]
```

Full settings: [chart values](../../charts/taskexec/values.yaml).
Upstream references: [Helm repositories](https://helm.sh/docs/topics/chart_repository/)
and [Kubernetes persistent volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/).
