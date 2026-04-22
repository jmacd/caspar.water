#!/bin/sh

# Install caddy if not present
if ! command -v caddy >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -y
    apt-get install -y caddy
fi

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

# Ensure docroot exists
mkdir -p /var/www/html/casparwater

# Start services
systemctl daemon-reload
systemctl enable caddy
systemctl restart caddy

# Start duckpond timer
systemctl --user -M jmacd@ daemon-reload
systemctl --user -M jmacd@ enable --now pond-site.timer

echo Setup complete.
