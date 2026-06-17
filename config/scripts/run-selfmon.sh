#!/usr/bin/env bash
# run-selfmon.sh -- run the native selfmon pond (no podman).
#
# Usage: run-selfmon.sh <instance>     (e.g. watershop-selfmon)
#
# Reads ${BASE_DIR}/env/${INSTANCE}.env for POND, S3_*, etc.
# Invokes the host-installed `/usr/bin/pond` directly.
set -e

INSTANCE=$1
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"

if [ ! -f "${ENV_FILE}" ]; then
    echo "ERROR: No env file for instance '${INSTANCE}' at ${ENV_FILE}"
    exit 1
fi

# shellcheck disable=SC1090
set -a
source "${ENV_FILE}"
set +a

PONDBIN="/usr/bin/pond"
if [ ! -x "${PONDBIN}" ]; then
    echo "ERROR: ${PONDBIN} not installed; run tools/build-on-watershop.sh"
    exit 1
fi

# POND must be set per-instance (e.g. /home/jmacd/pond-watershop-selfmon).
: "${POND:?POND must be set in ${ENV_FILE}}"
export POND

# Bootstrap: on first run there is no journal cursor, and journalctl
# would dump the entire host history at once, blowing the binary's
# 3GiB allocation cap.  Seed the cursor at "now" so we ingest only
# going-forward entries.  This is idempotent: only seeds if absent.
if ! "${PONDBIN}" cat /logs/journal/.journal-cursor >/dev/null 2>&1; then
    echo "Seeding journal cursor at current head (first run bootstrap)..."
    CURSOR_TMP=$(mktemp)
    journalctl -n 0 --show-cursor --no-pager 2>/dev/null \
        | sed -n 's/^-- cursor: //p' > "${CURSOR_TMP}"
    if [ -s "${CURSOR_TMP}" ]; then
        "${PONDBIN}" copy "host:///${CURSOR_TMP}" /logs/journal/.journal-cursor
    else
        echo "WARNING: failed to obtain current journal cursor; first ingest may OOM"
    fi
    rm -f "${CURSOR_TMP}"
fi

# Per-tick work, in dependency order:
#
#   1. measure  -- write fresh per-pond jsonl + _self.jsonl into
#                  ${MEASURE_OUT_DIR}.  Done FIRST so subsequent
#                  ingest picks up THIS tick's data and sitegen
#                  renders it the same tick (vs the legacy ordering
#                  which was always one tick stale).  Probes only
#                  read pond state, never write -- safe before
#                  ingest/maintain.
#   2. ingest   -- journal, caddy access, per-pond perf jsonl.
#   3. sync     -- copy site templates from host into pond.
#   4. maintain -- delta-log checkpoint / cleanup.
#   5. sitegen  -- render dashboard from /derived/perf into /var/www.

export MEASURE_OUT_DIR="${SELFMON_METRICS_DIR}"
mkdir -p "${MEASURE_OUT_DIR}"

# ── Selfmon-process scope: write _self.jsonl ──────────────────────
# Inlined (was measure-self.sh).  Two metrics:
#   read.seconds        -- timed COUNT(*) over kernel.jsonl, a
#                          jsonlogs scan that grows with retained
#                          log volume.  Selfmon-only: other ponds
#                          don't have a comparable canonical path.
#   sitegen.seconds &   -- pulled from the *prior* tick's
#   sitegen_peak_rss.bytes  .sitegen-last.json (this tick's sitegen
#                          hasn't run yet).
{
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
} || echo "WARNING: _self measurement failed" >&2

# One probe per pond defined under ${BASE_DIR}/env/.
for envf in "${BASE_DIR}/env"/*.env; do
    [ -f "$envf" ] || continue
    pond_name=$(basename "$envf" .env)
    "${SCRIPTS}/measure-pond.sh" "${pond_name}" || \
        echo "WARNING: measure-pond.sh ${pond_name} failed" >&2
done

# Ingest external sources.
"${PONDBIN}" run /system/etc/journal push
"${PONDBIN}" run /system/etc/caddy-access push

# Ingest per-pond perf jsonl.  One mknod per pond + _self because
# logfile-ingest selects exactly ONE active file per mknod.  Mknods
# match the env file enumeration above one-for-one, plus _self.
"${PONDBIN}" run /system/etc/measure/_self push 2>/dev/null || true
for envf in "${BASE_DIR}/env"/*.env; do
    [ -f "$envf" ] || continue
    pond_name=$(basename "$envf" .env)
    "${PONDBIN}" run "/system/etc/measure/${pond_name}" push \
        2>/dev/null || true
done

# Sync templates (host -> pond).  /system/site is created by the yaml
# mkdir; we copy each template file individually because `pond copy`
# operates per-file.
TEMPLATE_SRC="${BASE_DIR}/config/selfmon/site"
if [ -d "${TEMPLATE_SRC}" ]; then
    for f in "${TEMPLATE_SRC}"/*.md; do
        [ -f "$f" ] || continue
        "${PONDBIN}" copy "host://${f}" "/system/site/$(basename "$f")"
    done
fi

# Maintain: checkpoint + vacuum every tick (gated internally),
# compact weekly on Sunday mornings (3am UTC ~ 8pm PST Saturday).
HOUR=$(date -u +%H)
DAY=$(date -u +%u)  # 1=Mon, 7=Sun
if [ "${DAY}" = "7" ] && [ "${HOUR}" = "03" ]; then
    echo "Running weekly compact (Sunday 3am UTC)..."
    "${PONDBIN}" maintain --compact
else
    "${PONDBIN}" maintain
fi

# Sitegen render, with wall-clock timing.  Output dir is owned by
# ${USER} (provisioned by terraform) and served by Caddy at /selfmon/.
# Vendor assets (DuckDB-WASM, Plot, D3) are installed at
# /usr/share/duckpond/vendor by the duckpond .deb (see
# install-duckpond.sh), which is where sitegen's find_vendor_dir()
# searches for them.
SITE_OUT="/var/www/selfmon/${INSTANCE}"
SITEGEN_TIMING="${SELFMON_METRICS_DIR}/.sitegen-last.json"

SG_START=$(date +%s.%N)
SG_LOG=$(mktemp)
if "${PONDBIN}" run /system/etc/sitegen build "${SITE_OUT}" >"${SG_LOG}" 2>&1; then
    SG_STATUS=ok
else
    SG_STATUS=fail
fi
SG_END=$(date +%s.%N)
SG_SECONDS=$(awk -v a="${SG_END}" -v b="${SG_START}" 'BEGIN{printf "%.3f", a-b}')
SG_PEAK_MB=$(grep -oE 'Peak memory usage: [0-9.]+ MB' "${SG_LOG}" \
    | awk '{if ($4+0 > max) max=$4+0} END{printf "%.2f", (max==""?0:max)}')
printf '{"status":"%s","seconds":%s,"peak_rss_mb":%s}\n' \
    "${SG_STATUS}" "${SG_SECONDS}" "${SG_PEAK_MB}" > "${SITEGEN_TIMING}"
[ "${SG_STATUS}" = fail ] && cat "${SG_LOG}" >&2
rm -f "${SG_LOG}"
