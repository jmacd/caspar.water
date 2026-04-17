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

# Apply all configs: dirs, copies, factory nodes
${EXE} apply -f "${SCRIPTS}/water.yaml"

echo
echo "=== Water pond setup complete ==="
