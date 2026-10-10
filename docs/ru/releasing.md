# Выпуск версий

[English](../en/releasing.md) | **Русский**

`VERSION` хранит номер без `v`. Теги, названия GitHub Releases и версии образов имеют
формат `v<major>.<minor>.<patch>`. Основная ветка — `develop`.

Для каждого выпуска выбирайте свободный номер версии и тег. Репозитории образов:
`impishmd/taskexec` и `ghcr.io/impishmd/taskexec`.

## Подготовка

1. Обновите `VERSION`, версии frontend-пакета и примеры развёртывания.
2. Обновите `CHANGELOG.md`, добавьте `release-notes/en/v<версия>.md` и `release-notes/ru/v<версия>.md` для GitHub Releases. В руководствах описывайте текущий функционал.
3. Выполните `make test`, `make check`, `make build`, runtime smoke tests и `make release-check`.
4. Закоммитьте и запушьте изменения; дождитесь CI и Packages.
5. Создайте тег на проверенном коммите:

   ```sh
   release_tag="v$(cat VERSION)"
   git tag -a "$release_tag" -m "$release_tag"
   git push origin "$release_tag"
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

Release требует успешный CI от push в `develop` или `main` для точного коммита тега.
Если проверка ещё выполняется, Release ждёт её завершения; отсутствие CI или ошибка
блокируют публикацию. Тесты, включая `go test -race`, и сборка временных CI-образов
выполняются только в CI ветки. Проверка использует
[API запусков workflow](https://docs.github.com/en/rest/actions/workflow-runs#list-workflow-runs-for-a-workflow).

Release проверяет версию, собирает и тестирует релизные пакеты, затем создаёт черновик.
Пакеты ветки — снимки с суффиксом `-dev`, а внутри CI-образов указана версия `dev`;
они не подходят для релиза. Нативные amd64/arm64 jobs один раз собирают версионные
server/runner/job/helper, отправляют их по digest в Docker Hub и GHCR и проверяют эти образы.
Финальная job использует проверенные digest без пересборки: создаёт multi-platform теги,
проверяет архитектуры, добавляет `container-digests.txt`, обновляет стабильные алиасы
и публикует GitHub Release. Prerelease не меняет `latest`; релизы выполняются последовательно.

Образы: `impishmd/taskexec`, `ghcr.io/impishmd/taskexec`.
Артефакты: [GitHub Releases](https://github.com/impishMD/TaskExec/releases).
При сбое перезапустите упавшие jobs: черновик будет продолжен. Опубликованные релизы
не перезаписываются. Два реестра обновляются не атомарно, повторный запуск завершает
частичную публикацию. Не публикуйте старый стабильный тег после нового: это откатит алиасы.
Видимость репозитория и пакетов настраивается отдельно от workflow.

## Выпуск Helm-чарта

Теги приложения и чарта независимы: `v1.0.6` выпускает приложение,
`chart-v1.0.1` — чарт версии `1.0.1`. Для изменений только в чарте увеличивайте `version`
в `Chart.yaml`; `appVersion` соответствует `VERSION`. Helm CI проверяет рендеринг в Helm 3/4,
схемы Kubernetes и установку собранного сервера в отдельном kind-кластере: вход, сохранность
данных при обновлении/перезапуске, подпуть URL и подключение сохранённого PVC.

Перед первой публикацией включите **Settings → Pages → Source: GitHub Actions**.
В правилах окружения `github-pages` разрешите развёртывание из тегов `chart-v*`.
Workflow создаёт GitHub Releases и обновляет ветку `gh-pages` через `GITHUB_TOKEN`;
отдельный персональный токен для чарта не нужен. Проверьте доступность образа в GHCR
либо настройте `imagePullSecrets` для закрытой установки.

1. Сначала выпустите соответствующую версию приложения и дождитесь завершения Release.
2. Укажите версии в `charts/taskexec/Chart.yaml`, проверьте чарт и добавьте описание выпуска
   в `release-notes/charts/en/v<версия-чарта>.md` и `release-notes/charts/ru/v<версия-чарта>.md`.
3. Закоммитьте и отправьте изменения, дождитесь Helm CI. Создайте тег `chart-v<версия-чарта>`
   на проверенном коммите и отправьте его.
4. Helm Release повторит проверки, проверит наличие образа приложения, упакует чарт,
   загрузит его в отдельный GitHub Release и опубликует индекс репозитория через Pages.

Адрес репозитория: `https://impishmd.github.io/TaskExec`. Первый выпуск сам создаёт `gh-pages`.
Релиз чарта не заменяет приложение в GitHub “Latest”. При повторном запуске индекс использует
уже опубликованный архив, сохраняя его контрольную сумму. Для продолжения прерванного выпуска
выберите **Helm Release → Run workflow** и укажите существующий тег чарта.
Не переносите опубликованные теги и не переиспользуйте версию для другого содержимого.

Локальные проверки (для Python нужен `PyYAML==6.0.3`):

```sh
make version-check helm-check
python3 tools/check-chart.py --release-tag chart-v1.0.1
helm package charts/taskexec --destination /tmp/taskexec-charts
```

Проверка установки с Docker и kind использует отдельный kubeconfig:

```sh
export KUBECONFIG=$(mktemp)
kind create cluster --name taskexec-chart --kubeconfig "$KUBECONFIG"
docker build --target server --build-arg VERSION="v$(cat VERSION)" -t taskexec:helm-ci .
kind load docker-image taskexec:helm-ci --name taskexec-chart
bash tools/test-chart-install.sh
kind delete cluster --name taskexec-chart
rm "$KUBECONFIG"
unset KUBECONFIG
```

Скрипт установки отказывается работать, если текущий контекст отличается от `kind-taskexec-chart`.
