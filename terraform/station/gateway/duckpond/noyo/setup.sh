#!/usr/bin/env bash
# setup.sh -- Initialize the noyo pond on the gateway.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"
VOLUME=pond-noyo

# Create podman volume if needed
if ! podman volume exists "${VOLUME}" 2>/dev/null; then
    echo "Creating podman volume: ${VOLUME}"
    podman volume create "${VOLUME}"
fi

${EXE} init

# Create directory structure
${EXE} mkdir -p /system/run
${EXE} mkdir -p /system/etc

# Install factory nodes
${EXE} mknod remote /system/run/1-backup --config-path /root/config/backup.yaml
${EXE} mknod hydrovu /system/etc/20-hydrovu --config-path /root/config/hydrovu.yaml
${EXE} mknod column-rename /system/etc/10-hrename --config-path /root/config/hrename.yaml
${EXE} mknod dynamic-dir /combined --config-path /root/config/combine.yaml
${EXE} mknod dynamic-dir /singled --config-path /root/config/single.yaml
${EXE} mknod dynamic-dir /reduced --config-path /root/config/reduce.yaml

echo
echo "=== Noyo pond setup complete ==="
