#!/usr/bin/env bash
# generate.sh -- Build the combined site and deploy to nginx via atomic symlink.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

export RUST_BACKTRACE=1
export POND_MAX_ALLOC_MB=3000

WWW_ROOT="${WWW_ROOT:-/var/www/html}"
BUILDDIR="${STAGING_DIR}/build"

# Build into a temporary directory
rm -rf "${BUILDDIR}"
mkdir -p "${BUILDDIR}"
${EXE} run /system/etc/90-sitegen build "${BUILDDIR}"

# Deploy with atomic symlink swap (same pattern as production cron.sh)
TIMESTAMP=$(date -u +"%Y%m%d-%H%M%S")
DIST_NAME="staging-${TIMESTAMP}"

# Read the current symlink target to determine the old dist folder
OLD_DIST=""
if [[ -L "${WWW_ROOT}/staging" ]]; then
    OLD_DIST=$(readlink "${WWW_ROOT}/staging")
    echo "Found existing symlink pointing to: ${OLD_DIST}"
fi

# Move build to timestamped directory in WWW_ROOT
mv "${BUILDDIR}" "${WWW_ROOT}/${DIST_NAME}"

# Atomic symlink swap
TEMP_SYMLINK="${WWW_ROOT}/staging-temp-$$"
ln -sf "${DIST_NAME}" "${TEMP_SYMLINK}"
mv -fT "${TEMP_SYMLINK}" "${WWW_ROOT}/staging"

# Clean up old distribution
if [[ -n "${OLD_DIST}" && -d "${WWW_ROOT}/${OLD_DIST}" ]]; then
    echo "Removing old distribution: ${WWW_ROOT}/${OLD_DIST}"
    rm -rf "${WWW_ROOT}/${OLD_DIST}"
fi

echo
echo "Site deployed to: ${WWW_ROOT}/staging"
echo "Preview: http://watershop.local/staging/"
