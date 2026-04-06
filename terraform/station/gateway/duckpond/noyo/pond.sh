#!/usr/bin/env bash
# pond.sh -- Podman wrapper for the noyo pond on the gateway.

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
DUCKPOND_DIR=$(dirname "$SCRIPTS")

source "$DUCKPOND_DIR/env.sh"

VOLUME=pond-noyo

podman run --pull=newer -ti --rm \
    -v "${VOLUME}:/pond" \
    -v "${SCRIPTS}:/root/config:ro" \
    -e POND=/pond \
    -e HYDRO_KEY_ID="${HYDRO_KEY_ID}" \
    -e HYDRO_KEY_VALUE="${HYDRO_KEY_VALUE}" \
    -e R2_ENDPOINT="${R2_ENDPOINT}" \
    -e R2_KEY="${R2_KEY}" \
    -e R2_SECRET="${R2_SECRET}" \
    "${IMAGE}" "$@"
