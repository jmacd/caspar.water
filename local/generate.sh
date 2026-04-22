#!/usr/bin/env bash
# generate.sh -- Build the combined site locally.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

export RUST_BACKTRACE=1
export POND_MAX_ALLOC_MB=3000

BUILDDIR="${SCRIPTS}/build"

rm -rf "${BUILDDIR}"
mkdir -p "${BUILDDIR}"
${EXE} run /system/etc/90-sitegen build "${BUILDDIR}"

echo
echo "=== Site built ==="
echo "Output: ${BUILDDIR}"
echo "Next: ./serve.sh  # serve locally"
