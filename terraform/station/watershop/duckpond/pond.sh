#!/usr/bin/env bash
# pond.sh -- Podman wrapper for duckpond instances on watershop.
#
# Usage: pond.sh <instance> [pond-args...]
#
# Selects the right image, volume, env, and mounts based on instance name.
set -e

INSTANCE=$1
shift

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
ENV_FILE="${SCRIPTS}/env/${INSTANCE}.env"

if [ ! -f "${ENV_FILE}" ]; then
    echo "ERROR: No env file for instance '${INSTANCE}' at ${ENV_FILE}"
    exit 1
fi

source "${ENV_FILE}"

# Image selection: staging uses latest, production uses pinned version
VERSION=$(cat "${SCRIPTS}/config/DUCKPOND_VERSION")
if [[ "${INSTANCE}" == *-staging ]]; then
    IMAGE="ghcr.io/jmacd/duckpond/duckpond:latest-arm64"
    PULL="--pull=newer"
else
    IMAGE="ghcr.io/jmacd/duckpond/duckpond:${VERSION}-arm64"
    PULL="--pull=missing"
fi

# Volume name from env file (POND_VOLUME)
VOLUME="${POND_VOLUME:-pond-${INSTANCE}}"

# Base podman args
PODMAN_ARGS=(
    run ${PULL} --rm
    --network=host
    --env-file "${ENV_FILE}"
    -e POND=/pond
    -v "${VOLUME}:/pond"
    -v "${SCRIPTS}/config:/config:ro"
    -v "${SCRIPTS}/site:/site:ro"
)

# Mount data directory if set (water, septic)
if [ -n "${DATA_DIR}" ]; then
    PODMAN_ARGS+=(-v "${DATA_DIR}:/data:ro")
fi

# Mount noyo archive if set
if [ -n "${NOYO_ARCHIVE_DIR}" ]; then
    PODMAN_ARGS+=(-v "${NOYO_ARCHIVE_DIR}:${NOYO_ARCHIVE_DIR}:ro")
fi

# Mount site build output directory if set
if [ -n "${SITE_BUILD_DIR}" ]; then
    PODMAN_ARGS+=(-v "${SITE_BUILD_DIR}:/www")
fi

exec podman "${PODMAN_ARGS[@]}" "${IMAGE}" "$@"
