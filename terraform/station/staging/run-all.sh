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
echo "Preview: cd ${SCRIPTS}/dist && python3 -m http.server 4180"
