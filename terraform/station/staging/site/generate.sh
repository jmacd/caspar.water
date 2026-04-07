#!/usr/bin/env bash
# generate.sh -- Build the combined site from imported sources.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

export RUST_BACKTRACE=1
export POND_MAX_ALLOC_MB=1000

OUTDIR="${STAGING_DIR}/dist"

# Clear and recreate output dir
rm -rf "${OUTDIR}"
mkdir -p "${OUTDIR}"

# Run sitegen build
${EXE} run /system/etc/90-sitegen build "${OUTDIR}"

echo
echo "Site generated at: ${OUTDIR}"
echo "To preview: cd ${OUTDIR} && python3 -m http.server 4180"
