#!/bin/bash
# Shared environment for staging on watershop.casparwater.us.
# Sourced by other scripts, not run directly.

# Duckpond workspace root (build with: cd $DUCKPOND_ROOT && cargo build --release --bin pond)
STAGING_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
export DUCKPOND_ROOT=$(cd "${STAGING_DIR}/../../.." && cd duckpond && pwd)
export CARGO="cargo run --release -p cmd --"

# MinIO (local S3 for staging backup/import)
export R2_ENDPOINT=http://localhost:9000
export R2_KEY=caspar
export R2_SECRET=watertown

# Water data (NFS mount from linux.local)
export WATER_DATA_DIR=/home/shared/water/archive/data

export RUST_LOG=info

# HydroVu credentials
if [ -f ~/.bashrc.private ]; then
    source ~/.bashrc.private
fi
