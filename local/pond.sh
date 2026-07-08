#!/usr/bin/env bash
# pond.sh -- Run watertown commands for the local site pond.

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

source "$SCRIPTS/env.sh"

export POND="${SCRIPTS}/pond"

cd "${WATERTOWN_ROOT}"
exec ${CARGO} "$@"
