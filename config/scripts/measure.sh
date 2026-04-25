#!/usr/bin/env bash
# measure.sh -- emit one JSON line of perf metrics per selfmon tick.
#
# Usage: measure.sh <instance>
#
# Reads ${BASE_DIR}/env/${INSTANCE}.env to pick up POND and
# SELFMON_METRICS_DIR.  Writes a single JSON object to
# ${SELFMON_METRICS_DIR}/metrics.jsonl, which the selfmon pond's
# `selfmon-metrics` logfile-ingest factory then mirrors into
# /measure/metrics.jsonl inside the pond.
#
# Captured per tick (after ingest+maintain has finished):
#   ts                  RFC3339 wall clock
#   instance            instance name
#   txn_seq             current pond commit sequence
#   parquet_files       count of *.parquet under $POND (compaction signal)
#   delta_log_files     count of *.json under */_delta_log/ (cleanup signal)
#   pond_size_bytes     du -sb $POND
#   list_seconds        wall-clock for `pond list /logs/journal/`
#   read_seconds        wall-clock for a representative jsonlogs query
#   peak_rss_mb         systemd-reported peak RSS of the *previous* run
#                       (extracted from journalctl --user -u <unit>)
set -e

INSTANCE=$1
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"

# shellcheck disable=SC1090
set -a
source "${ENV_FILE}"
set +a

: "${POND:?POND must be set}"
: "${SELFMON_METRICS_DIR:?SELFMON_METRICS_DIR must be set}"

case "${INSTANCE}" in
    *-staging) PONDBIN="/usr/local/bin/pond-selfmon-staging" ;;
    *-prod)    PONDBIN="/usr/local/bin/pond-selfmon-prod" ;;
    *)         echo "ERROR: bad instance '${INSTANCE}'"; exit 2 ;;
esac

mkdir -p "${SELFMON_METRICS_DIR}"

# --- timed `pond list` ---
LIST_START=$(date +%s.%N)
"${PONDBIN}" list /logs/journal/ >/dev/null 2>&1 || true
LIST_END=$(date +%s.%N)
LIST_SECONDS=$(awk -v a="${LIST_END}" -v b="${LIST_START}" 'BEGIN{printf "%.3f", a-b}')

# --- timed read (count of kernel entries via DataFusion) ---
READ_START=$(date +%s.%N)
"${PONDBIN}" cat 'jsonlogs:///logs/journal/kernel.jsonl' \
    --sql 'SELECT COUNT(*) FROM source' --format=table >/dev/null 2>&1 || true
READ_END=$(date +%s.%N)
READ_SECONDS=$(awk -v a="${READ_END}" -v b="${READ_START}" 'BEGIN{printf "%.3f", a-b}')

# --- structural counters ---
PARQUET_FILES=$(find "${POND}" -name '*.parquet' 2>/dev/null | wc -l | tr -d ' ')
DELTA_LOG_FILES=$(find "${POND}" -path '*_delta_log*' -name '*.json' 2>/dev/null | wc -l | tr -d ' ')
POND_SIZE_BYTES=$(du -sb "${POND}" 2>/dev/null | awk '{print $1}')
[ -z "${POND_SIZE_BYTES}" ] && POND_SIZE_BYTES=0

# --- current commit sequence ---
# `pond log --limit 1` outputs human-readable boxes; the most recent
# COMMITTED transaction line looks like:
#   +- Transaction 26 (write) -----------------------------
# The transaction number IS the txn_seq.
TXN_SEQ=$("${PONDBIN}" log --limit 1 2>/dev/null \
    | grep -oE 'Transaction [0-9]+' \
    | head -1 \
    | awk '{print $2}')
[ -z "${TXN_SEQ}" ] && TXN_SEQ=0

# --- peak RSS of previous run (from pond's own log line) ---
# Pond emits one "Peak memory usage: NN.NN MB" per CLI invocation to
# stderr (captured by journalctl).  We take the maximum across the
# *previous* tick's invocations to capture the worst step.  systemd's
# own "memory peak" reporting requires v254+; debian bookworm has 252.
PEAK_RSS_MB=$(journalctl --user -u "pond-selfmon@${INSTANCE}.service" \
    --no-pager -n 200 --since "5 minutes ago" 2>/dev/null \
    | grep -oE 'Peak memory usage: [0-9.]+ MB' \
    | awk '{if ($4+0 > max) max=$4+0} END{printf "%.2f", (max==""?0:max)}')

TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Single JSON line, append.
printf '{"ts":"%s","instance":"%s","txn_seq":%s,"parquet_files":%s,"delta_log_files":%s,"pond_size_bytes":%s,"list_seconds":%s,"read_seconds":%s,"peak_rss_mb":%s}\n' \
    "${TS}" "${INSTANCE}" \
    "${TXN_SEQ}" "${PARQUET_FILES}" "${DELTA_LOG_FILES}" "${POND_SIZE_BYTES}" \
    "${LIST_SECONDS}" "${READ_SECONDS}" "${PEAK_RSS_MB}" \
    >> "${SELFMON_METRICS_DIR}/metrics.jsonl"
