#!/usr/bin/env bash
# pond.sh -- Podman wrapper for the site pond on the cloud machine.

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

source "$SCRIPTS/env.sh"

VOLUME=pond-site

podman run --pull=newer -ti --rm \
    -v "${VOLUME}:/pond" \
    -v "${SCRIPTS}:/root/config:ro" \
    -v "${SCRIPTS}/site:/root/site:ro" \
    -e POND=/pond \
    -e R2_ENDPOINT="${R2_ENDPOINT}" \
    -e R2_KEY="${R2_KEY}" \
    -e R2_SECRET="${R2_SECRET}" \
    "${IMAGE}" "$@"
