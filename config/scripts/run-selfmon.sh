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
    echo "ERROR: ${PONDBIN} not installed; run update-selfmon.sh ${INSTANCE}"
    exit 1
fi

# POND must be set per-instance (e.g. /home/jmacd/pond-watershop-selfmon).
: "${POND:?POND must be set in ${ENV_FILE}}"
export POND

# Expose the selfmon instance name to measure-pond.sh so its self-probe
# selects the pond-selfmon@ systemd units instead of the pond@ units used
# by container ponds.  Without this the selfmon pond's own status card
# reads timer.active from a nonexistent pond@<instance>.timer and is
# perpetually classified red.
export SELFMON_INSTANCE="${INSTANCE}"

# ── Step accounting: no failure in this tick may be silent ──
#
# Most steps below are deliberately non-fatal: aborting mid-tick is what
# historically wedged the pond (see the maintain comment).  But "non-fatal"
# used to mean "invisible" -- several steps ran under `2>/dev/null || true`,
# discarding both the exit code AND the error text, so a step could fail
# every minute for weeks with nothing anywhere to show for it.
#
# Every non-fatal step now runs through `step`, which keeps the tick going
# but records the failure in three places: stderr (journal), the step's own
# exit code, and a counter published as the `tick.failures` metric.  The
# last one matters most -- it puts failures on the dashboard the operator
# already watches, instead of in a log nobody reads.
FAILURE_COUNT=0
FAILED_STEPS=""
STATUS_FILE="${SELFMON_METRICS_DIR:-/tmp}/.tick-status.json"

step() {
    step_name="$1"
    shift
    set +e
    "$@"
    step_rc=$?
    set -e
    if [ "${step_rc}" -ne 0 ]; then
        FAILURE_COUNT=$((FAILURE_COUNT + 1))
        FAILED_STEPS="${FAILED_STEPS}${FAILED_STEPS:+,}${step_name}"
        echo "ERROR: selfmon step '${step_name}' failed (rc=${step_rc})" >&2
    fi
    return 0
}

# Written on EXIT rather than at the end of the script so that an abort --
# `set -e` firing on a fatal step, or an OOM kill -- is still recorded.  A
# tick that dies halfway is exactly the case that must not vanish.
write_tick_status() {
    exit_rc=$?
    if [ "${exit_rc}" -ne 0 ]; then
        FAILURE_COUNT=$((FAILURE_COUNT + 1))
        FAILED_STEPS="${FAILED_STEPS}${FAILED_STEPS:+,}tick-aborted"
    fi
    # Written atomically via rename so a reader never sees a partial record.
    # If even this fails, say so on stderr: the status file is the channel
    # that makes every other failure visible, so losing it silently would
    # defeat the whole mechanism.
    if ! { printf '{"failures":%d,"steps":"%s","exit_rc":%d}\n' \
        "${FAILURE_COUNT}" "${FAILED_STEPS}" "${exit_rc}" \
        > "${STATUS_FILE}.tmp" && mv -f "${STATUS_FILE}.tmp" "${STATUS_FILE}"; }
    then
        echo "ERROR: could not write tick status to ${STATUS_FILE};" \
            "tick.failures will be stale" >&2
    fi
    if [ "${FAILURE_COUNT}" -ne 0 ]; then
        echo "selfmon tick finished with ${FAILURE_COUNT} failed step(s):" \
            "${FAILED_STEPS}" >&2
        # Exit non-zero so systemd marks the run failed and it shows up in
        # `systemctl --failed`.  A tick that ran every step, had five of them
        # fail, and then reported success is itself a silent failure.  This
        # is safe precisely because it happens in the EXIT trap: all the work
        # has already been done, so the non-zero status reports the outcome
        # rather than truncating the tick.  The unit is timer-driven and
        # one-shot, so a failed run does not cascade into a restart loop.
        [ "${exit_rc}" -eq 0 ] && exit_rc=1
    fi
    exit "${exit_rc}"
}
trap write_tick_status EXIT

# ── Pre-tick maintain: trim the delta log BEFORE anything reads it ──
# This is the ONLY maintain per tick, and it runs FIRST on purpose.
# Every commit appends an uncheckpointed entry to the Delta log; listing
# or resolving /logs/journal re-reads all of them through a DataFusion
# external sort.  When maintain ran LAST instead, a single out-of-memory
# abort under `set -e` skipped it, so uncheckpointed versions accumulated
# across ticks until that sort exceeded the memory pool and every tick
# wedged permanently on the first read.  Running maintain first checkpoints
# the log so the rest of this tick reads a small checkpoint; the handful of
# versions this tick then appends before sitegen reads are trimmed by the
# next tick's pass.  A failure here is non-fatal so the tick still proceeds
# and retries next minute.
#
# --compact runs every tick on purpose.  The control table gains one
# small record-parquet per control commit and is otherwise NEVER merged:
# post-commit auto-maintain and a plain `pond maintain` both pass
# compact=false, so without per-tick compaction its add-file count grows
# without bound and every force=true checkpoint must re-list all of them,
# bloating {POND}/control to many GB of checkpoint parquets.  Compacting
# each tick keeps the control add-file count -- and therefore checkpoint
# size -- bounded, which also keeps default log retention harmless.  This
# is the aggressive-maintenance mode selfmon exists to exercise.
#
# --prune shrinks the control table's logical ROW count, which compaction
# alone never does: compaction merges add-files but the append-only
# lifecycle log grows ~3-6 rows per transaction forever.  Pruning deletes
# replicated history at/below a safe horizon in this same checkpoint +
# vacuum pass.  selfmon has no push remote, so --allow-no-remote enables
# retention-only pruning: keep the most recent --keep-txns transactions.
# Pruned history is unrecoverable, which is fine for selfmon.
#
# --collapse-versions 100 collapses data:series files with >100 live
# versions; the threshold self-gates.
step maintain "${PONDBIN}" maintain --compact --collapse-versions 100 \
    --prune --allow-no-remote --keep-txns 1000

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
        FAILURE_COUNT=$((FAILURE_COUNT + 1))
        FAILED_STEPS="${FAILED_STEPS}${FAILED_STEPS:+,}journal-cursor-seed"
        echo "ERROR: selfmon step 'journal-cursor-seed' failed to obtain the" \
            "current journal cursor; first ingest may OOM" >&2
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
#   5. materialize -- append this tick's new /derived/perf rows into the
#                  physical /metrics/perf.series that /reduced reads.
#   6. sitegen  -- render dashboard from /reduced into /var/www.

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
    READ_OK=0
    if "${PONDBIN}" list /logs/journal/kernel.jsonl >/dev/null 2>&1; then
        # A failed read must not be published as a FAST read.  This block
        # used to time the command under `|| true` and record the elapsed
        # time regardless, so a read that errored out in 5 ms landed on the
        # chart as a 200x performance improvement -- the failure looked like
        # the best tick we ever had.  Now the duration is only published
        # when the read actually returned, and the failure is counted.
        READ_START=$(date +%s.%N)
        if "${PONDBIN}" cat 'jsonlogs:///logs/journal/kernel.jsonl' \
            --sql 'SELECT COUNT(*) FROM source' --format=table >/dev/null; then
            READ_END=$(date +%s.%N)
            READ_SECONDS=$(awk -v a="${READ_END}" -v b="${READ_START}" \
                'BEGIN{printf "%.3f", a-b}')
            READ_OK=1
        else
            echo "ERROR: selfmon step 'read-benchmark' failed" >&2
        fi
    else
        echo "ERROR: selfmon step 'read-benchmark' failed:" \
            "/logs/journal/kernel.jsonl not listable" >&2
    fi
    if [ "${READ_OK}" -eq 0 ]; then
        FAILURE_COUNT=$((FAILURE_COUNT + 1))
        FAILED_STEPS="${FAILED_STEPS}${FAILED_STEPS:+,}read-benchmark"
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

    # tick.failures is the PREVIOUS tick's count, for exactly the reason
    # sitegen.seconds is: this record is written near the top of the tick,
    # before ingest/materialize/sitegen have had a chance to fail.  Lagging
    # by one minute is the price of publishing it in-band, and in-band is
    # what makes a failure visible on the dashboard instead of only in the
    # journal.  read.ok is current-tick, since that step has already run.
    PREV_FAILURES=0
    if [ -f "${STATUS_FILE}" ]; then
        PREV_FAILURES=$(awk -F'[:,}]' '/failures/ {
            for (i=1;i<=NF;i++) if ($i ~ /failures/) { print $(i+1); exit } }' \
            "${STATUS_FILE}" | tr -d ' "')
        [ -z "${PREV_FAILURES}" ] && PREV_FAILURES=0
    fi

    TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
    printf '{"ts":"%s","read.seconds":%s,"read.ok":%s,"tick.failures":%s,"sitegen.seconds":%s,"sitegen_peak_rss.bytes":%s}\n' \
        "${TS}" "${READ_SECONDS}" "${READ_OK}" "${PREV_FAILURES}" \
        "${SITEGEN_SECONDS}" "${SITEGEN_PEAK_RSS_BYTES}" \
        >> "${MEASURE_OUT_DIR}/_self.jsonl"
} || {
    FAILURE_COUNT=$((FAILURE_COUNT + 1))
    FAILED_STEPS="${FAILED_STEPS}${FAILED_STEPS:+,}_self-measurement"
    echo "ERROR: selfmon step '_self-measurement' failed" >&2
}

# One probe per pond defined under ${BASE_DIR}/env/.
#
# Not every env file is a pond: terraform also writes credential env files
# there (env/_minio-admin.env, used by the aws-cli container to create and
# empty buckets).  A leading underscore marks "not a pond" -- _self is the
# other one, and it is handled explicitly rather than by this loop.  Without
# the skip, _minio-admin is enumerated as a pond name and its ingest below
# runs against a mknod the yaml never declares, failing on every tick.
for envf in "${BASE_DIR}/env"/*.env; do
    [ -f "$envf" ] || continue
    pond_name=$(basename "$envf" .env)
    case "$pond_name" in _*) continue ;; esac
    step "measure:${pond_name}" "${SCRIPTS}/measure-pond.sh" "${pond_name}"
done

# Ingest external sources.  Non-fatal: a transient failure in one source
# must not abort the tick before the pre-sitegen maintain runs, which is
# what historically let uncheckpointed versions pile up and wedge the pond.
step ingest:journal "${PONDBIN}" run /system/etc/journal push
step ingest:caddy-access "${PONDBIN}" run /system/etc/caddy-access push

# Ingest per-pond perf jsonl.  One mknod per pond + _self because
# logfile-ingest selects exactly ONE active file per mknod.  Mknods match
# the pond env files one-for-one, plus _self; underscore-prefixed env files
# are skipped here for the same reason as the measure loop above.
# These were the worst offenders: `2>/dev/null || true` discarded the error
# text as well as the status.  This is the ingest that feeds every chart, so
# a silent failure here stalls the entire dataset while the page keeps
# rendering the last good data as if nothing were wrong.
step ingest:measure:_self "${PONDBIN}" run /system/etc/measure/_self push
for envf in "${BASE_DIR}/env"/*.env; do
    [ -f "$envf" ] || continue
    pond_name=$(basename "$envf" .env)
    case "$pond_name" in _*) continue ;; esac
    step "ingest:measure:${pond_name}" \
        "${PONDBIN}" run "/system/etc/measure/${pond_name}" push
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

# Materialize the perf join into /metrics/perf.series before sitegen, so
# the /reduced rollup sitegen exports includes this tick's samples.  Must
# come AFTER the per-pond ingest above (it reads /derived/perf, which reads
# the ingested jsonl) and BEFORE sitegen.
#
# Non-fatal for the same reason as ingest: a failure here should leave the
# dashboard one tick stale, not abort the tick.  The watermark is recomputed
# from the target on every run, so a skipped tick self-heals -- the next run
# picks up everything past the last stored row.
step materialize-perf "${PONDBIN}" run /system/etc/materialize-perf

# Maintenance already ran at the top of this tick; sitegen reads the pond
# as-is.  The few versions appended since that pass are collapsed by the
# next tick's pre-tick maintain.

# Sitegen render, with wall-clock timing.  Output dir is owned by
# ${USER} (provisioned by terraform) and served by Caddy at /selfmon/.
# Vendor assets (DuckDB-WASM, Plot, D3) are installed at
# /usr/share/watertown/vendor by the watertown .deb, which is where
# sitegen's find_vendor_dir() searches for them.
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
