#!/usr/bin/env bash
set -e

HERE="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

mkdir -p /etc/taskexec
cp ${HERE}/../taskexec.service /etc/systemd/system
cp ${HERE}/../env /etc/taskexec/env
systemctl daemon-reload
systemctl start taskexec.service