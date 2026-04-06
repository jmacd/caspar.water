#!/usr/bin/env bash
# generate.sh -- Build the combined site from imported sources.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
STAGING_DIR=$(dirname "$SCRIPTS")
EXE="${SCRIPTS}/pond.sh"

OUTDIR="${STAGING_DIR}/dist"

# Clear and recreate output dir
rm -rf "${OUTDIR}"
mkdir -p "${OUTDIR}"

# Run sitegen build — output goes to bind-mounted dist/
# We need to re-run with an extra mount for the output directory
REPO_ROOT=$(dirname "$(dirname "$(dirname "$STAGING_DIR")")")

source "$STAGING_DIR/env.sh"

VOLUME=pond-site-staging

podman run --pull=newer -ti --rm \
    -v "${VOLUME}:/pond" \
    -v "${REPO_ROOT}/site:/root/site:ro" \
    -v "${SCRIPTS}:/root/config:ro" \
    -v "${OUTDIR}:/output" \
    -e POND=/pond \
    -e R2_ENDPOINT="${MINIO_ENDPOINT}" \
    -e R2_KEY="${MINIO_ACCESS_KEY}" \
    -e R2_SECRET="${MINIO_SECRET_KEY}" \
    "${IMAGE}" run /system/etc/90-sitegen build /output

echo
echo "Site generated at: ${OUTDIR}"
echo "To preview: cd ${OUTDIR} && python3 -m http.server 4180"
