#!/usr/bin/env bash
# reset.sh -- Reset a duckpond instance on watershop (run locally).
#
# Usage: ./reset.sh <instance>
#   e.g., ./reset.sh water-staging
#         ./reset.sh noyo-staging
set -e

INSTANCE=$1
if [ -z "${INSTANCE}" ]; then
    echo "Usage: ./reset.sh <instance>"
    echo ""
    echo "Instances: water-staging, noyo-staging, septic-staging, site-staging"
    echo "           water-prod, noyo-prod, septic-prod"
    exit 1
fi

HOST="${WATERSHOP_HOST:-watershop.casparwater.us}"
USER="${WATERSHOP_USER:-jmacd}"

echo "Resetting ${INSTANCE} on ${HOST}..."
ssh "${USER}@${HOST}" "~/duckpond/reset.sh ${INSTANCE}"
