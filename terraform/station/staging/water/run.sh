#!/usr/bin/env bash
# run.sh -- Ingest new logfile data and push backup to MinIO.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

# Ingest new/updated files
${EXE} run /etc/ingest

# Push backup to MinIO
${EXE} run /system/run/1-backup push

echo
echo "=== Water staging run complete ==="
