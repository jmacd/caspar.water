#!/usr/bin/env bash
# pond.sh -- Podman wrapper for the site staging pond on watershop.

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")

source "$STAGING_DIR/env.sh"

VOLUME=pond-site-staging

podman run --pull=newer -ti --rm \
    -v "${VOLUME}:/pond" \
    -v "${STAGING_DIR}/site-content:/root/site:ro" \
    -v "${SCRIPTS}:/root/config:ro" \
    -e POND=/pond \
    -e R2_ENDPOINT="${MINIO_ENDPOINT}" \
    -e R2_KEY="${MINIO_ACCESS_KEY}" \
    -e R2_SECRET="${MINIO_SECRET_KEY}" \
    "${IMAGE}" "$@"
