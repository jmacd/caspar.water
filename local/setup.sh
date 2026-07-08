#!/usr/bin/env bash
# setup.sh -- Initialize the local site pond for content development.
#
# Uses the canonical config/site.yaml plus local import configs.
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/.." && pwd)
EXE="${SCRIPTS}/pond.sh"

source "$SCRIPTS/env.sh"

# Ensure vendor dependencies are available (DuckDB-WASM, Plot, D3).
# These are needed for charts to work locally. One-time download.
VENDOR_DIR="${REPO_ROOT}/watertown/crates/sitegen/vendor/dist"
if [ ! -f "${VENDOR_DIR}/duckdb-eh.wasm" ]; then
    echo "=== Downloading vendor dependencies (one-time) ==="
    cd "${REPO_ROOT}/watertown/crates/sitegen/vendor" && bash download.sh
    cd "${SCRIPTS}"
fi

set -x

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Apply canonical site config (dirs, git-ingest, imports, sitegen)
${EXE} apply -f "${REPO_ROOT}/config/site.yaml"

echo
echo "=== Local site pond setup complete ==="
echo "Next: ./sync.sh      # pull content + data"
echo "Then: ./generate.sh  # build the site"
echo "Then: ./serve.sh     # serve locally"
