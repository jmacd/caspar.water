#!/usr/bin/env bash
# pond-native.sh -- Run a native pond instance with its deployed environment.
#
# Usage: pond-native.sh <instance> <pond arguments...>
set -euo pipefail

INSTANCE=${1:?usage: pond-native.sh <instance> <pond arguments...>}
shift

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"

if [ ! -f "${ENV_FILE}" ]; then
    echo "ERROR: missing environment file ${ENV_FILE}" >&2
    exit 1
fi

set -a
# shellcheck disable=SC1090
source "${ENV_FILE}"
set +a

: "${POND:?${ENV_FILE} must define POND}"
exec /usr/bin/pond "$@"
