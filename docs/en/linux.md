# Linux installation

**English** | [Русский](../ru/linux.md)

Use a DEB/RPM package for your architecture (amd64 or arm64). Build packages with
`make release-snapshot` following [Releasing](releasing.md), or select an available
TaskExec package from [GitHub Releases](https://github.com/impishMD/TaskExec/releases).
Verify it against the accompanying `checksums.txt` before installation.

```sh
sudo apt install ./taskexec_<version>_linux_amd64.deb
# Or: sudo dnf install ./taskexec_<version>_linux_amd64.rpm
```

The package installs `/usr/bin/taskexec`, `taskexec.service`, system user/group `taskexec`,
and writable `/etc/taskexec` and `/var/lib/taskexec` directories. It does not start an
unconfigured service. Install Ansible and the execution tools you intend to use.

```sh
cd /etc/taskexec
sudo -u taskexec /usr/bin/taskexec setup
# Select /var/lib/taskexec/database.sqlite for SQLite and /etc/taskexec for configuration.
sudo systemctl enable --now taskexec
sudo journalctl -u taskexec -f
```

The service reads `/etc/taskexec/config.json` and optional `/etc/taskexec/env`.
Bind addresses, database settings and public URL are described in [Configuration](configuration.md).
Before upgrading, back up configuration and data. Install the new package and run
`sudo systemctl restart taskexec`. Remove the service before deleting the account or data;
package installation and removal do not intentionally delete the database/configuration.
