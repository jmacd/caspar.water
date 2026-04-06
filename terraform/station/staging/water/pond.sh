#!/usr/bin/env bash
# pond.sh -- Podman wrapper for the water staging pond on watershop.
#
# Runs the duckpond CLI inside a container with:
#   - Named volume "pond-water-staging" for pond storage
#   - Bind mount of /home/data for logfile access
#   - Bind mount of the repo site/ directory for configs
#   - MinIO credentials for backup

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
REPO_ROOT=$(dirname "$(dirname "$(dirname "$STAGING_DIR")")")

source "$STAGING_DIR/env.sh"

VOLUME=pond-water-staging

podman run --pull=newer -ti --rm \
    -v "${VOLUME}:/pond" \
    -v "/home/data:/data:ro" \
    -v "${REPO_ROOT}/site:/root/site:ro" \
    -v "${SCRIPTS}:/root/config:ro" \
    -e POND=/pond \
    -e R2_ENDPOINT="${MINIO_ENDPOINT}" \
    -e R2_KEY="${MINIO_ACCESS_KEY}" \
    -e R2_SECRET="${MINIO_SECRET_KEY}" \
    "${IMAGE}" "$@"
