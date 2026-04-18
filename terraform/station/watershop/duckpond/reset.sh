#!/usr/bin/env bash
# reset.sh -- Wipe a duckpond instance and re-initialize from config.
#
# Usage: reset.sh <instance>
#
# Destroys the podman volume, erases the S3 backup bucket,
# re-creates the pond, and applies the canonical config.
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

# Erase the S3 backup bucket (if S3_URL is set)
if [ -n "${S3_URL}" ]; then
    ${EXE} "${INSTANCE}" emergency erase-bucket "${S3_URL}" \
        --endpoint "${S3_ENDPOINT}" \
        --region "${S3_REGION}" \
        --access-key "${S3_ACCESS_KEY}" \
        --secret-key "${S3_SECRET_KEY}" \
        ${S3_ALLOW_HTTP:+--allow-http} \
        --dangerous \
        || echo "  (S3 erase skipped or bucket empty)"
fi

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
