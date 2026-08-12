#!/usr/bin/env bash
# Send the configured weekly report for one site pond.
set -euo pipefail

INSTANCE=$1
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"
LOCK_DIR=${XDG_RUNTIME_DIR:-/tmp}

exec 9>"${LOCK_DIR}/watertown-${UID}-${INSTANCE}.lock"
flock -w 1800 9

set -a
source "${ENV_FILE}"
set +a

if [ "${POND_RUNTIME:-container}" = "native" ]; then
    EXE="${SCRIPTS}/pond-native.sh"
else
    EXE="${SCRIPTS}/pond.sh"
    "${EXE}" "${INSTANCE}" --pull-image
fi

"${EXE}" "${INSTANCE}" run /system/etc/95-email-report send
