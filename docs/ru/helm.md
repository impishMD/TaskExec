# Kubernetes и Helm

[English](../en/helm.md) | **Русский**

[Чарт TaskExec](../../charts/taskexec/README.md) разворачивает один сервер, постоянный том
и Service. Можно подключить Ingress/TLS и внешнюю PostgreSQL или MySQL.
Нужны Kubernetes 1.26+, Helm 3 или 4, StorageClass либо существующий PVC.

Образ сервера содержит инструменты автоматизации. Задачи выполняются в pod сервера
или на отдельно настроенных удалённых исполнителях. Чарт не устанавливает исполнителей,
базу данных, Redis или Ingress-контроллер. Несколько реплик сервера не поддерживаются.

## Реквизиты

Создайте Secret один раз в том же namespace, что и приложение. Чарт использует ссылку
на существующий Secret: пароли не попадают в values и манифесты Helm-релиза.
Для GitOps создавайте Secret через принятую у вас систему управления секретами.

При ручной установке:

```sh
kubectl create namespace taskexec
umask 077
bootstrap_dir=$(mktemp -d)
openssl rand -hex 24 | tr -d '\n' > "$bootstrap_dir/admin-password"
openssl rand -base64 32 | tr -d '\n' > "$bootstrap_dir/encryption-key"
kubectl -n taskexec create secret generic taskexec-credentials \
  --from-file=TASKEXEC_ADMIN_PASSWORD="$bootstrap_dir/admin-password" \
  --from-file=TASKEXEC_ACCESS_KEY_ENCRYPTION="$bootstrap_dir/encryption-key"
# Сохраните оба файла в менеджере паролей/резервной копии перед удалением временных файлов.
rm -r "$bootstrap_dir"
```

Ключ шифрования должен оставаться одинаковым при замене pod и обновлении Helm-релиза.
Без него TaskExec не сможет расшифровать сохранённые реквизиты. Пароль администратора
используется только при первоначальной настройке; далее меняйте его через TaskExec.

## Установка

Опубликованный Helm-репозиторий доступен после выпуска `chart-v…` и развёртывания GitHub Pages.
Для первого выпуска сначала опубликуйте образ приложения `v1.0.0`: [порядок выпуска](releasing.md).

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

Откройте <http://localhost:3000>. Логин — `admin`, пароль — сохранённый выше.
Для установки из исходников замените `taskexec/taskexec --version 1.0.0` на `./charts/taskexec`.
До публикации образа соберите и загрузите собственный образ сервера в реестр и укажите
`image.repository` и `image.tag`; для локального kind можно загрузить образ прямо в кластер.

По умолчанию используется `ghcr.io/impishmd/taskexec:v1.0.0`. Для Docker Hub задайте
`image.repository: impishmd/taskexec`. `image.digest` позволяет закрепить образ по `sha256:…`
и имеет приоритет над тегом. Для закрытого реестра укажите `imagePullSecrets` в namespace релиза.

## Ingress и TLS

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

Создайте TLS Secret отдельно или настройте выпуск сертификата через `ingress.annotations`.
Контроллер должен сохранять путь запроса и поддерживать WebSocket.
Для установки в корне домена используйте путь `/` и уберите `/taskexec` из `config.webRoot`.
Проверки здоровья и `helm test` учитывают подпуть автоматически. TLS завершается на Ingress;
Service обращается к pod по HTTP, порт 3000.

## Внешняя база данных

Добавьте `TASKEXEC_DB_USER` и `TASKEXEC_DB_PASS` в `taskexec-credentials`, создайте базу
и выдайте пользователю права на создание и изменение её схемы. Выберите БД при первой установке:

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

Для MySQL используйте `dialect: mysql` и порт `3306`; параметры TLS описаны в
[справочнике конфигурации](../en/reference/configuration.md). Значения `options` — строки.
Не добавляйте порт в `host`. Реквизиты подключения остаются в Secret.
Изменение адреса или типа базы не переносит существующие данные.

PVC нужен и с внешней БД: в нём хранится `config/config.json`, включая ключи подписи сессий.
Сертификаты CA и другие файлы можно подключить через `extraVolumes` и `extraVolumeMounts`.
`extraEnv` добавляет несекретные строковые параметры, `extraEnvFrom` — ссылки на дополнительные
Secret/ConfigMap. Для переменных, которыми управляет чарт, используйте отдельные поля values.

## Хранение данных и обновление

PVC подключён в `/var/lib/taskexec`. SQLite использует `database.sqlite`, конфигурация —
`config/config.json`. Выберите StorageClass, подходящий для работы базы данных.
`persistence.storageClass: null` выбирает класс по умолчанию; `""` — отсутствие класса.
`persistence.existingClaim` подключает готовый PVC из namespace релиза.

Перед обновлением прекратите запуск новых задач, дождитесь завершения текущих и сохраните
базу, конфигурацию с PVC и Secret с ключом шифрования. SQLite копируйте при остановленном
сервере либо через согласованную резервную копию/снимок. Обновление использует `Recreate`
и вызывает короткий перерыв в доступности.
Миграции БД выполняются при запуске; `helm rollback` не откатывает схему базы.
Если старая версия не поддерживает новую схему, восстанавливайте совместимую резервную копию.

Изменения конфигурации в values автоматически перезапускают pod. Обновление содержимого
внешнего Secret само по себе не вызывает перезапуск. После изменения реквизитов:

```sh
kubectl -n taskexec rollout restart deployment/taskexec-taskexec
kubectl -n taskexec rollout status deployment/taskexec-taskexec
```

При удалении Helm-релиза PVC сохраняется. Он также защищён от удаления/prune в Argo CD.
Удаление namespace или авария хранилища всё равно могут уничтожить данные; это не замена
резервным копиям. Для повторной установки с сохранённым томом:

```sh
helm upgrade --install taskexec taskexec/taskexec --version 1.0.0 -n taskexec \
  -f taskexec-values.yaml --set persistence.existingClaim=taskexec-taskexec --wait
```

При импорте файлов сохраните владельца UID/GID 1001 или включите
`volumePermissions.enabled: true`: init-контейнер исправит права от root.
Оставляйте его выключенным, если драйвер учитывает `fsGroup` или политика кластера запрещает root.

Локальные задачи делят CPU и память с сервером; задавайте `resources` под свою нагрузку.
Временные файлы задач и инструменты, установленные вне PVC, не сохраняются при замене pod.
Для вынесения нагрузки используйте удалённых исполнителей.

## Argo CD

Сначала создайте Secret с реквизитами. Пример Application:

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

Полный набор параметров: [values.yaml](../../charts/taskexec/values.yaml).
Справка: [репозитории Helm](https://helm.sh/docs/topics/chart_repository/),
[постоянные тома Kubernetes](https://kubernetes.io/docs/concepts/storage/persistent-volumes/).
