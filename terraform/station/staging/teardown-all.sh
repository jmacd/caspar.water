#!/usr/bin/env bash
# teardown-all.sh -- Remove all staging ponds and data.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

"${SCRIPTS}/water/teardown.sh"
"${SCRIPTS}/noyo/teardown.sh"
"${SCRIPTS}/site/teardown.sh"

# Clean up MinIO staging buckets
mc rm --recursive --force local/water-staging 2>/dev/null || true
mc rm --recursive --force local/noyo-staging 2>/dev/null || true

# Remove generated site
rm -rf "${SCRIPTS}/dist"

echo
echo "=== All staging teardown complete ==="
