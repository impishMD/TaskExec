# Разработка

[English](../en/development.md) | **Русский**

Требования: Go из `go.mod`, Node.js 24, npm, Make и Docker для проверки контейнеров.

```sh
make deps
make build
make test
make check
bash tools/test-binary.sh ./bin/jeh v$(cat VERSION)
make container
bash tools/test-container.sh impishmd/jeh:v$(cat VERSION) v$(cat VERSION) server
```

`web/` — Vue 2/Vuetify, `api/public/` — генерируемый интерфейс, встроенный в Go-бинарник.
Backend находится в `cli/`, `api/`, `services/`, `db/`, `util/`; `pro/` — заглушки
открытой сборки. Путь модуля: `github.com/impishMD/jeh`.

`npm --prefix web run serve` запускает интерфейс разработки с API на порту 3000.
`make docs` обновляет справочники конфигурации и CLI; коммитьте их вместе с исходниками.
`make docs-check` проверяет актуальность справочников и локальные Markdown-ссылки.
Базы, секреты, `node_modules`, `bin`, `dist` не коммитятся. CI выполняет Go race tests,
тесты/lint интерфейса, проверки docs и контейнеров на обеих Linux-архитектурах.
[Выпуск версии](releasing.md).
