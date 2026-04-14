#!/usr/bin/env bash
# setup.sh -- Initialize the local site pond for content development.
#
# Creates a local site pond, copies site content, and installs
# import remotes + sitegen using shared configs from site/.
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/.." && pwd)
SITE_DIR="${REPO_ROOT}/site"
EXE="${SCRIPTS}/pond.sh"

source "$SCRIPTS/env.sh"

set -x

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Create directory structure
${EXE} mkdir -p /system/etc
${EXE} mkdir -p /sources

# Copy site content and templates into the pond
${EXE} copy "host:///${SITE_DIR}/content" /content
${EXE} copy "host:///${SITE_DIR}/templates" /templates
${EXE} copy "host:///${SITE_DIR}/img" /img

# Install import factories (pull from staging MinIO)
${EXE} mknod remote /system/etc/10-water --config-path "${SITE_DIR}/import-water.yaml"
${EXE} mknod remote /system/etc/11-noyo --config-path "${SITE_DIR}/import-noyo.yaml"
${EXE} mknod remote /system/etc/12-septic --config-path "${SITE_DIR}/import-septic.yaml"

# Install combined sitegen
${EXE} mknod sitegen /system/etc/90-sitegen --config-path "${SITE_DIR}/site.yaml"

echo
echo "=== Local site pond setup complete ==="
echo "Next: ./sync.sh      # pull data from staging MinIO"
echo "Then: ./generate.sh  # build the site"
echo "Then: ./serve.sh     # serve locally"
