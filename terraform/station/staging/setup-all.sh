#!/usr/bin/env bash
# setup-all.sh -- Initialize all three staging ponds from scratch.
#
# Usage:
#   ./setup-all.sh    # first time or full reset
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)

echo "=== Setting up water pond ==="
"${SCRIPTS}/water/setup.sh"

echo ""
echo "=== Setting up noyo pond ==="
"${SCRIPTS}/noyo/setup.sh"

echo ""
echo "=== Setting up site pond ==="
"${SCRIPTS}/site/setup.sh"

echo ""
echo "=== All staging ponds initialized ==="
echo "Next: ./run-all.sh"
