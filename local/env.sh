#!/bin/bash
# Shared environment for local site generation.
# Sourced by other scripts, not run directly.

LOCAL_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${LOCAL_DIR}/.." && pwd)
export WATERTOWN_ROOT="${REPO_ROOT}/watertown"
export CARGO="cargo run -p cmd --"

# Git content source (local repo, current branch)
export GIT_REF="${GIT_REF:-$(git -C "${REPO_ROOT}" rev-parse --abbrev-ref HEAD)}"

# Staging MinIO (cross-pond import source)
export S3_ENDPOINT="${S3_ENDPOINT:-http://watershop.casparwater.us:9000}"
export S3_REGION="${S3_REGION:-us-east-1}"
export S3_ACCESS_KEY="${S3_ACCESS_KEY:-caspar}"
export S3_SECRET_KEY="${S3_SECRET_KEY:-watertown}"
export S3_ALLOW_HTTP=true

# Staging bucket URLs
export WATER_S3_URL="${WATER_S3_URL:-s3://water-staging}"
export NOYO_S3_URL="${NOYO_S3_URL:-s3://noyo-staging}"
export SEPTIC_S3_URL="${SEPTIC_S3_URL:-s3://septic-staging}"

export RUST_LOG=info
