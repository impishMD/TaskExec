<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="docs/assets/logo-dark.svg">
    <img src="docs/assets/logo-light.svg" alt="Job Executor Hub — JEH" width="540">
  </picture>
</p>

<p align="center"><strong>Ansible · Terraform · OpenTofu · Scripts</strong></p>

[![CI](https://github.com/impishMD/jeh/actions/workflows/ci.yml/badge.svg)](https://github.com/impishMD/jeh/actions/workflows/ci.yml)
[![Release](https://github.com/impishMD/jeh/actions/workflows/release.yml/badge.svg)](https://github.com/impishMD/jeh/actions/workflows/release.yml)
[![Docker pulls](https://img.shields.io/docker/pulls/impishmd/jeh)](https://hub.docker.com/r/impishmd/jeh)

**English** | [Русский](docs/ru/README.md)

Job Executor Hub (JEH) is a self-hosted web interface and API for running automation jobs.
Manage projects, repositories, inventories, credentials, reusable task templates,
schedules and execution logs in one place.

## Quick start

```sh
git clone https://github.com/impishMD/jeh.git
cd jeh
cp .env.example .env
openssl rand -base64 32
# Set JEH_ADMIN_PASSWORD and JEH_ACCESS_KEY_ENCRYPTION in .env.
docker compose up -d
```

Open <http://localhost:3000> and sign in with the credentials from `.env`.
The default Compose configuration listens on loopback and persists both configuration
and SQLite data. Set `JEH_BIND_ADDRESS` when exposing it through your own proxy.

Images for Linux amd64 and arm64 are published to both registries:

```text
impishmd/jeh:v0.0.1
ghcr.io/impishmd/jeh:v0.0.1
```

Use `v0.0.1-runner`, `v0.0.1-job` and `v0.0.1-helper` for the corresponding
execution images. Stable aliases are `latest`, `latest-runner`, `latest-job`
and `latest-helper`. [Container guide](docs/en/containers.md).

## Documentation

| Topic | Guide |
| --- | --- |
| Installation and operation | [English](docs/en/README.md) · [Русский](docs/ru/README.md) |
| Containers and runners | [English](docs/en/containers.md) · [Русский](docs/ru/containers.md) |
| Configuration and backups | [English](docs/en/configuration.md) · [Русский](docs/ru/configuration.md) |
| Linux packages and service | [English](docs/en/linux.md) · [Русский](docs/ru/linux.md) |
| API | [English](docs/en/api.md) · [Русский](docs/ru/api.md) |
| Building and testing | [English](docs/en/development.md) · [Русский](docs/ru/development.md) |
| Migration from upstream | [English](docs/en/migration.md) · [Русский](docs/ru/migration.md) |
| Release process | [English](docs/en/releasing.md) · [Русский](docs/ru/releasing.md) |
| First release | [v0.0.1](docs/en/releases/v0.0.1.md) · [v0.0.1 RU](docs/ru/releases/v0.0.1.md) |

## Build from source

Use the Go version in `go.mod`, Node.js 24 and npm:

```sh
make deps
make build
./bin/jeh setup
./bin/jeh server --config ./config.json
```

Release downloads include Linux/macOS binaries, Linux DEB/RPM packages and a source archive.
Install Ansible and other execution tools separately when using a native binary.
Containers already include Ansible, Terraform, OpenTofu and Terragrunt.

## Project and license

JEH is an independent fork of Semaphore UI. Original copyright notices remain in
[LICENSE](LICENSE) and [NOTICE](NOTICE); dependencies are attributed in
[THIRD-PARTY-LICENSES.md](THIRD-PARTY-LICENSES.md).
This release builds the available open-source code. Upstream proprietary Pro/Enterprise
implementations are not included; see [feature scope](docs/en/configuration.md#feature-scope).

[Contributing](CONTRIBUTING.md) · [Security](SECURITY.md) · [Changelog](CHANGELOG.md)
