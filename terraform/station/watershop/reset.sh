#!/usr/bin/env bash
# reset.sh -- Reset a duckpond staging instance (run locally).
#
# Usage: ./reset.sh <instance>
#   e.g., ./reset.sh water-staging
#
# This erases the S3 bucket from your workstation (no SSH needed),
# then uses terraform to re-deploy the instance.
set -e

INSTANCE=$1
if [ -z "${INSTANCE}" ]; then
    echo "Usage: ./reset.sh <instance>"
    echo ""
    echo "Instances: water-staging, noyo-staging, septic-staging, site-staging"
    exit 1
fi

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/../../.." && pwd)
POND="${REPO_ROOT}/duckpond/target/release/pond"

if [ ! -x "${POND}" ]; then
    echo "Building pond..."
    (cd "${REPO_ROOT}/duckpond" && cargo build --release -p cmd)
fi

# Load terraform variables for S3 credentials
if [ -f "${SCRIPTS}/terraform.tfvars" ]; then
    # Extract variables we need
    eval "$(grep -E '^(minio_|r2_)' "${SCRIPTS}/terraform.tfvars" | sed 's/ *= */=/' | sed 's/"//g')"
fi

TYPE="${INSTANCE%-staging}"
TYPE="${TYPE%-prod}"

# Determine S3 config based on staging vs prod
if [[ "${INSTANCE}" == *-staging ]]; then
    # Use external MinIO address (tfvars has localhost for on-machine use)
    ENDPOINT="http://watershop.casparwater.us:9000"
    ACCESS_KEY="${minio_access_key}"
    SECRET_KEY="${minio_secret_key}"
    ALLOW_HTTP="--allow-http"
else
    ENDPOINT="${r2_endpoint}"
    ACCESS_KEY="${r2_access_key}"
    SECRET_KEY="${r2_secret_key}"
    ALLOW_HTTP=""
fi

# Map instance to bucket name
case "${TYPE}" in
    water)  BUCKET="s3://water-staging" ;;
    noyo)   BUCKET="s3://noyo-staging" ;;
    septic) BUCKET="s3://septic-staging" ;;
    site)   BUCKET="" ;;  # site pond has no backup bucket
    *)      echo "ERROR: Unknown type '${TYPE}'"; exit 1 ;;
esac

# Erase the S3 bucket (locally, no SSH)
if [ -n "${BUCKET}" ] && [ -n "${ACCESS_KEY}" ]; then
    echo "Erasing ${BUCKET}..."
    "${POND}" emergency erase-bucket "${BUCKET}" \
        --endpoint "${ENDPOINT}" \
        --access-key "${ACCESS_KEY}" \
        --secret-key "${SECRET_KEY}" \
        ${ALLOW_HTTP} \
        --dangerous
fi

echo ""
echo "Bucket erased. Now run terraform to re-deploy:"
echo "  cd ${SCRIPTS}"
echo "  terraform apply"
