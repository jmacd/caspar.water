#!/usr/bin/env bash
# run-all.sh -- Run the full staging pipeline end-to-end.
#
# Executes: water ingest → noyo collect → site import + generate
#
# Prerequisites:
#   - All three ponds set up (./water/setup.sh, ./noyo/setup.sh, ./site/setup.sh)
#   - HydroVu credentials in ~/.bashrc.private
#   - MinIO running at localhost:9000
#   - Logfiles available at /home/data/
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

source "$SCRIPTS/env.sh"

# Verify pond UUIDs match between source ponds and site import configs.
# If a source pond was re-created (new UUID), site/setup.sh must be re-run.
pond_id() {
    cd "${DUCKPOND_ROOT}" && POND="$1" ${CARGO} config 2>/dev/null | grep "Pond ID" | awk '{print $NF}'
}
import_pond_id() {
    cd "${DUCKPOND_ROOT}" && POND="${SCRIPTS}/site/pond" ${CARGO} cat "$1" 2>/dev/null | grep -oP 'pond-\K[0-9a-f-]+'
}

WATER_ID=$(pond_id "${SCRIPTS}/water/pond")
NOYO_ID=$(pond_id "${SCRIPTS}/noyo/pond")
WATER_IMPORT_ID=$(import_pond_id /system/etc/10-water)
NOYO_IMPORT_ID=$(import_pond_id /system/etc/11-noyo)

MISMATCH=0
if [ "${WATER_ID}" != "${WATER_IMPORT_ID}" ]; then
    echo "ERROR: Water pond UUID mismatch"
    echo "  water/pond:  ${WATER_ID}"
    echo "  site import: ${WATER_IMPORT_ID}"
    MISMATCH=1
fi
if [ "${NOYO_ID}" != "${NOYO_IMPORT_ID}" ]; then
    echo "ERROR: Noyo pond UUID mismatch"
    echo "  noyo/pond:   ${NOYO_ID}"
    echo "  site import: ${NOYO_IMPORT_ID}"
    MISMATCH=1
fi
if [ "${MISMATCH}" -eq 1 ]; then
    echo ""
    echo "Re-run site/setup.sh to pick up new pond UUIDs, then try again."
    exit 1
fi

echo "============================="
echo "=== Water: ingest + backup ==="
echo "============================="
"${SCRIPTS}/water/run.sh"

echo ""
echo "============================"
echo "=== Noyo: collect + backup ==="
echo "============================"
"${SCRIPTS}/noyo/run.sh"

echo ""
echo "=========================="
echo "=== Site: import + generate ==="
echo "=========================="
"${SCRIPTS}/site/import.sh"
"${SCRIPTS}/site/generate.sh"

echo ""
echo "============================="
echo "=== Full pipeline complete ==="
echo "============================="
echo "Preview: http://watershop.local/staging/"
