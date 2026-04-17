#!/usr/bin/env bash
# run.sh -- Run a duckpond instance (called by systemd timer).
#
# Usage: run.sh <instance>
set -ex

INSTANCE=$1
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

# Extract pond type from instance name (e.g., noyo-staging → noyo)
TYPE="${INSTANCE%-staging}"
TYPE="${TYPE%-prod}"

case "${TYPE}" in
    noyo)
        ${EXE} "${INSTANCE}" run /system/etc/20-hydrovu collect
        ;;
    water)
        ${EXE} "${INSTANCE}" run /etc/ingest
        ;;
    septic)
        ${EXE} "${INSTANCE}" run /etc/ingest
        ;;
    site)
        ${EXE} "${INSTANCE}" run /system/etc/10-water pull
        ${EXE} "${INSTANCE}" run /system/etc/11-noyo pull
        ${EXE} "${INSTANCE}" run /system/etc/12-septic pull
        # Build site — pond.sh mounts www/ at /www
        DEPLOY_BASE="${SCRIPTS}/www"
        TIMESTAMP=$(date +%Y%m%d-%H%M%S)
        DEPLOY_DIR="${DEPLOY_BASE}/build-${TIMESTAMP}"
        mkdir -p "${DEPLOY_DIR}"
        SITE_BUILD_DIR="${DEPLOY_DIR}" ${EXE} "${INSTANCE}" run /system/etc/90-sitegen build /www
        ln -sfn "${DEPLOY_DIR}" "${DEPLOY_BASE}/current"
        # Clean old builds (keep last 3)
        ls -dt "${DEPLOY_BASE}"/build-* 2>/dev/null | tail -n +4 | xargs rm -rf
        ;;
    *)
        echo "ERROR: Unknown pond type '${TYPE}' for instance '${INSTANCE}'"
        exit 1
        ;;
esac
