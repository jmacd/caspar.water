#!/usr/bin/env bash
# teardown.sh -- Remove the noyo staging pond.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

echo "Removing noyo pond directory"
rm -rf "${SCRIPTS}/pond"

echo "Noyo staging teardown complete."
