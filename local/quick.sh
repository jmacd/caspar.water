#!/usr/bin/env bash
# quick.sh -- Fast layout/formatting iteration.
#
# Reads content directly from disk via host+sitegen:// (no pond, no sync).
# Skips subsites and data exports -- just builds content pages.
# Uncommitted changes to site/ are visible immediately.
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/.." && pwd)
DUCKPOND_ROOT="${REPO_ROOT}/duckpond"
POND_BIN="${DUCKPOND_ROOT}/target/debug/pond"

if [ ! -x "${POND_BIN}" ]; then
    echo "Building pond binary..."
    (cd "${DUCKPOND_ROOT}" && cargo build --bin pond)
fi

BUILDDIR="${SCRIPTS}/build"
rm -rf "${BUILDDIR}"
mkdir -p "${BUILDDIR}"

set -x
"${POND_BIN}" run \
    -d "${REPO_ROOT}" \
    "host+sitegen:///local/quick-site.yaml" \
    build "${BUILDDIR}"

echo
echo "=== Quick build done ==="
echo "Output: ${BUILDDIR}"
echo "Next: ./serve.sh"
