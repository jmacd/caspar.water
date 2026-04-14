#!/usr/bin/env bash
# setup-minio.sh -- Create MinIO buckets for staging instances.
#
# Uses the mc (MinIO Client) CLI. Assumes MinIO is running locally.
# Idempotent — skips buckets that already exist.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
source "${SCRIPTS}/env/water-staging.env"

ALIAS="local"

# Configure mc alias for local MinIO
mc alias set "${ALIAS}" "${S3_ENDPOINT}" "${S3_ACCESS_KEY}" "${S3_SECRET_KEY}" 2>/dev/null || true

# Create staging buckets
for BUCKET in noyo-staging water-staging septic-staging; do
    mc mb --ignore-existing "${ALIAS}/${BUCKET}"
done

echo "=== MinIO buckets ready ==="
