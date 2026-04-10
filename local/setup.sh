#!/usr/bin/env bash
# setup.sh -- Initialize the local site pond for content development.
#
# Discovers water and noyo pond UUIDs from the staging MinIO,
# creates a local site pond, copies site content, and installs
# import remotes + sitegen.
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
SITE_DIR=$(cd "${SCRIPTS}/../site" && pwd)
EXE="${SCRIPTS}/pond.sh"

source "$SCRIPTS/env.sh"

# Discover pond UUIDs from staging MinIO using pond's list-ponds command.
# This runs without a local pond (host+remote:// needs no POND context).
discover_pond_uuid() {
    local bucket="$1"
    local cfg
    cfg=$(mktemp)
    cat > "${cfg}" <<EOF
region: "us-east-1"
url: "s3://${bucket}"
allow_http: true
endpoint: "\${env:R2_ENDPOINT}"
access_key: "\${env:R2_KEY}"
secret_key: "\${env:R2_SECRET}"
EOF
    local uuid
    uuid=$(cd "${DUCKPOND_ROOT}" && ${CARGO} run "host+remote:///${cfg}" list-ponds 2>/dev/null \
        | grep "pond-id:" | tail -1 | awk '{print $2}')
    rm -f "${cfg}"
    echo "${uuid}"
}

echo "Discovering pond UUIDs from staging MinIO at ${R2_ENDPOINT}..."
WATER_POND_ID=$(discover_pond_uuid water-staging)
NOYO_POND_ID=$(discover_pond_uuid noyo-staging)

echo "Water pond ID: ${WATER_POND_ID}"
echo "Noyo pond ID:  ${NOYO_POND_ID}"

if [ -z "${WATER_POND_ID}" ] || [ -z "${NOYO_POND_ID}" ]; then
    echo "ERROR: Could not discover pond IDs from staging MinIO."
    echo "Check that R2_ENDPOINT=${R2_ENDPOINT} is reachable and buckets exist."
    exit 1
fi

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

# Generate import configs with discovered pond UUIDs.
# URLs are baked; credentials stay as ${env:} for runtime expansion.
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

# Install import factories (pull from staging MinIO)
${EXE} mknod remote /system/etc/10-water --config-path "${IMPORT_WATER_CFG}"
${EXE} mknod remote /system/etc/11-noyo --config-path "${IMPORT_NOYO_CFG}"

# Install combined sitegen
${EXE} mknod sitegen /system/etc/90-sitegen --config-path "${SCRIPTS}/site.yaml"

rm -f "${IMPORT_WATER_CFG}" "${IMPORT_NOYO_CFG}"

echo
echo "=== Local site pond setup complete ==="
echo "Next: ./sync.sh      # pull data from staging MinIO"
echo "Then: ./generate.sh  # build the site"
echo "Then: ./serve.sh     # serve locally"
