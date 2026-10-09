#!/usr/bin/env bash
set -e

systemctl stop taskexec.service
systemctl disable taskexec.service
rm /etc/systemd/system/taskexec.service
rm -rf /etc/taskexec