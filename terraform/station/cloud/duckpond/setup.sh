#!/usr/bin/env bash
# setup.sh -- Initialize the cross-pond site on the cloud machine.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"
VOLUME=pond-site

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

# Install import factories (pull from R2 production buckets)
${EXE} mknod remote /system/etc/10-water --config-path /root/config/import-water.yaml
${EXE} mknod remote /system/etc/11-noyo --config-path /root/config/import-noyo.yaml

# Install combined sitegen
${EXE} mknod sitegen /system/etc/90-sitegen --config-path /root/config/site.yaml

echo
echo "=== Cloud site pond setup complete ==="
