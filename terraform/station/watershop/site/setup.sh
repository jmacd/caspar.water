#!/usr/bin/env bash
# setup.sh -- Initialize the site staging pond (cross-pond sitegen).
#
# Creates a local pond directory, copies site content and templates,
# installs import remotes and sitegen.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

source "$STAGING_DIR/env.sh"

# Export for ${env:...} expansion in apply configs
export SITE_DIR=$(cd "${STAGING_DIR}/../../../site" && pwd)

# Discover pond UUIDs from sibling staging ponds.
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

# Generate dynamic import configs with discovered pond UUIDs.
DYNAMIC_CFG=$(mktemp)
cat > "${DYNAMIC_CFG}" <<ENDCFG
version: v1
kind: mknod
metadata:
  path: /system/etc/10-water
spec:
  factory: remote
  config:
    region: "us-east-1"
    url: "s3://water-staging/pond-${WATER_POND_ID}"
    allow_http: true
    endpoint: "\${env:R2_ENDPOINT}"
    access_key: "\${env:R2_KEY}"
    secret_key: "\${env:R2_SECRET}"
    import:
      source_path: "/**"
      local_path: "/sources/water"
---
version: v1
kind: mknod
metadata:
  path: /system/etc/11-noyo
spec:
  factory: remote
  config:
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

# Apply static configs + dynamic import configs in one transaction
${EXE} apply -f "${SCRIPTS}/site.yaml" "${DYNAMIC_CFG}"

rm -f "${DYNAMIC_CFG}"

echo
echo "=== Site staging pond setup complete ==="
echo "Next: ./site/import.sh    # pull data from water + noyo ponds"
echo "Then: ./site/generate.sh  # build the combined site"
