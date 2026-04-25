#!/usr/bin/env bash
# run-selfmon.sh -- run a native selfmon pond instance (no podman).
#
# Usage: run-selfmon.sh <instance>     (e.g. watershop-selfmon-staging)
#
# Reads ${BASE_DIR}/env/${INSTANCE}.env for POND, S3_*, etc.
# Invokes the host-installed `pond-selfmon-<tier>` binary directly.
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

# Choose binary by tier (staging vs prod, matching pond.sh image tag logic).
case "${INSTANCE}" in
    *-staging) PONDBIN="/usr/local/bin/pond-selfmon-staging" ;;
    *-prod)    PONDBIN="/usr/local/bin/pond-selfmon-prod" ;;
    *)         echo "ERROR: instance must end in -staging or -prod"; exit 2 ;;
esac

if [ ! -x "${PONDBIN}" ]; then
    echo "ERROR: ${PONDBIN} not installed; run extract-pond-binary.sh"
    exit 1
fi

# POND must be set per-instance (e.g. /home/jmacd/pond-watershop-selfmon-staging).
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

# Per-tick work: ingest journal, ingest caddy access logs, run
# Delta-Lake maintenance (checkpoint, log cleanup, vacuum-lite),
# render the dashboard, then capture perf metrics.  Order:
#   1. ingest sources (journal, caddy, prior-tick metrics)
#   2. sync site templates from /home/jmacd/duckpond/config/selfmon/site/
#      into pond /system/site/  (lets us iterate templates without
#      a git push -- terraform apply rsyncs config/, next tick picks up)
#   3. maintain (checkpoint)
#   4. sitegen build -> /var/www/selfmon/${INSTANCE}/  (timed)
#   5. measure (consumes the sitegen timing file written in step 4)
"${PONDBIN}" run /system/etc/journal push
"${PONDBIN}" run /system/etc/caddy-access push
"${PONDBIN}" run /system/etc/selfmon-metrics push 2>/dev/null || true

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

"${PONDBIN}" maintain

# Sitegen render, with wall-clock timing.  Output dir is owned by
# ${USER} (provisioned by terraform) and served by Caddy at /selfmon/.
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

"${SCRIPTS}/measure.sh" "${INSTANCE}"
