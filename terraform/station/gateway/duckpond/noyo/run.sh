#!/usr/bin/env bash
# run.sh -- Collect from HydroVu and push backup to R2.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

${EXE} run /system/etc/20-hydrovu collect
${EXE} run /system/run/1-backup push
