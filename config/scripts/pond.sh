#!/usr/bin/env bash
# pond.sh -- Podman wrapper for watertown instances.
#
# Usage: pond.sh <instance> [pond-args...]
#
# Selects the right image, volume, env, and mounts based on instance name.
# Architecture is detected from the host (uname -m).
set -e

INSTANCE=$1
shift

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
# Base dir is the watertown deployment root (parent of config/)
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
    IMAGE="ghcr.io/jmacd/watertown/watertown:latest-${ARCH}"
else
    IMAGE="ghcr.io/jmacd/watertown/watertown:prod-${ARCH}"
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

# Host-published monitoring output. The env file stores the host path for the
# wrapper; inside the container the factory always writes to /monitor.
if [ -n "${MONITOR_OUTPUT_DIR:-}" ]; then
    PODMAN_ARGS+=(
        -v "${MONITOR_OUTPUT_DIR}:/monitor"
        -e MONITOR_OUTPUT_DIR=/monitor
    )
fi

# Reporting is staging-only until its separate production approval. Ordinary
# commands see only saved reports; refresh gets one site's writable directory.
if [ -n "${ANALYTICS_REPORT_ROOT:-}" ] || [ "${1:-}" = "--analytics-report" ]; then
    if [[ ! "${INSTANCE}" =~ ^[a-zA-Z0-9_-]+-staging$ ]]; then
        echo "ERROR: Analytics report mounts are restricted to staging instances" >&2
        exit 1
    fi
    if [[ "${ANALYTICS_REPORT_ROOT:-}" != /* ]] || [ ! -d "${ANALYTICS_REPORT_ROOT:-}" ]; then
        echo "ERROR: ANALYTICS_REPORT_ROOT must be an existing absolute private directory" >&2
        exit 1
    fi
    if [ "${1:-}" = "--analytics-report" ]; then
        if [ "$#" -ne 1 ] || [ "${ANALYTICS_REPORT_ENABLED:-0}" != 1 ]; then
            echo "ERROR: --analytics-report requires explicit enablement and no extra arguments" >&2
            exit 1
        fi
        if [ "${POND_RUNTIME:-container}" != container ]; then
            echo "ERROR: Analytics report staging currently requires the container runtime" >&2
            exit 1
        fi
        if [ ! -d "${ANALYTICS_REPORT_ROOT}/${INSTANCE}" ] \
            || [[ "${ANALYTICS_CATALOG_DIR:-}" != /* ]] \
            || [ ! -d "${ANALYTICS_CATALOG_DIR:-}" ]; then
            echo "ERROR: Provision the private per-instance report directory and ANALYTICS_CATALOG_DIR first" >&2
            exit 1
        fi
        PODMAN_ARGS+=(
            -v "${ANALYTICS_REPORT_ROOT}/${INSTANCE}:/analytics-reports/${INSTANCE}"
            -v "${ANALYTICS_CATALOG_DIR}:/analytics-catalogs:ro"
        )
        set -- run /system/etc/90-sitegen analytics-report
    else
        PODMAN_ARGS+=(-v "${ANALYTICS_REPORT_ROOT}:/analytics-reports:ro")
    fi
fi

# Deliberate one-shot seed override. Keep it out of the persistent env file:
# callers must opt in on the specific command, and the wrapper forwards only
# this recognized control variable into the container.
if [ -n "${POND_IGNORE_LIMITS:-}" ]; then
    PODMAN_ARGS+=(-e "POND_IGNORE_LIMITS=${POND_IGNORE_LIMITS}")
fi

# Mount data directory if set (water, septic)
if [ -n "${DATA_DIR}" ]; then
    PODMAN_ARGS+=(-v "${DATA_DIR}:/data:ro")
fi

# Mount site build output directory if set
if [ -n "${SITE_BUILD_DIR}" ]; then
    PODMAN_ARGS+=(-v "${SITE_BUILD_DIR}:/www")
fi

exec podman "${PODMAN_ARGS[@]}" "${IMAGE}" "$@"
