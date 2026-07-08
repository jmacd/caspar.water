#!/usr/bin/env bash
# reset.sh -- Erase S3 bucket for a watertown instance (run locally).
#
# Usage: reset.sh <env-file> [env-file...]
#   e.g., reset.sh env/water-staging.env
#
# Reads S3 credentials from the env file and erases the bucket.
# Then pass -var reset_instances=[...] to terraform apply to
# wipe volumes and re-initialize.
set -e

if [ $# -eq 0 ]; then
    echo "Usage: reset.sh <env-file> [env-file...]"
    exit 1
fi

# Find the pond binary.  Use POND_BIN (not POND) because the env files
# sourced below set POND=<pond data dir> and would clobber it.
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/../.." && pwd)
POND_BIN="${REPO_ROOT}/watertown/target/release/pond"

if [ ! -x "${POND_BIN}" ]; then
    echo "Building pond..."
    (cd "${REPO_ROOT}/watertown" && cargo build --release -p cmd)
fi

for ENV_FILE in "$@"; do
    if [ ! -f "${ENV_FILE}" ]; then
        echo "ERROR: env file not found: ${ENV_FILE}"
        exit 1
    fi

    source "${ENV_FILE}"

    if [ -z "${S3_URL}" ]; then
        echo "Skipping $(basename "${ENV_FILE}") (no S3_URL)"
        continue
    fi

    echo "Erasing ${S3_URL} via ${S3_ENDPOINT}..."

    ALLOW_HTTP_FLAG=""
    if [ "${S3_ALLOW_HTTP}" = "true" ]; then
        ALLOW_HTTP_FLAG="--allow-http"
    fi

    "${POND_BIN}" emergency erase-bucket "${S3_URL}" \
        --endpoint "${S3_ENDPOINT}" \
        --region "${S3_REGION}" \
        --access-key "${S3_ACCESS_KEY}" \
        --secret-key "${S3_SECRET_KEY}" \
        ${ALLOW_HTTP_FLAG} \
        --dangerous \
        || echo "  (bucket empty or not found)"
done
