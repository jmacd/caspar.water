#!/bin/sh

systemctl daemon-reload

# Collector
chmod +x /home/jmacd/bin/collector
systemctl start collector

# Install podman if not present
if ! command -v podman >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y podman
fi

# Install and start user timers
systemctl --user -M jmacd@ daemon-reload

echo Installation complete.
