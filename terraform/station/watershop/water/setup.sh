#!/usr/bin/env bash
# setup.sh -- Initialize the water staging pond.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

source "$STAGING_DIR/env.sh"

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Apply canonical config
${EXE} apply -f /config/water.yaml

echo
echo "=== Water staging pond setup complete ==="
