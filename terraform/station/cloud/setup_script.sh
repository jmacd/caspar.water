#!/bin/sh
# setup_script.sh -- Install caddy + podman on the cloud host.
#
# Pond initialization and systemd setup are handled by terraform
# provisioners after this script runs.
set -e

# Install caddy if not present
if ! command -v caddy >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -y
    apt-get install -y caddy
fi

# Install rsync if not present (needed for site-prod deploy to cloud)
if ! command -v rsync >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y rsync
fi

# Install podman if not present
if ! command -v podman >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y podman
fi

# Enable lingering for user timers
loginctl enable-linger jmacd

# Allow Caddy to traverse /home/jmacd for serving site files
chmod 711 /home/jmacd

echo Setup complete.
