#!/bin/sh
set -eu
getent group taskexec >/dev/null || groupadd --system taskexec
id taskexec >/dev/null 2>&1 || useradd --system --gid taskexec --home-dir /var/lib/taskexec --shell /usr/sbin/nologin taskexec
install -d -m 0750 -o taskexec -g taskexec /var/lib/taskexec /etc/taskexec
