# Job Executor Hub (JEH)

[English](../en/README.md) | **Русский**

JEH — веб-интерфейс и API для запуска Ansible, Terraform, OpenTofu и скриптов.
Создайте проект, добавьте репозиторий и ключи, настройте инвентарь и переменные,
затем создайте шаблон задачи. Задачи можно запускать вручную или по расписанию;
результаты и журналы доступны в интерфейсе.

## Быстрый запуск

```sh
git clone https://github.com/impishMD/jeh.git
cd jeh
cp .env.example .env
openssl rand -base64 32
# Заполните JEH_ADMIN_PASSWORD и JEH_ACCESS_KEY_ENCRYPTION в .env.
docker compose up -d
```

Откройте <http://localhost:3000>. Compose сохраняет конфигурацию и SQLite в томах,
а порт по умолчанию доступен только через loopback.

| Задача | Документация |
| --- | --- |
| Контейнеры и раннеры | [Контейнеры](containers.md) |
| Параметры, ключи и резервные копии | [Конфигурация](configuration.md) |
| Установка пакета и systemd | [Linux](linux.md) |
| HTTP API | [API](api.md) |
| Сборка и проверки | [Разработка](development.md) |
| Переход с исходного проекта | [Миграция](migration.md) |
| Выпуск версии | [Релизы](releasing.md) |
| Все переменные окружения | [Генерируемый справочник EN](../en/reference/configuration.md) |
| Все команды CLI | [Генерируемый справочник EN](../en/reference/cli.md) |
| Первый релиз | [v0.0.1](releases/v0.0.1.md) |

JEH — независимый форк Semaphore UI с лицензией MIT. Авторство сохранено в
[LICENSE](../../LICENSE) и [NOTICE](../../NOTICE). Закрытые реализации Pro/Enterprise
в выпуск не входят; [доступность функций](configuration.md#доступность-функций).
