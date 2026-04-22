#!/usr/bin/env bash
# cron.sh -- Import data, generate site, and deploy atomically.
#
# Designed to be run by a systemd timer. Pulls latest data from both
# R2 sources, generates the combined site, and deploys to nginx via
# atomic symlink swap.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

source "$SCRIPTS/env.sh"

WWW_ROOT="${WWW_ROOT:-/var/www/html}"
VOLUME=pond-site

# Pull latest data from R2
${EXE} run /system/etc/10-water pull
${EXE} run /system/etc/11-noyo pull

# Generate site to a temporary directory
TIMESTAMP=$(date -u +"%Y%m%d-%H%M%S")
DIST_NAME="casparwater-${TIMESTAMP}"
DIST_DIR="${WWW_ROOT}/${DIST_NAME}"
mkdir -p "${DIST_DIR}"

# Run sitegen with output mount
podman run --pull=newer -ti --rm \
    -v "${VOLUME}:/pond" \
    -v "${SCRIPTS}:/root/config:ro" \
    -v "${SCRIPTS}/site:/root/site:ro" \
    -v "${DIST_DIR}:/output" \
    -e POND=/pond \
    -e R2_ENDPOINT="${R2_ENDPOINT}" \
    -e R2_KEY="${R2_KEY}" \
    -e R2_SECRET="${R2_SECRET}" \
    "${IMAGE}" run /system/etc/90-sitegen build /output

# Read the current symlink target
OLD_DIST=""
if [ -L "${WWW_ROOT}/casparwater" ]; then
    OLD_DIST=$(readlink "${WWW_ROOT}/casparwater")
fi

# Atomic symlink swap
TEMP_SYMLINK="${WWW_ROOT}/casparwater-temp-$$"
ln -sf "${DIST_NAME}" "${TEMP_SYMLINK}"
mv -fT "${TEMP_SYMLINK}" "${WWW_ROOT}/casparwater"

# Clean up old distribution
if [ -n "${OLD_DIST}" ] && [ -d "${WWW_ROOT}/${OLD_DIST}" ]; then
    rm -rf "${WWW_ROOT}/${OLD_DIST}"
fi

echo "Site deployed: ${WWW_ROOT}/casparwater → ${DIST_NAME}"
