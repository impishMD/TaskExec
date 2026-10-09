# Установка в Linux

[English](../en/linux.md) | **Русский**

Используйте DEB/RPM для своей архитектуры (amd64 или arm64). Соберите пакеты командой
`make release-snapshot` по [инструкции выпуска](releasing.md) либо выберите доступный
пакет TaskExec в [GitHub Releases](https://github.com/impishMD/TaskExec/releases).
Перед установкой проверьте его по прилагаемому `checksums.txt`.

```sh
sudo apt install ./taskexec_<version>_linux_amd64.deb
# Или: sudo dnf install ./taskexec_<version>_linux_amd64.rpm
```

Пакет устанавливает `/usr/bin/taskexec`, `taskexec.service`, пользователя/группу `taskexec`
и каталоги `/etc/taskexec`, `/var/lib/taskexec`. Служба не запускается без настройки.
Ansible и другие используемые инструменты установите отдельно.

```sh
cd /etc/taskexec
sudo -u taskexec /usr/bin/taskexec setup
# Для SQLite: /var/lib/taskexec/database.sqlite; каталог конфигурации: /etc/taskexec.
sudo systemctl enable --now taskexec
sudo journalctl -u taskexec -f
```

Служба читает `/etc/taskexec/config.json` и необязательный `/etc/taskexec/env`.
Перед обновлением сделайте резервную копию, установите новый пакет и выполните
`sudo systemctl restart taskexec`. До удаления пользователя или данных остановите службу.
Установка/удаление пакета не предназначены для удаления конфигурации или базы.
