#!/usr/bin/env bash
# teardown.sh -- Remove the water staging pond.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

echo "Removing water pond directory"
rm -rf "${SCRIPTS}/pond"

echo "Water staging teardown complete."
