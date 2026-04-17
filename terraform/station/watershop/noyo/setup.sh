#!/usr/bin/env bash
# setup.sh -- Initialize the noyo staging pond.
#
# Creates a local pond directory and installs factory nodes
# for HydroVu collection.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

source "$STAGING_DIR/env.sh"

# Export NOYO_ROOT for ${env:NOYO_ROOT} in apply configs
export NOYO_ROOT=$(cd "${SCRIPTS}/../../../../noyo-blue-econ" && pwd)

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Apply all configs: dirs, copies, factory nodes
${EXE} apply -f "${SCRIPTS}/apply.yaml"

echo
echo "=== Noyo staging pond setup complete ==="
echo "Next: ./noyo/run.sh    # collect from HydroVu + backup"
