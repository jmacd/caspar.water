#!/usr/bin/env bash
# serve.sh -- Serve the generated site locally.
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BUILDDIR="${SCRIPTS}/build"

if [ ! -d "${BUILDDIR}" ]; then
    echo "No build directory. Run ./generate.sh first."
    exit 1
fi

PORT="${PORT:-8080}"
echo "Serving site at http://localhost:${PORT}/"
echo "Press Ctrl+C to stop."

cd "${BUILDDIR}"
exec python3 -m http.server "${PORT}"
