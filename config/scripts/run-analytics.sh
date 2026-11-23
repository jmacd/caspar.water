#!/usr/bin/env bash
# Called by run.sh under the ordinary per-instance lock.
set -euo pipefail

INSTANCE=$1
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
# shellcheck source=/dev/null
source "${BASE_DIR}/env/${INSTANCE}.env"
ANALYTICS_PUBLISHED_DIR=$(readlink -f "${ANALYTICS_SITE_ROOT}")
export ANALYTICS_PUBLISHED_DIR
# Containers see the published build at /published and write reports to /www;
# native ponds use host paths directly.
NATIVE=0
EXE="${SCRIPTS}/pond.sh"
PUBLISHED_URL="host:///published"
if [ "${POND_RUNTIME:-container}" = "native" ]; then
    NATIVE=1
    EXE="${SCRIPTS}/pond-native.sh"
    PUBLISHED_URL="host://${ANALYTICS_PUBLISHED_DIR}"
fi
build_output() {
    if [ "${NATIVE}" = 1 ]; then
        echo "$1"
    else
        echo /www
    fi
}
DEPLOY_BASE="${BASE_DIR}/www/${INSTANCE}"
mkdir -p "${DEPLOY_BASE}"
WORK=$(mktemp -d "${DEPLOY_BASE}/.render-XXXXXX")
trap 'rm -r -- "${WORK}"' EXIT
TIMESTAMP=$(date -u +%Y%m%d-%H%M%S)
DEPLOY_DIR="${DEPLOY_BASE}/build-${TIMESTAMP}"
mkdir "${DEPLOY_DIR}"
FAILED=0

for SITE in caspar noyo; do
    INVENTORY="_analytics/pages.parquet"
    if [ "${SITE}" = noyo ]; then
        INVENTORY="noyo-harbor/${INVENTORY}"
    fi
    READY=1
    if ! "${EXE}" "${INSTANCE}" run "/etc/ingest-${SITE}" push; then
        echo "ERROR: ${SITE} analytics ingestion failed; publishing unavailable report" >&2
        READY=0
    fi
    if [ ! -f "${ANALYTICS_PUBLISHED_DIR}/${INVENTORY}" ]; then
        echo "ERROR: Missing published ${SITE} inventory; publishing unavailable report" >&2
        READY=0
    else
        COPY=1
        if "${EXE}" "${INSTANCE}" cat "/analytics/${SITE}/pages.parquet" --format=raw > "${WORK}/${SITE}-previous.parquet"; then
            if cmp -s "${WORK}/${SITE}-previous.parquet" "${ANALYTICS_PUBLISHED_DIR}/${INVENTORY}"; then
                COPY=0
            fi
        else
            echo "WARNING: Cannot read previous ${SITE} inventory; replacing from published source" >&2
        fi
        if [ "${COPY}" = 1 ] &&
            ! "${EXE}" "${INSTANCE}" copy "${PUBLISHED_URL}/${INVENTORY}" "/analytics/${SITE}/pages.parquet"; then
            echo "ERROR: ${SITE} inventory copy failed; publishing unavailable report" >&2
            READY=0
        fi
    fi
    mkdir "${WORK}/${SITE}"
    if [ "${READY}" = 1 ] &&
        SITE_BUILD_DIR="${WORK}/${SITE}" "${EXE}" "${INSTANCE}" run "/etc/site-${SITE}" \
            -- build "$(build_output "${WORK}/${SITE}")"; then
        mv "${WORK}/${SITE}" "${DEPLOY_DIR}/${SITE}"
    else
        echo "ERROR: ${SITE} analytics unavailable; no previous counts will be republished" >&2
        FAILED=1
        mkdir "${WORK}/${SITE}-unavailable"
        SITE_BUILD_DIR="${WORK}/${SITE}-unavailable" "${EXE}" "${INSTANCE}" \
            run "/etc/site-${SITE}" -- build "$(build_output "${WORK}/${SITE}-unavailable")" --quick
        mv "${WORK}/${SITE}-unavailable" "${DEPLOY_DIR}/${SITE}"
    fi
done

ln -sfn "${DEPLOY_DIR}" "${DEPLOY_BASE}/current"
mapfile -t BUILDS < <(find "${DEPLOY_BASE}" -mindepth 1 -maxdepth 1 -type d -name 'build-*' | sort -r)
for OLD_BUILD in "${BUILDS[@]:3}"; do
    rm -r -- "${OLD_BUILD}"
done
exit "${FAILED}"
