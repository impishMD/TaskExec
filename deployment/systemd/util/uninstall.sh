#!/usr/bin/env bash
set -e

systemctl stop jeh.service
systemctl disable jeh.service
rm /etc/systemd/system/jeh.service
rm -rf /etc/jeh