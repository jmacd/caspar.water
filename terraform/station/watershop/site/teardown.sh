#!/usr/bin/env bash
# teardown.sh -- Remove the site staging pond.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

echo "Removing site pond directory"
rm -rf "${SCRIPTS}/pond"

echo "Site staging teardown complete."
