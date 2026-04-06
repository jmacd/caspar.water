#!/usr/bin/env bash
# pond.sh -- Podman wrapper for the noyo staging pond on watershop.
#
# Runs the duckpond CLI inside a container with:
#   - Named volume "pond-noyo-staging" for pond storage
#   - Bind mount of the noyo config directory
#   - HydroVu and MinIO credentials

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")

source "$STAGING_DIR/env.sh"

VOLUME=pond-noyo-staging

podman run --pull=newer -ti --rm \
    -v "${VOLUME}:/pond" \
    -v "${SCRIPTS}:/root/config:ro" \
    -e POND=/pond \
    -e HYDRO_KEY_ID="${HYDRO_KEY_ID}" \
    -e HYDRO_KEY_VALUE="${HYDRO_KEY_VALUE}" \
    -e R2_ENDPOINT="${MINIO_ENDPOINT}" \
    -e R2_KEY="${MINIO_ACCESS_KEY}" \
    -e R2_SECRET="${MINIO_SECRET_KEY}" \
    "${IMAGE}" "$@"
