#!/usr/bin/env bash
# setup.sh -- Initialize the water pond on the gateway.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"
VOLUME=pond-water

# Create podman volume if needed
if ! podman volume exists "${VOLUME}" 2>/dev/null; then
    echo "Creating podman volume: ${VOLUME}"
    podman volume create "${VOLUME}"
fi

${EXE} init

# Create directory structure
${EXE} mkdir -p /ingest
${EXE} mkdir -p /system/run

# Copy site content and templates into the pond
${EXE} copy host:///root/site/content /content
${EXE} copy host:///root/site/templates /templates
${EXE} copy host:///root/site/img /img

# Install factory nodes
${EXE} mknod logfile-ingest /etc/ingest --config-path /root/config/ingest.yaml
${EXE} mknod remote /system/run/1-backup --config-path /root/config/backup.yaml
${EXE} mknod dynamic-dir /reduced --config-path /root/config/reduce.yaml
${EXE} mknod dynamic-dir /analysis --config-path /root/config/analysis.yaml
${EXE} mknod sitegen /etc/site --config-path /root/site/site.yaml

echo
echo "=== Water pond setup complete ==="
