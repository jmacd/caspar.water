#!/usr/bin/env bash
# setup.sh -- Initialize the water staging pond.
#
# Creates a local pond directory, copies site content and templates,
# and installs factory nodes.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
SITE_DIR=$(cd "${STAGING_DIR}/../../../site" && pwd)
EXE="${SCRIPTS}/pond.sh"

source "$STAGING_DIR/env.sh"

# Generate ingest config with absolute paths for NFS data
INGEST_CFG=$(mktemp)
cat > "${INGEST_CFG}" <<EOF
archived_pattern: ${WATER_DATA_DIR}/casparwater-*.json
active_pattern: ${WATER_DATA_DIR}/casparwater.json
pond_path: /ingest
EOF

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Create directory structure
${EXE} mkdir -p /ingest
${EXE} mkdir -p /system/run
${EXE} mkdir -p /etc

# Copy site content and templates into the pond
${EXE} copy "host:///${SITE_DIR}/content" /content
${EXE} copy "host:///${SITE_DIR}/templates" /templates
${EXE} copy "host:///${SITE_DIR}/img" /img

# Install factory nodes
${EXE} mknod logfile-ingest /etc/ingest --config-path "${INGEST_CFG}"
${EXE} mknod remote /system/run/1-backup --config-path "${SCRIPTS}/backup.yaml"
${EXE} mknod dynamic-dir /reduced --config-path "${SCRIPTS}/reduce.yaml"
${EXE} mknod dynamic-dir /analysis --config-path "${SCRIPTS}/analysis.yaml"
${EXE} mknod sitegen /etc/site --config-path "${SITE_DIR}/site.yaml"

rm -f "${INGEST_CFG}"

echo
echo "=== Water staging pond setup complete ==="
echo "Next: ./water/run.sh    # ingest data + backup"
