#!/usr/bin/env bash
# setup.sh -- Initialize the site staging pond (cross-pond sitegen).
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

source "$STAGING_DIR/env.sh"

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Apply canonical config (static resources: dirs, copies, sitegen).
# Import remotes are added by import.sh after discovering pond UUIDs.
${EXE} apply -f /config/site.yaml

echo
echo "=== Site staging pond setup complete ==="
