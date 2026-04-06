#!/usr/bin/env bash
# import.sh -- Pull data from water and noyo staging ponds via MinIO.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

echo "--- Importing water data ---"
${EXE} run /system/etc/10-water pull

echo ""
echo "--- Importing noyo data ---"
${EXE} run /system/etc/11-noyo pull

echo ""
echo "=== Import complete ==="
echo "Next: ./site/generate.sh  # build the combined site"
