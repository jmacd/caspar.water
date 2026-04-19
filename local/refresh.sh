#!/usr/bin/env bash
# refresh.sh -- Pull latest site content from git and regenerate.
# Use this for fast content/style iteration without re-importing data.
# Note: only committed changes are visible (git-ingest reads from the repo).
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

source "$SCRIPTS/env.sh"

POND_BIN="${DUCKPOND_ROOT}/target/debug/pond"

if [ ! -x "${POND_BIN}" ]; then
    echo "Building pond binary..."
    (cd "${DUCKPOND_ROOT}" && cargo build --bin pond)
fi

export POND="${SCRIPTS}/pond"

# Pull latest content from git
"${POND_BIN}" run /content pull

# Regenerate
export RUST_BACKTRACE=1
export POND_MAX_ALLOC_MB=3000

BUILDDIR="${SCRIPTS}/build"
rm -rf "${BUILDDIR}"
mkdir -p "${BUILDDIR}"
"${POND_BIN}" run /system/etc/90-sitegen build "${BUILDDIR}"

echo
echo "=== Site refreshed ==="
echo "Output: ${BUILDDIR}"
