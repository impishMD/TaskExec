<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
    <img src="docs/assets/logo-light.svg" alt="TaskExec" width="540">
  </picture>
</p>

<p align="center"><strong>Ansible · Terraform · OpenTofu · Scripts</strong></p>

[![CI](https://github.com/impishMD/TaskExec/actions/workflows/ci.yml/badge.svg)](https://github.com/impishMD/TaskExec/actions/workflows/ci.yml)
[![Release](https://github.com/impishMD/TaskExec/actions/workflows/release.yml/badge.svg)](https://github.com/impishMD/TaskExec/actions/workflows/release.yml)

**English** | [Русский](docs/ru/README.md)

TaskExec is a self-hosted web interface and API for running automation tasks.
Manage projects, repositories, inventories, credentials, reusable task templates,
schedules and execution logs in one place.

## Quick start

```sh
git clone https://github.com/impishMD/TaskExec.git taskexec
cd taskexec
cp .env.example .env
openssl rand -base64 32
# Set TASKEXEC_ADMIN_PASSWORD and TASKEXEC_ACCESS_KEY_ENCRYPTION in .env.
docker compose -f compose.yaml -f compose.dev.yaml up -d --build
```

Open <http://localhost:3000> and sign in with the credentials from `.env`.
The default Compose configuration listens on loopback and persists both configuration
and SQLite data. Set `TASKEXEC_BIND_ADDRESS` when exposing it through your own proxy.

The quick start builds `taskexec:dev` from this repository. See the [container guide](docs/en/containers.md)
for server and runner settings. To reuse an existing installation, follow
[Migration](docs/en/migration.md) before starting Compose with its database and configuration.

## Documentation

| Topic | Guide |
| --- | --- |
| Installation and operation | [English](docs/en/README.md) · [Русский](docs/ru/README.md) |
| Kubernetes and Helm | [English](docs/en/helm.md) · [Русский](docs/ru/helm.md) |
| Containers and runners | [English](docs/en/containers.md) · [Русский](docs/ru/containers.md) |
| Configuration and backups | [English](docs/en/configuration.md) · [Русский](docs/ru/configuration.md) |
| Linux packages and service | [English](docs/en/linux.md) · [Русский](docs/ru/linux.md) |
| API | [English](docs/en/api.md) · [Русский](docs/ru/api.md) |
| Building and testing | [English](docs/en/development.md) · [Русский](docs/ru/development.md) |
| Migration from JEH or upstream | [English](docs/en/migration.md) · [Русский](docs/ru/migration.md) |
| Release process | [English](docs/en/releasing.md) · [Русский](docs/ru/releasing.md) |

## Build from source

Use the Go version in `go.mod`, Node.js 24 and npm:

```sh
make deps
make build
./bin/taskexec setup
./bin/taskexec server --config ./config.json
```

The release pipeline produces Linux/macOS binaries, Linux DEB/RPM packages and a source archive.
Install Ansible and other execution tools separately when using a native binary.
Containers already include Ansible, Terraform, OpenTofu and Terragrunt.

## Project and license

TaskExec is an independent fork of Semaphore UI. Original copyright notices remain in
[LICENSE](LICENSE) and [NOTICE](NOTICE); dependencies are attributed in
[THIRD-PARTY-LICENSES.md](THIRD-PARTY-LICENSES.md).


[Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)
