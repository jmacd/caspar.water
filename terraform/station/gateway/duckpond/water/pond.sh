#!/usr/bin/env bash
# pond.sh -- Podman wrapper for the water pond on the gateway.

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
DUCKPOND_DIR=$(dirname "$SCRIPTS")

source "$DUCKPOND_DIR/env.sh"

VOLUME=pond-water

podman run --pull=newer -ti --rm \
    -v "${VOLUME}:/pond" \
    -v "/home/data:/data:ro" \
    -v "${SCRIPTS}:/root/config:ro" \
    -v "${DUCKPOND_DIR}/site:/root/site:ro" \
    -e POND=/pond \
    -e R2_ENDPOINT="${R2_ENDPOINT}" \
    -e R2_KEY="${R2_KEY}" \
    -e R2_SECRET="${R2_SECRET}" \
    "${IMAGE}" "$@"
