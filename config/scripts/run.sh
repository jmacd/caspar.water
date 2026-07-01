#!/usr/bin/env bash
# run.sh -- Run a duckpond instance (called by systemd timer).
#
# Usage: run.sh <instance>
set -ex

INSTANCE=$1
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
EXE="${SCRIPTS}/pond.sh"
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"

# Source env file for variables needed by run.sh itself (e.g., CLOUD_HOST)
if [ -f "${ENV_FILE}" ]; then
    source "${ENV_FILE}"
fi

# Extract pond type from instance name (e.g., noyo-staging -> noyo).
# watershop-selfmon has no -staging/-prod suffix and matches *-selfmon.
TYPE="${INSTANCE%-staging}"
TYPE="${TYPE%-prod}"

case "${TYPE}" in
    noyo)
        ${EXE} "${INSTANCE}" run /laketech/data pull
        ${EXE} "${INSTANCE}" run /system/site pull
        ${EXE} "${INSTANCE}" run /system/etc/20-hydrovu collect
        ;;
    water)
        ${EXE} "${INSTANCE}" run /etc/ingest
        ;;
    septic)
        ${EXE} "${INSTANCE}" run /etc/ingest
        ;;
    *-selfmon)
        # Self-monitoring runs natively (no podman): see
        # config/scripts/run-selfmon.sh and pond-selfmon@.service.
        echo "ERROR: selfmon instances run natively, not via pond@.service" >&2
        exit 2
        ;;
    site)
        ${EXE} "${INSTANCE}" run /content pull
        ${EXE} "${INSTANCE}" run /templates pull
        ${EXE} "${INSTANCE}" run /img pull
        ${EXE} "${INSTANCE}" pull water
        ${EXE} "${INSTANCE}" pull noyo
        ${EXE} "${INSTANCE}" pull septic
        # Build site with atomic deploy
        DEPLOY_BASE="${BASE_DIR}/www/${INSTANCE}"
        TIMESTAMP=$(date +%Y%m%d-%H%M%S)
        DEPLOY_DIR="${DEPLOY_BASE}/build-${TIMESTAMP}"
        mkdir -p "${DEPLOY_DIR}"
        SITE_BUILD_DIR="${DEPLOY_DIR}" ${EXE} "${INSTANCE}" run /system/etc/90-sitegen build /www
        ln -sfn "${DEPLOY_DIR}" "${DEPLOY_BASE}/current"
        # Clean old builds (keep last 3)
        ls -dt "${DEPLOY_BASE}"/build-* 2>/dev/null | tail -n +4 | xargs rm -rf
        # For production: deploy to cloud host via rsync + atomic symlink
        if [[ "${INSTANCE}" == *-prod ]] && [ -n "${CLOUD_HOST}" ]; then
            CLOUD_WWW="/home/jmacd/duckpond/www"
            CLOUD_BUILD="${CLOUD_WWW}/build-${TIMESTAMP}"
            rsync -az --delete "${DEPLOY_DIR}/" "${CLOUD_HOST}:${CLOUD_BUILD}/"
            ssh "${CLOUD_HOST}" "ln -sfn '${CLOUD_BUILD}' '${CLOUD_WWW}/current' && ls -dt '${CLOUD_WWW}'/build-* | tail -n +4 | xargs rm -rf"
        fi
        ;;
    *)
        echo "ERROR: Unknown pond type '${TYPE}' for instance '${INSTANCE}'"
        exit 1
        ;;
esac

# Automatic maintenance after each successful run: checkpoint + vacuum
# (both internally gated to run periodically).  Also collapse data:series
# files with more than 100 live versions into one merged version; the
# threshold self-gates so only noisy files are touched.  --compact merges
# the data table's add-files (self-gated to noisy partitions) and is recorded
# as a pushable Compact bundle, so the data volume stays maintained on remotes.
${EXE} "${INSTANCE}" maintain --compact --collapse-versions 100
