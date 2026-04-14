#!/usr/bin/env bash
# refresh.sh -- Re-copy site content from ./site/ and regenerate.
# Use this for fast content/style iteration without re-importing data.
set -ex

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/.." && pwd)
SITE_DIR="${REPO_ROOT}/site"

source "$SCRIPTS/env.sh"

POND_BIN="${DUCKPOND_ROOT}/target/release/pond"

if [ ! -x "${POND_BIN}" ]; then
    echo "Building pond binary..."
    (cd "${DUCKPOND_ROOT}" && cargo build --release --bin pond)
fi

export POND="${SCRIPTS}/pond"

# Build host:// source lists for batch copy (one invocation per directory)
copy_dir_files() {
    local src_dir="$1" dest_dir="$2"
    local srcs=()
    for f in "${src_dir}"/*; do
        [ -f "$f" ] && srcs+=("host:///${f}")
    done
    if [ ${#srcs[@]} -gt 0 ]; then
        "${POND_BIN}" copy "${srcs[@]}" "${dest_dir}/"
    fi
}

copy_dir_files "${SITE_DIR}/content" /content
copy_dir_files "${SITE_DIR}/templates" /templates
copy_dir_files "${SITE_DIR}/img" /img

# Re-install sitegen config (picks up site.yaml changes)
"${POND_BIN}" mknod sitegen /system/etc/90-sitegen --config-path "${SITE_DIR}/site.yaml" --overwrite

# Regenerate
export RUST_BACKTRACE=1
export POND_MAX_ALLOC_MB=3000

BUILDDIR="${SCRIPTS}/build"
rm -rf "${BUILDDIR}"
mkdir -p "${BUILDDIR}"
"${POND_BIN}" run /system/etc/90-sitegen build "${BUILDDIR}"

echo
echo "=== Site refreshed ==="
echo "Output: ${BUILDDIR}"
