#!/usr/bin/env bash
# run.sh -- Ingest new logfile data. Backup pushes automatically post-commit.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

${EXE} run /etc/ingest
