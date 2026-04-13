#!/usr/bin/env bash
# run.sh -- Collect from HydroVu and push backup to MinIO.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

# Collect from HydroVu API
${EXE} run /system/etc/20-hydrovu collect

# Push backup to MinIO
${EXE} run /system/run/1-backup push

echo
echo "=== Noyo staging run complete ==="
