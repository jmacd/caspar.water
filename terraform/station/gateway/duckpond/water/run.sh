#!/usr/bin/env bash
# run.sh -- Ingest new logfile data and push backup to R2.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

${EXE} run /etc/ingest
${EXE} run /system/run/1-backup push
