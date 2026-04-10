#!/usr/bin/env bash
# teardown.sh -- Remove the local site pond and build output.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

rm -rf "${SCRIPTS}/pond"
rm -rf "${SCRIPTS}/build"

echo "=== Local site pond removed ==="
