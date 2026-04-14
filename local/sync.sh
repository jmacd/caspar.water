#!/usr/bin/env bash
# sync.sh -- Pull data from water, noyo, and septic staging ponds via MinIO.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

echo "--- Importing water data ---"
${EXE} run /system/etc/10-water pull

echo ""
echo "--- Importing noyo data ---"
${EXE} run /system/etc/11-noyo pull

echo ""
echo "--- Importing septic data ---"
${EXE} run /system/etc/12-septic pull

echo ""
echo "=== Import complete ==="
echo "Next: ./generate.sh  # build the site"
