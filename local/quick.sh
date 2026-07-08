#!/usr/bin/env bash
# quick.sh -- Fast layout/formatting iteration.
#
# Reads content directly from disk via host+sitegen:// (no pond, no sync).
# Uses the canonical config/site.yaml with --hostmount overlays to map
# pond paths (/content, /templates, /img) to the repo's site/ directory.
# --quick skips data exports and subsites — just builds content pages.
# Uncommitted changes to site/ are visible immediately.
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/.." && pwd)
WATERTOWN_ROOT="${REPO_ROOT}/watertown"
POND_BIN="${WATERTOWN_ROOT}/target/debug/pond"

if [ ! -x "${POND_BIN}" ]; then
    echo "Building pond binary..."
    (cd "${WATERTOWN_ROOT}" && cargo build --bin pond)
fi

BUILDDIR="${SCRIPTS}/build"
rm -rf "${BUILDDIR}"
mkdir -p "${BUILDDIR}"

set -x
"${POND_BIN}" run \
    -d "${REPO_ROOT}" \
    --hostmount /content=site/content \
    --hostmount /templates=site/templates \
    --hostmount /img=site/img \
    "host+sitegen:///config/site.yaml" \
    -- build --quick "${BUILDDIR}"

echo
echo "=== Quick build done ==="
echo "Output: ${BUILDDIR}"
echo "Next: ./serve.sh"
