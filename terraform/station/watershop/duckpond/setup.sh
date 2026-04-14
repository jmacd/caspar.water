#!/usr/bin/env bash
# setup.sh -- Initialize a duckpond instance (run once).
#
# Usage: setup.sh <instance>
set -ex

INSTANCE=$1
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
EXE="${SCRIPTS}/pond.sh"

# Extract pond type from instance name (e.g., noyo-staging → noyo)
TYPE="${INSTANCE%-staging}"
TYPE="${TYPE%-prod}"

echo "=== Setting up ${INSTANCE} (type: ${TYPE}) ==="

# Initialize pond (skip if volume already has data)
if podman volume exists "pond-${INSTANCE}" 2>/dev/null; then
    echo "Volume pond-${INSTANCE} already exists, skipping init"
else
    ${EXE} "${INSTANCE}" init
fi

# Common directories
${EXE} "${INSTANCE}" mkdir -p /system/etc
${EXE} "${INSTANCE}" mkdir -p /system/run

case "${TYPE}" in
    noyo)
        # Copy historical data from NFS archive
        if [ -n "${NOYO_ARCHIVE_DIR}" ]; then
            ${EXE} "${INSTANCE}" mkdir -p /laketech
            ${EXE} "${INSTANCE}" copy "host://${NOYO_ARCHIVE_DIR}/laketech" /laketech/data
            if [ -d "${NOYO_ARCHIVE_DIR}/hydrovu" ]; then
                ${EXE} "${INSTANCE}" copy "host://${NOYO_ARCHIVE_DIR}/hydrovu" /hydrovu
            fi
        fi
        # Copy site content
        ${EXE} "${INSTANCE}" copy host:///config/noyo/site /system/site
        ${EXE} "${INSTANCE}" mknod remote /system/run/1-backup --config-path /config/backup.yaml
        ${EXE} "${INSTANCE}" mknod hydrovu /system/etc/20-hydrovu --config-path /config/noyo/hydrovu.yaml
        ${EXE} "${INSTANCE}" mknod column-rename /system/etc/10-hrename --config-path /config/noyo/hrename.yaml
        ${EXE} "${INSTANCE}" mknod dynamic-dir /combined --config-path /config/noyo/combine.yaml
        ${EXE} "${INSTANCE}" mknod dynamic-dir /singled --config-path /config/noyo/single.yaml
        ${EXE} "${INSTANCE}" mknod dynamic-dir /reduced --config-path /config/noyo/reduce.yaml
        ${EXE} "${INSTANCE}" mknod sitegen /system/etc/90-sitegen --config-path /config/noyo/site.yaml
        ;;
    water)
        ${EXE} "${INSTANCE}" mkdir -p /ingest
        ${EXE} "${INSTANCE}" mkdir -p /etc
        # Copy site content
        ${EXE} "${INSTANCE}" copy host:///config/water/site /site
        ${EXE} "${INSTANCE}" mknod logfile-ingest /etc/ingest --config-path /config/water/ingest.yaml
        ${EXE} "${INSTANCE}" mknod remote /system/run/1-backup --config-path /config/backup.yaml
        ${EXE} "${INSTANCE}" mknod dynamic-dir /reduced --config-path /config/water/reduce.yaml
        ${EXE} "${INSTANCE}" mknod dynamic-dir /analysis --config-path /config/water/analysis.yaml
        ${EXE} "${INSTANCE}" mknod sitegen /system/etc/90-sitegen --config-path /config/water/site.yaml
        ;;
    septic)
        ${EXE} "${INSTANCE}" mkdir -p /ingest
        ${EXE} "${INSTANCE}" mkdir -p /etc
        # Copy site content
        ${EXE} "${INSTANCE}" copy host:///config/septic/site /etc/site
        ${EXE} "${INSTANCE}" mknod logfile-ingest /etc/ingest --config-path /config/septic/ingest.yaml
        ${EXE} "${INSTANCE}" mknod remote /system/run/1-backup --config-path /config/backup.yaml
        ${EXE} "${INSTANCE}" mknod dynamic-dir /reduced --config-path /config/septic/reduce.yaml
        ${EXE} "${INSTANCE}" mknod sitegen /etc/site.yaml --config-path /config/septic/site.yaml
        ;;
    site)
        ${EXE} "${INSTANCE}" mkdir -p /sources
        # Copy site content into pond
        ${EXE} "${INSTANCE}" copy host:///site/content /content
        ${EXE} "${INSTANCE}" copy host:///site/templates /templates
        ${EXE} "${INSTANCE}" copy host:///site/img /img
        # Install imports
        ${EXE} "${INSTANCE}" mknod remote /system/etc/10-water --config-path /site/import-water.yaml
        ${EXE} "${INSTANCE}" mknod remote /system/etc/11-noyo --config-path /site/import-noyo.yaml
        ${EXE} "${INSTANCE}" mknod remote /system/etc/12-septic --config-path /site/import-septic.yaml
        # Install sitegen
        ${EXE} "${INSTANCE}" mknod sitegen /system/etc/90-sitegen --config-path /site/site.yaml
        ;;
    *)
        echo "ERROR: Unknown pond type '${TYPE}'"
        exit 1
        ;;
esac

echo "=== ${INSTANCE} setup complete ==="
