#!/bin/sh
set -eu
getent group jeh >/dev/null || groupadd --system jeh
id jeh >/dev/null 2>&1 || useradd --system --gid jeh --home-dir /var/lib/jeh --shell /usr/sbin/nologin jeh
install -d -m 0750 -o jeh -g jeh /var/lib/jeh /etc/jeh
