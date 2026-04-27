#!/usr/bin/env bash
# measure-self.sh -- emit one JSON line of selfmon-process metrics.
#
# Usage: measure-self.sh
#
# Reads ${SELFMON_METRICS_DIR}/.sitegen-last.json (written by
# run-selfmon.sh after each sitegen invocation) and projects it
# into ${MEASURE_OUT_DIR}/_self.jsonl.  Column names match the
# semconv registry's metric_names:
#
#   sitegen.seconds          gauge          wallclock of last sitegen
#   sitegen_peak_rss.bytes   updowncounter  peak RSS of last sitegen
#
# The selfmon-process scope is implicit: the timeseries-join input
# that consumes this file uses scope "_self".
set -e

: "${MEASURE_OUT_DIR:?MEASURE_OUT_DIR must be set}"
: "${SELFMON_METRICS_DIR:?SELFMON_METRICS_DIR must be set}"
: "${POND:?POND must be set}"
mkdir -p "${MEASURE_OUT_DIR}"

PONDBIN=/usr/bin/pond

# ── timed read (DataFusion COUNT over kernel.jsonl) ───────────────
# Selfmon-pond-only probe: every selfmon instance ingests the host
# journal incl. the kernel ring buffer, so this query exercises a
# meaningful jsonlogs scan that scales with retained log volume.
# Other (non-selfmon) ponds don't have a comparable canonical path,
# which is why this lives under the `_self` scope rather than per-pond.
READ_SECONDS=0
if "${PONDBIN}" list /logs/journal/kernel.jsonl >/dev/null 2>&1; then
    READ_START=$(date +%s.%N)
    "${PONDBIN}" cat 'jsonlogs:///logs/journal/kernel.jsonl' \
        --sql 'SELECT COUNT(*) FROM source' --format=table \
        >/dev/null 2>&1 || true
    READ_END=$(date +%s.%N)
    READ_SECONDS=$(awk -v a="${READ_END}" -v b="${READ_START}" \
        'BEGIN{printf "%.3f", a-b}')
fi

SITEGEN_FILE="${SELFMON_METRICS_DIR}/.sitegen-last.json"

SITEGEN_SECONDS=0
SITEGEN_PEAK_RSS_BYTES=0
if [ -f "${SITEGEN_FILE}" ]; then
    SITEGEN_SECONDS=$(awk -F'[:,}]' '/seconds/ {
        for (i=1;i<=NF;i++) if ($i ~ /seconds/) { print $(i+1); exit } }' \
        "${SITEGEN_FILE}" | tr -d ' "')
    PEAK_MB=$(awk -F'[:,}]' '/peak_rss_mb/ {
        for (i=1;i<=NF;i++) if ($i ~ /peak_rss_mb/) { print $(i+1); exit } }' \
        "${SITEGEN_FILE}" | tr -d ' "')
    [ -z "${SITEGEN_SECONDS}" ] && SITEGEN_SECONDS=0
    [ -z "${PEAK_MB}" ] && PEAK_MB=0
    SITEGEN_PEAK_RSS_BYTES=$(awk -v m="${PEAK_MB}" \
        'BEGIN{printf "%.0f", m * 1048576}')
fi

TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)

printf '{"ts":"%s","read.seconds":%s,"sitegen.seconds":%s,"sitegen_peak_rss.bytes":%s}\n' \
    "${TS}" "${READ_SECONDS}" "${SITEGEN_SECONDS}" "${SITEGEN_PEAK_RSS_BYTES}" \
    >> "${MEASURE_OUT_DIR}/_self.jsonl"
