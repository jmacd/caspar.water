#!/usr/bin/env bash
# setup.sh -- Initialize the site staging pond (cross-pond sitegen).
#
# Creates a local pond directory, copies site content and templates,
# installs import remotes and sitegen.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
SITE_DIR=$(cd "${STAGING_DIR}/../../../site" && pwd)
EXE="${SCRIPTS}/pond.sh"

source "$STAGING_DIR/env.sh"

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

# Install import factories (pull from MinIO staging buckets)
${EXE} mknod remote /system/etc/10-water --config-path "${SCRIPTS}/import-water.yaml"
${EXE} mknod remote /system/etc/11-noyo --config-path "${SCRIPTS}/import-noyo.yaml"

# Install combined sitegen
${EXE} mknod sitegen /system/etc/90-sitegen --config-path "${SCRIPTS}/site.yaml"

echo
echo "=== Site staging pond setup complete ==="
echo "Next: ./site/import.sh    # pull data from water + noyo ponds"
echo "Then: ./site/generate.sh  # build the combined site"
