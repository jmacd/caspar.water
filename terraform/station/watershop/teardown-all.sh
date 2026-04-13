#!/usr/bin/env bash
# teardown-all.sh -- Remove all staging ponds and data.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

"${SCRIPTS}/water/teardown.sh"
"${SCRIPTS}/noyo/teardown.sh"
"${SCRIPTS}/site/teardown.sh"

# Remove generated site
rm -rf "${SCRIPTS}/dist"

echo
echo "=== All staging teardown complete ==="
