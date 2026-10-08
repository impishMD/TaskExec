# Linux installation

**English** | [Русский](../ru/linux.md)

Download the matching amd64/arm64 DEB or RPM from
[GitHub Releases](https://github.com/impishMD/jeh/releases). Verify it against `checksums.txt`.

```sh
sudo apt install ./jeh_0.0.1_linux_amd64.deb
# Or: sudo dnf install ./jeh_0.0.1_linux_amd64.rpm
```

The package installs `/usr/bin/jeh`, `jeh.service`, system user/group `jeh`,
and writable `/etc/jeh` and `/var/lib/jeh` directories. It does not start an
unconfigured service. Install Ansible and the execution tools you intend to use.

```sh
cd /etc/jeh
sudo -u jeh /usr/bin/jeh setup
# Select /var/lib/jeh/database.sqlite for SQLite and /etc/jeh for configuration.
sudo systemctl enable --now jeh
sudo journalctl -u jeh -f
```

The service reads `/etc/jeh/config.json` and optional `/etc/jeh/env`.
Bind addresses, database settings and public URL are described in [Configuration](configuration.md).
Before upgrading, back up configuration and data. Install the new package and run
`sudo systemctl restart jeh`. Remove the service before deleting the account or data;
package installation and removal do not intentionally delete the database/configuration.
