#!/usr/bin/env bash
# pond.sh -- Run duckpond commands for the noyo staging pond.

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")

source "$STAGING_DIR/env.sh"

export POND="${SCRIPTS}/pond"

cd "${DUCKPOND_ROOT}"
exec ${CARGO} "$@"
