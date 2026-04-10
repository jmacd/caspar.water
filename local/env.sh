#!/bin/bash
# Shared environment for local site generation.
# Sourced by other scripts, not run directly.

LOCAL_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
export DUCKPOND_ROOT=$(cd "${LOCAL_DIR}/../duckpond" && pwd)
export CARGO="cargo run --release -p cmd --"

# Staging MinIO (cross-pond import source)
export R2_ENDPOINT="${R2_ENDPOINT:-http://watershop.casparwater.us:9000}"
export R2_KEY="${R2_KEY:-caspar}"
export R2_SECRET="${R2_SECRET:-watertown}"

export RUST_LOG=info
