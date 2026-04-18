#!/usr/bin/env bash
# reset.sh -- Reset duckpond staging instances (run locally).
#
# Usage: ./reset.sh <instance> [instance...]
#   e.g., ./reset.sh water-staging
#         ./reset.sh water-staging noyo-staging septic-staging
#
# Erases S3 buckets from your workstation, then runs terraform
# apply to wipe volumes and re-initialize on the remote machine.
set -e

if [ $# -eq 0 ]; then
    echo "Usage: ./reset.sh <instance> [instance...]"
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

# Load S3 credentials from terraform.tfvars
if [ -f "${SCRIPTS}/terraform.tfvars" ]; then
    eval "$(grep -E '^(minio_|r2_)' "${SCRIPTS}/terraform.tfvars" | sed 's/ *= */=/' | sed 's/"//g')"
fi

# Erase S3 buckets for each instance
for INSTANCE in "$@"; do
    TYPE="${INSTANCE%-staging}"
    TYPE="${TYPE%-prod}"

    # Map type to bucket
    case "${TYPE}" in
        water)  BUCKET="s3://water-staging" ;;
        noyo)   BUCKET="s3://noyo-staging" ;;
        septic) BUCKET="s3://septic-staging" ;;
        site)   BUCKET="" ;;
        *)      echo "ERROR: Unknown type '${TYPE}'"; exit 1 ;;
    esac

    if [ -n "${BUCKET}" ]; then
        echo "Erasing ${BUCKET}..."
        "${POND}" emergency erase-bucket "${BUCKET}" \
            --endpoint "http://watershop.casparwater.us:9000" \
            --access-key "${minio_access_key}" \
            --secret-key "${minio_secret_key}" \
            --allow-http \
            --dangerous \
            || echo "  (bucket empty or not found)"
    fi
done

# Build terraform reset_instances list
RESET_LIST=$(printf '"%s",' "$@" | sed 's/,$//')

echo ""
echo "Running terraform apply with reset..."
cd "${SCRIPTS}"
terraform apply -var "reset_instances=[${RESET_LIST}]"
