#!/usr/bin/env bash
# pond.sh -- Run duckpond commands for the local site pond.

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

source "$SCRIPTS/env.sh"

export POND="${SCRIPTS}/pond"

cd "${DUCKPOND_ROOT}"
exec ${CARGO} "$@"
