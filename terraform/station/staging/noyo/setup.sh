#!/usr/bin/env bash
# setup.sh -- Initialize the noyo staging pond.
#
# Creates a local pond directory and installs factory nodes
# for HydroVu collection.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

source "$STAGING_DIR/env.sh"

# Wipe and initialize
rm -rf "${SCRIPTS}/pond"
${EXE} init

# Create directory structure
${EXE} mkdir -p /system/run
${EXE} mkdir -p /system/etc

# Install factory nodes
${EXE} mknod remote /system/run/1-backup --config-path "${SCRIPTS}/backup.yaml"
${EXE} mknod hydrovu /system/etc/20-hydrovu --config-path "${SCRIPTS}/hydrovu.yaml"
${EXE} mknod column-rename /system/etc/10-hrename --config-path "${SCRIPTS}/hrename.yaml"
${EXE} mknod dynamic-dir /combined --config-path "${SCRIPTS}/combine.yaml"
${EXE} mknod dynamic-dir /singled --config-path "${SCRIPTS}/single.yaml"
${EXE} mknod dynamic-dir /reduced --config-path "${SCRIPTS}/reduce.yaml"

echo
echo "=== Noyo staging pond setup complete ==="
echo "Next: ./noyo/run.sh    # collect from HydroVu + backup"
