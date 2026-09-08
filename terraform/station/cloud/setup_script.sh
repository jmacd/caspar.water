#!/bin/sh
# setup_script.sh -- Install cloud host system dependencies.
#
# Caddy serves the site and proxies InfluxDB.  The native site-prod pond
# builds directly into ${HOME}/watertown/www and is installed separately
# from the promoted Watertown deb artifact.
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

# Install rsync for atomic site deployments from Watershop.
if ! command -v rsync >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y rsync
fi

# Allow Caddy to traverse /home/jmacd to serve site files
chmod 711 /home/jmacd

echo Setup complete.
