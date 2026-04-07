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

# Discover pond UUIDs from sibling staging ponds.
# Query directly (not through pond.sh which forces POND to site/pond).
WATER_POND_ID=$(cd "${DUCKPOND_ROOT}" && POND="${STAGING_DIR}/water/pond" ${CARGO} config 2>/dev/null | grep "Pond ID" | awk '{print $NF}')
NOYO_POND_ID=$(cd "${DUCKPOND_ROOT}" && POND="${STAGING_DIR}/noyo/pond" ${CARGO} config 2>/dev/null | grep "Pond ID" | awk '{print $NF}')

echo "Water pond ID: ${WATER_POND_ID}"
echo "Noyo pond ID: ${NOYO_POND_ID}"

if [ -z "${WATER_POND_ID}" ] || [ -z "${NOYO_POND_ID}" ]; then
    echo "ERROR: Could not discover pond IDs. Run water/setup.sh and noyo/setup.sh first."
    exit 1
fi

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

# Generate import configs with discovered pond UUIDs.
# URLs are baked (dynamic config); credentials stay as ${env:} for runtime.
IMPORT_WATER_CFG=$(mktemp)
cat > "${IMPORT_WATER_CFG}" <<ENDCFG
region: "us-east-1"
url: "s3://water-staging/pond-${WATER_POND_ID}"
allow_http: true
endpoint: "\${env:R2_ENDPOINT}"
access_key: "\${env:R2_KEY}"
secret_key: "\${env:R2_SECRET}"
import:
  source_path: "/**"
  local_path: "/sources/water"
ENDCFG

IMPORT_NOYO_CFG=$(mktemp)
cat > "${IMPORT_NOYO_CFG}" <<ENDCFG
region: "us-east-1"
url: "s3://noyo-staging/pond-${NOYO_POND_ID}"
allow_http: true
endpoint: "\${env:R2_ENDPOINT}"
access_key: "\${env:R2_KEY}"
secret_key: "\${env:R2_SECRET}"
import:
  source_path: "/**"
  local_path: "/sources/noyo"
ENDCFG

# Install import factories (pull from MinIO staging buckets)
${EXE} mknod remote /system/etc/10-water --config-path "${IMPORT_WATER_CFG}"
${EXE} mknod remote /system/etc/11-noyo --config-path "${IMPORT_NOYO_CFG}"

# Install combined sitegen
${EXE} mknod sitegen /system/etc/90-sitegen --config-path "${SCRIPTS}/site.yaml"

rm -f "${IMPORT_WATER_CFG}" "${IMPORT_NOYO_CFG}"

echo
echo "=== Site staging pond setup complete ==="
echo "Next: ./site/import.sh    # pull data from water + noyo ponds"
echo "Then: ./site/generate.sh  # build the combined site"
