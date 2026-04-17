#!/usr/bin/env bash
# setup.sh -- Initialize the local site pond for content development.
#
# Creates a local site pond, copies site content, and installs
# import remotes + sitegen using declarative apply configs.
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/.." && pwd)
EXE="${SCRIPTS}/pond.sh"

source "$SCRIPTS/env.sh"

# Export SITE_DIR for ${env:SITE_DIR} expansion in apply configs
export SITE_DIR="${REPO_ROOT}/site"

set -x

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Apply all configs: dirs, copies, factories
${EXE} apply -f "${SCRIPTS}"/apply/*.yaml

echo
echo "=== Local site pond setup complete ==="
echo "Next: ./sync.sh      # pull data from staging MinIO"
echo "Then: ./generate.sh  # build the site"
echo "Then: ./serve.sh     # serve locally"
