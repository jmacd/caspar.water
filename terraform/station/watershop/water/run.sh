#!/usr/bin/env bash
# run.sh -- Ingest new logfile data and push backup to MinIO.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

# Ingest new/updated files.
# The active logfile may be rotated by the live collector during
# ingestion, causing a transient "prefix verification failed" error.
# Retry up to 3 times to handle this race.
for attempt in 1 2 3; do
    if ${EXE} run /etc/ingest; then
        break
    fi
    echo "Ingest attempt $attempt failed (file rotation?), retrying..."
    sleep 2
done

echo
echo "=== Water staging run complete ==="
