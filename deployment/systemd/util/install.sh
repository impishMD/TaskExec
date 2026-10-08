#!/usr/bin/env bash
set -e

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

mkdir -p /etc/jeh
cp ${HERE}/../jeh.service /etc/systemd/system
cp ${HERE}/../env /etc/jeh/env
systemctl daemon-reload
systemctl start jeh.service