# TaskExec

[English](../en/README.md) | **Русский**

TaskExec — веб-интерфейс и API для запуска Ansible, Terraform, OpenTofu и скриптов.
Создайте проект, добавьте репозиторий и ключи, настройте инвентарь и переменные,
затем создайте шаблон задачи. Задачи можно запускать вручную или по расписанию;
результаты и журналы доступны в интерфейсе.

## Быстрый запуск

```sh
git clone https://github.com/impishMD/TaskExec.git taskexec
cd taskexec
cp .env.example .env
openssl rand -base64 32
# Заполните TASKEXEC_ADMIN_PASSWORD и TASKEXEC_ACCESS_KEY_ENCRYPTION в .env.
docker compose -f compose.yaml -f compose.dev.yaml up -d --build
```

Откройте <http://localhost:3000>. Compose сохраняет конфигурацию и SQLite в томах,
а порт по умолчанию доступен только через loopback. Образ `taskexec:dev` собирается локально.
При переходе с существующей установки сначала выполните [миграцию](migration.md):
новое имя Compose создаёт новые тома, данные нужно перенести.

| Задача | Документация |
| --- | --- |
| Установка в Kubernetes | [Helm](helm.md) |
| Хранилища секретов HashiCorp Vault | [HashiCorp Vault](secret-storages.md) |
| Состояния, блокировки и история Terraform/OpenTofu | [HTTP backend](terraform-state.md) |
| Схемы, подтверждения и цепочки задач | [Рабочие процессы](workflows.md) |
| Контейнеры и раннеры | [Контейнеры](containers.md) |
| Параметры, ключи и резервные копии | [Конфигурация](configuration.md) |
| Установка пакета и systemd | [Linux](linux.md) |
| HTTP API | [API](api.md) |
| Сборка и проверки | [Разработка](development.md) |
| Переход с JEH или исходного проекта | [Миграция](migration.md) |
| Выпуск версии | [Релизы](releasing.md) |
| Все переменные окружения | [Генерируемый справочник EN](../en/reference/configuration.md) |
| Все команды CLI | [Генерируемый справочник EN](../en/reference/cli.md) |

TaskExec — независимый форк Semaphore UI с лицензией MIT. Авторство сохранено в
[LICENSE](../../LICENSE) и [NOTICE](../../NOTICE).
