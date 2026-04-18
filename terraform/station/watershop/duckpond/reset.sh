#!/usr/bin/env bash
# reset.sh -- Wipe a duckpond instance and re-initialize from config.
#
# Usage: reset.sh <instance>
#
# Destroys the podman volume, re-creates the pond, and applies
# the canonical config. After reset, the first backup push will
# fail if the S3 bucket still contains data from the old pond.
#
# To clean a MinIO staging bucket:
#   mc rb --force local/<bucket> && mc mb local/<bucket>
#
# Cloud buckets (R2, etc.) should be cleaned via their web console.
set -ex

INSTANCE=$1
if [ -z "${INSTANCE}" ]; then
    echo "Usage: reset.sh <instance>"
    echo "  e.g., reset.sh water-staging"
    exit 1
fi

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"
ENV_FILE="${SCRIPTS}/env/${INSTANCE}.env"

if [ ! -f "${ENV_FILE}" ]; then
    echo "ERROR: No env file for instance '${INSTANCE}'"
    exit 1
fi

source "${ENV_FILE}"

VOLUME="${POND_VOLUME:-pond-${INSTANCE}}"
TYPE="${INSTANCE%-staging}"
TYPE="${TYPE%-prod}"

echo "=== Resetting ${INSTANCE} (type: ${TYPE}, volume: ${VOLUME}) ==="

# Stop the timer during reset
systemctl --user stop "pond@${INSTANCE}.timer" 2>/dev/null || true

# Wipe the podman volume
if podman volume exists "${VOLUME}" 2>/dev/null; then
    podman volume rm "${VOLUME}"
fi

# Re-initialize and apply config
${EXE} "${INSTANCE}" init
${EXE} "${INSTANCE}" apply -f "/config/${TYPE}.yaml"

# Restart the timer
systemctl --user start "pond@${INSTANCE}.timer" 2>/dev/null || true

echo ""
echo "=== ${INSTANCE} reset complete ==="
if [ -n "${S3_URL}" ]; then
    BUCKET="${S3_URL#s3://}"
    echo "NOTE: Clean the S3 bucket '${BUCKET}' before the first backup push."
    echo "  MinIO: mc rb --force local/${BUCKET} && mc mb local/${BUCKET}"
fi
