#!/bin/sh
set -e

# Older deployments do not have the retry timer yet.
if [ -f /etc/systemd/system/collector-retry.timer ]; then
    systemctl stop collector-retry.timer
fi
systemctl stop collector

echo Teardown complete.
