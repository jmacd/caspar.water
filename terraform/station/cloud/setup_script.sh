#!/bin/sh

# Install podman if not present
if ! command -v podman >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y podman
fi

# Enable lingering for user timers
loginctl enable-linger jmacd

# Make duckpond scripts executable
chmod +x /home/jmacd/duckpond/*.sh

# Set up duckpond site pond
su - jmacd -c "/home/jmacd/duckpond/setup.sh"

# Start services
systemctl daemon-reload
systemctl enable nginx
systemctl start nginx

# Start duckpond timer
systemctl --user -M jmacd@ daemon-reload
systemctl --user -M jmacd@ enable --now pond-site.timer

echo Setup complete.
