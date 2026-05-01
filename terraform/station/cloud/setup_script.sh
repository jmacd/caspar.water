#!/bin/sh
# setup_script.sh -- Install caddy on the cloud host.
#
# The cloud host's only job is to serve the static site (built and
# rsync'd in from the watershop pond) plus terminate TLS via caddy.
# It does NOT run any duckpond instance -- that lived here historically
# but caused redundant R2 imports (cf. caspar.water remote-bandwidth-bug
# investigation).  Watershop's pond@site-prod builds and rsyncs the
# site to ${HOME}/duckpond/www/build-<ts>/ then atomically retargets
# the 'current' symlink, so this host needs only caddy + rsync over SSH.
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

# Install rsync if not present (watershop pushes builds over SSH)
if ! command -v rsync >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y rsync
fi

# Allow Caddy to traverse /home/jmacd to serve site files
chmod 711 /home/jmacd

echo Setup complete.
