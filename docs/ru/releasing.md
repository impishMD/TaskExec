# Выпуск версий

[English](../en/releasing.md) | **Русский**

`VERSION` хранит номер без `v`. Теги, названия GitHub Releases и версии образов имеют
формат `v<major>.<minor>.<patch>`, начиная с **v0.0.1**. Основная ветка — `develop`.

## Подготовка

1. Обновите `VERSION`, версии frontend-пакета и примеры развёртывания.
2. Добавьте EN/RU release notes в `docs/en/releases`, `docs/ru/releases` и запись в changelog.
3. Выполните `make test`, `make check`, `make build`, runtime smoke tests и `make release-check`.
4. Закоммитьте и запушьте изменения; дождитесь CI и Packages.
5. Создайте тег на проверенном коммите:

   ```sh
   git tag -a v0.0.1 -m "v0.0.1"
   git push origin v0.0.1
   ```

Существующие теги не перемещаются. `make release-snapshot` собирает локальный выпуск
без публикации. GoReleaser 2.18.2 создаёт четыре архива Linux/macOS amd64/arm64,
четыре DEB/RPM, архив исходников и `checksums.txt`. GPG-подпись пакетов не настроена.
В бинарные архивы входят обе языковые версии документации и лицензии.

## Доступы

Окружение GitHub **docker_hub** содержит **DOCKERHUB_USR** и **DOCKERHUB_TOKEN**.
Для GHCR используется `GITHUB_TOKEN` с `packages: write`. Проверки и сборки имеют права
чтения. Правила защиты окружения продолжают действовать; значения токенов в Git не хранятся.

## Публикация

Release проверяет версию, повторно запускает CI/Packages и создаёт черновик релиза.
Нативные amd64/arm64 jobs отправляют server/runner/job/helper по digest в Docker Hub и GHCR,
после чего проверяют опубликованные образы. Финальная job собирает multi-platform теги,
проверяет архитектуры, добавляет `container-digests.txt`, обновляет стабильные алиасы
и публикует GitHub Release. Prerelease не меняет `latest`; релизы выполняются последовательно.

Образы: `impishmd/jeh`, `ghcr.io/impishmd/jeh`.
Артефакты: [GitHub Releases](https://github.com/impishMD/jeh/releases).
При сбое перезапустите упавшие jobs: черновик будет продолжен. Опубликованные релизы
не перезаписываются. Два реестра обновляются не атомарно, повторный запуск завершает
частичную публикацию. Не публикуйте старый стабильный тег после нового: это откатит алиасы.
Видимость репозитория и пакетов настраивается отдельно от workflow.
