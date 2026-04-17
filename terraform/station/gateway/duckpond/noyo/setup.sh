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

# Apply all configs: dirs + factory nodes
${EXE} apply -f "${SCRIPTS}/noyo.yaml"

echo
echo "=== Noyo pond setup complete ==="
