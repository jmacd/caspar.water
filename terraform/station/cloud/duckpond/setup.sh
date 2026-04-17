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

# Apply all configs: dirs, copies, import remotes, sitegen
${EXE} apply -f "${SCRIPTS}/site.yaml"

echo
echo "=== Cloud site pond setup complete ==="
