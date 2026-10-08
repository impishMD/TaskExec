# Установка в Linux

[English](../en/linux.md) | **Русский**

Загрузите DEB/RPM для amd64 или arm64 из
[GitHub Releases](https://github.com/impishMD/jeh/releases) и проверьте `checksums.txt`.

```sh
sudo apt install ./jeh_0.0.1_linux_amd64.deb
# Или: sudo dnf install ./jeh_0.0.1_linux_amd64.rpm
```

Пакет устанавливает `/usr/bin/jeh`, `jeh.service`, пользователя/группу `jeh`
и каталоги `/etc/jeh`, `/var/lib/jeh`. Служба не запускается без настройки.
Ansible и другие используемые инструменты установите отдельно.

```sh
cd /etc/jeh
sudo -u jeh /usr/bin/jeh setup
# Для SQLite: /var/lib/jeh/database.sqlite; каталог конфигурации: /etc/jeh.
sudo systemctl enable --now jeh
sudo journalctl -u jeh -f
```

Служба читает `/etc/jeh/config.json` и необязательный `/etc/jeh/env`.
Перед обновлением сделайте резервную копию, установите новый пакет и выполните
`sudo systemctl restart jeh`. До удаления пользователя или данных остановите службу.
Установка/удаление пакета не предназначены для удаления конфигурации или базы.
