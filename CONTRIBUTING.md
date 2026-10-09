# Contributing to TaskExec

Use issues and pull requests in [the source repository](https://github.com/impishMD/TaskExec).
The default branch is `develop`.

1. Read the [development guide](docs/en/development.md) ([Русский](docs/ru/development.md)).
2. Keep changes focused and preserve API/database compatibility unless explicitly documented.
3. Run the relevant tests, `make check` and `make build`. Test actual runtime behavior for
   configuration, container or packaging changes.
4. Update EN/RU documentation and run `make docs` when CLI/configuration changes.
5. Describe user-visible behavior and verification in the pull request.

Go tests live beside implementation; frontend tests are in `web/tests`.
Never commit credentials, databases or generated build directories.
Preserve LICENSE, NOTICE and third-party attributions.
