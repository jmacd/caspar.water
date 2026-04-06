#!/usr/bin/env bash
# setup.sh -- Initialize the site staging pond (cross-pond sitegen).
#
# Creates the podman volume, initializes the pond, copies site content
# and templates, installs import remotes and sitegen.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"
VOLUME=pond-site-staging

# Create podman volume if needed
if ! podman volume exists "${VOLUME}" 2>/dev/null; then
    echo "Creating podman volume: ${VOLUME}"
    podman volume create "${VOLUME}"
fi

${EXE} init

# Create directory structure
${EXE} mkdir -p /system/etc
${EXE} mkdir -p /sources

# Copy site content and templates into the pond
${EXE} copy host:///root/site/content /content
${EXE} copy host:///root/site/templates /templates
${EXE} copy host:///root/site/img /img

# Install import factories (pull from MinIO staging buckets)
${EXE} mknod remote /system/etc/10-water --config-path /root/config/import-water.yaml
${EXE} mknod remote /system/etc/11-noyo --config-path /root/config/import-noyo.yaml

# Install combined sitegen
${EXE} mknod sitegen /system/etc/90-sitegen --config-path /root/config/site.yaml

echo
echo "=== Site staging pond setup complete ==="
echo "Next: ./site/import.sh    # pull data from water + noyo ponds"
echo "Then: ./site/generate.sh  # build the combined site"
