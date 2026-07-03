#!/usr/bin/env bash
# pond.sh -- Podman wrapper for duckpond instances.
#
# Usage: pond.sh <instance> [pond-args...]
#
# Selects the right image, volume, env, and mounts based on instance name.
# Architecture is detected from the host (uname -m).
set -e

INSTANCE=$1
shift

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
# Base dir is the duckpond deployment root (parent of config/)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"

if [ ! -f "${ENV_FILE}" ]; then
    echo "ERROR: No env file for instance '${INSTANCE}' at ${ENV_FILE}"
    exit 1
fi

source "${ENV_FILE}"

# Detect architecture
case "$(uname -m)" in
    aarch64|arm64) ARCH="arm64" ;;
    x86_64|amd64)  ARCH="amd64" ;;
    *)             ARCH="$(uname -m)" ;;
esac

# Image selection: staging uses latest, prod uses the promoted image.
# Both are mutable tags.  run.sh refreshes the image once per timer tick via
# `pond.sh <instance> --pull-image`; the individual `podman run` invocations
# below use --pull=missing so they reuse the already-pulled local image instead
# of each re-checking the registry (which GHCR counts as a download).
if [[ "${INSTANCE}" == *-staging ]]; then
    IMAGE="ghcr.io/jmacd/duckpond/duckpond:latest-${ARCH}"
else
    IMAGE="ghcr.io/jmacd/duckpond/duckpond:prod-${ARCH}"
fi

# One-shot image refresh: fetch the mutable tag once (a single registry check)
# so the per-command runs in this tick can use --pull=missing.
if [ "${1:-}" = "--pull-image" ]; then
    exec podman pull "${IMAGE}"
fi

PULL="--pull=missing"

# Volume name from env file (POND_VOLUME)
VOLUME="${POND_VOLUME:-pond-${INSTANCE}}"

# Base podman args
PODMAN_ARGS=(
    run ${PULL} --rm
    --network=host
    --env-file "${ENV_FILE}"
    -e POND=/pond
    -v "${VOLUME}:/pond"
    -v "${BASE_DIR}/config:/config:ro"
)

# Mount data directory if set (water, septic)
if [ -n "${DATA_DIR}" ]; then
    PODMAN_ARGS+=(-v "${DATA_DIR}:/data:ro")
fi

# Mount site build output directory if set
if [ -n "${SITE_BUILD_DIR}" ]; then
    PODMAN_ARGS+=(-v "${SITE_BUILD_DIR}:/www")
fi

exec podman "${PODMAN_ARGS[@]}" "${IMAGE}" "$@"
