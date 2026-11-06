#!/usr/bin/env bash
# Refresh only the saved public analytics projection, independently of site builds.
set -euo pipefail

INSTANCE=${1:?usage: run-analytics-report.sh <instance>}
if [ "$#" -ne 1 ] || [[ ! "${INSTANCE}" =~ ^[a-zA-Z0-9_-]+-staging$ ]]; then
    echo "ERROR: Analytics report refresh requires one staging instance" >&2
    exit 1
fi

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"
source "${ENV_FILE}"

if [ "${ANALYTICS_REPORT_ENABLED:-0}" != 1 ] \
    || [ "${POND_RUNTIME:-container}" != container ]; then
    echo "ERROR: Analytics report refresh must be explicitly enabled for a container instance" >&2
    exit 1
fi

# Share run.sh's pond lock, not the collector's append/rotation lock.
LOCK_DIR=${XDG_RUNTIME_DIR:-/tmp}
exec 9>"${LOCK_DIR}/watertown-${UID}-${INSTANCE}.lock"
flock -w 600 9

"${SCRIPTS}/pond.sh" "${INSTANCE}" --pull-image
"${SCRIPTS}/pond.sh" "${INSTANCE}" --analytics-report
