#!/usr/bin/env bash
# measure-pond.sh -- emit one JSON line of perf metrics for a single pond.
#
# Usage: measure-pond.sh <pond>
#
# Reads ${BASE_DIR}/env/<pond>.env to pick up POND (the pond's
# on-disk backing-store path on this host).  Writes a single JSON
# row to ${SELFMON_METRICS_DIR}/<pond>.jsonl (note: *the selfmon
# pond's* metrics dir, not the probed pond's), where it is mirrored
# into the selfmon pond by a `logfile-ingest` mknod and joined with
# every other pond's series via `timeseries-join`.
#
# The column names match the canonical metric_name values defined
# in config/semconv/duckpond-pond.yaml, namely:
#
#   committed.txn_ids   counter        cumulative commit count
#   parquet.files       updowncounter  *.parquet under POND
#   delta_log.files     updowncounter  *.json under */_delta_log/
#   size.bytes          updowncounter  du -sb POND
#   list.seconds        gauge          time `pond list /`
#   peak_rss.bytes      updowncounter  prior-tick max from journal
#
# Default kind is `gauge`; only non-gauge entries appear in the
# semconv registry, and chart.js applies a counter rate transform
# only for keys explicitly listed there.
set -e

POND_NAME=${1:?Usage: measure-pond.sh <pond>}
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${POND_NAME}.env"

if [ ! -f "${ENV_FILE}" ]; then
    echo "ERROR: no env file for '${POND_NAME}' at ${ENV_FILE}" >&2
    exit 1
fi

# Select the systemd unit prefix for this pond's liveness probes.  The
# selfmon pond runs under pond-selfmon@<instance>.{timer,service} while
# every container pond runs under pond@<name>.{timer,service}.
# run-selfmon.sh exports SELFMON_INSTANCE with its own instance name so
# this probe targets the correct units when it measures the selfmon pond
# itself.  Any other pond, or an unset SELFMON_INSTANCE, uses pond@.
# Without this the selfmon pond's own status card read timer.active from
# a nonexistent pond@<instance>.timer and was perpetually classified red.
if [ "${POND_NAME}" = "${SELFMON_INSTANCE:-}" ]; then
    UNIT_PREFIX="pond-selfmon@"
else
    UNIT_PREFIX="pond@"
fi

# Source env file.  The probed pond's env, NOT the selfmon pond's.
# We need POND (the storage path) to find this pond's data.
(
    # Subshell to keep env isolated; we re-read SELFMON_METRICS_DIR
    # from the *caller's* environment (see below).
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a
    # Systemd unit selection uses UNIT_PREFIX, computed above from
    # SELFMON_INSTANCE, so the leaked SELFMON marker from the selfmon
    # env is no longer consulted here.

    : "${POND:?POND must be set in ${ENV_FILE}}"

    # If POND_VOLUME is set, the pond runs in a podman named volume
    # (the production layout on watershop).  Rootless podman volumes
    # are owned by this user under
    #   ~/.local/share/containers/storage/volumes/<vol>/_data
    # which is fully readable on the host -- so we can run all the
    # host-fs probes (find, du) and the on-disk pond CLI (`pond list`,
    # `pond log`) directly against the volume mountpoint, without
    # paying podman startup latency for every probe.  The env file's
    # POND value (e.g. /home/jmacd/pond-water-prod) is only used to
    # label the *container's* bind mount and does not exist on the
    # host -- hence the all-zeros reports we used to emit.
    if [ -n "${POND_VOLUME:-}" ] && command -v podman >/dev/null 2>&1; then
        VOL_MOUNT=$(podman volume inspect "${POND_VOLUME}" \
            --format '{{.Mountpoint}}' 2>/dev/null || true)
        if [ -n "${VOL_MOUNT}" ] && [ -d "${VOL_MOUNT}" ]; then
            POND="${VOL_MOUNT}"
        fi
    fi

    # SELFMON_METRICS_DIR is exported by run-selfmon.sh from the
    # selfmon pond's env file (NOT this pond's env), and points at
    # the directory the selfmon pond mirrors per-pond jsonl files
    # out of.  Sourcing the probed pond's env may have clobbered it
    # if that pond also defines SELFMON_METRICS_DIR -- so we capture
    # the caller's value before the source.
    : "${MEASURE_OUT_DIR:?run-selfmon.sh must set MEASURE_OUT_DIR}"
    mkdir -p "${MEASURE_OUT_DIR}"

    # Skip ponds whose backing store doesn't exist yet (un-bootstrapped
    # tier, etc.) -- but still emit a single zero-row.  The per-pond
    # `sql-derived-series` needs SOMETHING to read; a missing jsonl
    # in the pond would break the dependent timeseries-join.  A zero
    # row tells the truth (no parquet yet, 0 commits) and the chart
    # gets a clean baseline rather than an undefined panel.
    if [ ! -d "${POND}" ]; then
        echo "measure-pond: '${POND_NAME}' has no POND dir at ${POND}; emitting zero row" >&2
        TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)
        printf '{"ts":"%s","committed.txn_ids":0,"parquet.files":0,"delta_log.files":0,"size.bytes":0,"list.seconds":0,"peak_rss.bytes":0,"run.seconds":0,"timer.active":0,"last_run.seconds_ago":-1,"timer.interval_s":0}\n' \
            "${TS}" >> "${MEASURE_OUT_DIR}/${POND_NAME}.jsonl"
        exit 0
    fi

    PONDBIN=/usr/bin/pond
    if [ ! -x "${PONDBIN}" ]; then
        echo "ERROR: ${PONDBIN} not installed; run install-duckpond.sh" >&2
        exit 1
    fi
    export POND   # pond CLI reads $POND

    # ── timed list ────────────────────────────────────────────────
    LIST_START=$(date +%s.%N)
    "${PONDBIN}" list / >/dev/null 2>&1 || true
    LIST_END=$(date +%s.%N)
    LIST_SECONDS=$(awk -v a="${LIST_END}" -v b="${LIST_START}" \
        'BEGIN{printf "%.3f", a-b}')

    # ── structural counters ───────────────────────────────────────
    PARQUET_FILES=$(find "${POND}" -name '*.parquet' 2>/dev/null \
        | wc -l | tr -d ' ')
    DELTA_LOG_FILES=$(find "${POND}" -path '*_delta_log*' -name '*.json' \
        2>/dev/null | wc -l | tr -d ' ')
    SIZE_BYTES=$(du -sb "${POND}" 2>/dev/null | awk '{print $1}')
    [ -z "${SIZE_BYTES}" ] && SIZE_BYTES=0

    # ── current commit sequence ───────────────────────────────────
    # `pond log --limit 1` outputs human-readable boxes; the most
    # recent COMMITTED line looks like:
    #   +- Transaction 26 (write) ----------------
    # The transaction number IS committed.txn_ids.
    TXN_SEQ=$("${PONDBIN}" log --limit 1 2>/dev/null \
        | grep -oE 'Transaction [0-9]+' \
        | head -1 \
        | awk '{print $2}')
    [ -z "${TXN_SEQ}" ] && TXN_SEQ=0

    # ── peak RSS of previous tick ─────────────────────────────────
    # Stored in bytes (see config/semconv/duckpond-pond.yaml: unit By).
    # Source line emitted by pond CLI to stderr at process exit:
    #   "Peak memory usage: NN.NN MB"
    # Container ponds run via pond@<name>.service; the selfmon pond runs
    # via pond-selfmon@<instance>.service.  UNIT_PREFIX selects the right
    # one.  The _self block in run-selfmon.sh contributes selfmon-only
    # metrics such as read.seconds and sitegen.seconds; this script still
    # probes the selfmon pond for the generic size, txn, and timer rows.
    UNIT="${UNIT_PREFIX}${POND_NAME}.service"
    PEAK_RSS_BYTES=$(journalctl --user -u "${UNIT}" \
        --no-pager -n 200 --since "5 minutes ago" 2>/dev/null \
        | grep -oE 'Peak memory usage: [0-9.]+ MB' \
        | awk '{ if ($4+0 > max) max=$4+0 }
               END   { printf "%.0f", (max==""?0:max) * 1048576 }')

    # ── elapsed time of last `pond run` (run.seconds) ─────────────
    # The pond CLI emits one structured "Run summary" line per `pond
    # run` invocation, e.g.:
    #   "Run summary  path=...  factory=...  args=[...]
    #    elapsed_s=12.345  peak_mem_mb=...  outcome=ok"
    # We grep the unit's journal for the most recent such line and
    # extract elapsed_s.  For pond@<name>.service this is the timer's
    # main job; for pond-selfmon@<...>.service it's whichever inner
    # `pond run` finished most recently in the selfmon loop.
    RUN_SECONDS=$(journalctl --user -u "${UNIT}" \
        --no-pager -n 500 --since "5 minutes ago" 2>/dev/null \
        | grep -oE 'Run summary .* elapsed_s=[0-9.]+' \
        | tail -1 \
        | grep -oE 'elapsed_s=[0-9.]+' \
        | awk -F= 'END{ printf "%.3f", ($2==""?0:$2) }')
    [ -z "${RUN_SECONDS}" ] && RUN_SECONDS=0

    # ── timer + service liveness via systemctl ────────────────────
    # Two signals that surface stopped/stale ponds even when nothing
    # is being logged (e.g. timer disabled, container failing to
    # start, host bind-mount missing):
    #   timer.active           1 if <unit>.timer is active, else 0
    #   last_run.seconds_ago   wall-clock seconds since the service's
    #                          most recent ExecMainExitTimestamp; -1
    #                          if the service has never run.
    TIMER_UNIT="${UNIT_PREFIX}${POND_NAME}.timer"
    SERVICE_UNIT="${UNIT_PREFIX}${POND_NAME}.service"

    if [ "$(systemctl --user is-active "${TIMER_UNIT}" 2>/dev/null)" = active ]; then
        TIMER_ACTIVE=1
    else
        TIMER_ACTIVE=0
    fi

    EXIT_TS=$(systemctl --user show "${SERVICE_UNIT}" \
        -p ExecMainExitTimestamp --value 2>/dev/null)
    if [ -n "${EXIT_TS}" ] && [ "${EXIT_TS}" != "n/a" ]; then
        EXIT_EPOCH=$(date -d "${EXIT_TS}" +%s 2>/dev/null)
        if [ -n "${EXIT_EPOCH}" ] && [ "${EXIT_EPOCH}" -gt 0 ] 2>/dev/null; then
            LAST_RUN_AGO=$(( $(date +%s) - EXIT_EPOCH ))
            [ "${LAST_RUN_AGO}" -lt 0 ] && LAST_RUN_AGO=0
        else
            LAST_RUN_AGO=-1
        fi
    else
        LAST_RUN_AGO=-1
    fi

    # Configured timer interval (OnUnitActiveSec), in integer seconds.
    # Drives the status grid's "stale = 2x interval" health threshold
    # so the page knows what "overdue" means without per-unit YAML
    # configuration.
    #
    # systemd does NOT expose this as a top-level scalar property
    # (`-p OnUnitActiveSecUSec` returns empty on v252+).  The value
    # lives inside the `TimersMonotonic` multi-record property in
    # human-readable form, e.g.:
    #     { OnUnitActiveUSec=30min ; next_elapse=... }
    #     { OnBootUSec=6min ; next_elapse=... }
    # We extract the OnUnitActiveUSec= field and convert the suffix
    # (us/ms/s/min/h) to integer seconds.  Falls back to 0 when the
    # property is missing -- the renderer treats 0 as Unknown rather
    # than a hard error, so a misconfigured timer doesn't break the
    # whole status grid.
    TIMER_INTERVAL_RAW=$(systemctl --user show "${TIMER_UNIT}" \
        -p TimersMonotonic --value 2>/dev/null \
        | grep -oE 'OnUnitActiveUSec=[^ ]+' \
        | head -1 \
        | cut -d= -f2)
    case "${TIMER_INTERVAL_RAW}" in
        *us)  TIMER_INTERVAL_S=$(( ( ${TIMER_INTERVAL_RAW%us} + 999999 ) / 1000000 )) ;;
        *ms)  TIMER_INTERVAL_S=$(( ( ${TIMER_INTERVAL_RAW%ms} + 999 ) / 1000 )) ;;
        *min) TIMER_INTERVAL_S=$(( ${TIMER_INTERVAL_RAW%min} * 60 )) ;;
        *h)   TIMER_INTERVAL_S=$(( ${TIMER_INTERVAL_RAW%h} * 3600 )) ;;
        *s)   TIMER_INTERVAL_S=${TIMER_INTERVAL_RAW%s} ;;
        *)    TIMER_INTERVAL_S=0 ;;
    esac
    [ -z "${TIMER_INTERVAL_S}" ] && TIMER_INTERVAL_S=0

    TS=$(date -u +%Y-%m-%dT%H:%M:%SZ)

    # Single JSON line, append.  Column names match metric_name
    # entries in config/semconv/duckpond-pond.yaml.
    printf '{"ts":"%s","committed.txn_ids":%s,"parquet.files":%s,"delta_log.files":%s,"size.bytes":%s,"list.seconds":%s,"peak_rss.bytes":%s,"run.seconds":%s,"timer.active":%s,"last_run.seconds_ago":%s,"timer.interval_s":%s}\n' \
        "${TS}" "${TXN_SEQ}" "${PARQUET_FILES}" "${DELTA_LOG_FILES}" \
        "${SIZE_BYTES}" "${LIST_SECONDS}" "${PEAK_RSS_BYTES}" "${RUN_SECONDS}" \
        "${TIMER_ACTIVE}" "${LAST_RUN_AGO}" "${TIMER_INTERVAL_S}" \
        >> "${MEASURE_OUT_DIR}/${POND_NAME}.jsonl"
)
