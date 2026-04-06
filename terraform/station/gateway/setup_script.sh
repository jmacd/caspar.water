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

# Enable lingering for user timers to survive logout
loginctl enable-linger jmacd

# Make duckpond scripts executable
chmod +x /home/jmacd/duckpond/water/*.sh
chmod +x /home/jmacd/duckpond/noyo/*.sh
chmod +x /home/jmacd/duckpond/env.sh

# Set up duckpond ponds
su - jmacd -c "/home/jmacd/duckpond/water/setup.sh"
su - jmacd -c "/home/jmacd/duckpond/noyo/setup.sh"

# Install and start user timers
systemctl --user -M jmacd@ daemon-reload
systemctl --user -M jmacd@ enable --now pond-water.timer
systemctl --user -M jmacd@ enable --now pond-noyo.timer

echo Installation complete.
