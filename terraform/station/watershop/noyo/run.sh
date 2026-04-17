#!/usr/bin/env bash
# run.sh -- Collect from HydroVu. Backup pushes automatically post-commit.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

${EXE} run /system/etc/20-hydrovu collect

echo
echo "=== Noyo staging run complete ==="
