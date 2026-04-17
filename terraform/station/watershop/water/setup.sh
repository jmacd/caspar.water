#!/usr/bin/env bash
# setup.sh -- Initialize the water staging pond.
#
# Creates a local pond directory, copies site content and templates,
# and installs factory nodes.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

source "$STAGING_DIR/env.sh"

# Export for ${env:...} expansion in apply configs
export SITE_DIR=$(cd "${STAGING_DIR}/../../../site" && pwd)

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Apply all configs: dirs, copies, factory nodes
${EXE} apply -f "${SCRIPTS}/apply.yaml"

echo
echo "=== Water staging pond setup complete ==="
echo "Next: ./water/run.sh    # ingest data + backup"
