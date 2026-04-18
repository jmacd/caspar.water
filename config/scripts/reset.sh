#!/usr/bin/env bash
# reset.sh -- Erase S3 bucket for a duckpond instance (run locally).
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

# Find the pond binary
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/../.." && pwd)
POND="${REPO_ROOT}/duckpond/target/release/pond"

if [ ! -x "${POND}" ]; then
    echo "Building pond..."
    (cd "${REPO_ROOT}/duckpond" && cargo build --release -p cmd)
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

    # When running locally, replace localhost with the actual host
    ENDPOINT="${S3_ENDPOINT}"
    if [[ "${ENDPOINT}" == *"localhost"* ]]; then
        ENDPOINT="${ENDPOINT//localhost/${DUCKPOND_S3_HOST:-watershop.casparwater.us}}"
    fi

    echo "Erasing ${S3_URL} via ${ENDPOINT}..."

    ALLOW_HTTP_FLAG=""
    if [ "${S3_ALLOW_HTTP}" = "true" ]; then
        ALLOW_HTTP_FLAG="--allow-http"
    fi

    "${POND}" emergency erase-bucket "${S3_URL}" \
        --endpoint "${ENDPOINT}" \
        --region "${S3_REGION}" \
        --access-key "${S3_ACCESS_KEY}" \
        --secret-key "${S3_SECRET_KEY}" \
        ${ALLOW_HTTP_FLAG} \
        --dangerous \
        || echo "  (bucket empty or not found)"
done
