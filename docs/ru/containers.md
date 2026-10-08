# Контейнеры и раннеры

[English](../en/containers.md) | **Русский**

## Образы

| Роль | Версия | Стабильный тег |
| --- | --- | --- |
| Сервер | `v0.0.1` | `latest` |
| Раннер | `v0.0.1-runner` | `latest-runner` |
| Среда задачи | `v0.0.1-job` | `latest-job` |
| Вспомогательный образ | `v0.0.1-helper` | `latest-helper` |

Все теги доступны в `impishmd/jeh` и `ghcr.io/impishmd/jeh` для Linux amd64/arm64.
Сервер и раннер работают с UID 1001; подключённые каталоги должны быть доступны на запись.

Используйте [compose.yaml](../../compose.yaml) и [.env.example](../../.env.example).
Тома `/etc/jeh` и `/var/lib/jeh` содержат конфигурацию и базу; при обновлении сохраняйте их.
Переменные администратора создают пользователя при первом запуске. Для смены пароля
существующего пользователя используйте `jeh user change-by-login`.
PostgreSQL/MySQL настраиваются через `JEH_DB_DIALECT`, `JEH_DB_HOST`, `JEH_DB_USER`,
`JEH_DB_PASS` и `JEH_DB`. Для секретов доступны, например, `JEH_DB_PASS_FILE`
и `JEH_ADMIN_PASSWORD_FILE`.

## Удалённый раннер

Включите `JEH_USE_REMOTE_RUNNER=true` на сервере и создайте токен регистрации
в интерфейсе администратора:

```sh
docker run -d --name jeh-runner --restart unless-stopped \
  -e JEH_WEB_ROOT=https://jeh.example.com \
  -e JEH_RUNNER_REGISTRATION_TOKEN="$RUNNER_REGISTRATION_TOKEN" \
  -v jeh-runner-data:/var/lib/jeh \
  impishmd/jeh:v0.0.1-runner
```

Том раннера хранит его зарегистрированный токен. Образы job/helper предназначены
для совместимых исполнителей; их наличие не добавляет закрытые Docker/Kubernetes
исполнители в открытую сборку.
