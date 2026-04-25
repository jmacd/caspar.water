#!/usr/bin/env bash
# extract-pond-binary.sh -- pull the duckpond image and copy the `pond`
# binary out of it onto the host filesystem.
#
# Usage: extract-pond-binary.sh <staging|prod> <dest>
#
# Selects the image tag matching the host architecture, like pond.sh.
# Used to install the native binary for selfmon (which runs outside
# of podman so it can read the host journal natively).
set -e

TIER=${1:?usage: extract-pond-binary.sh <staging|prod> <dest>}
DEST=${2:?usage: extract-pond-binary.sh <staging|prod> <dest>}

case "$(uname -m)" in
    aarch64|arm64) ARCH="arm64" ;;
    x86_64|amd64)  ARCH="amd64" ;;
    *)             ARCH="$(uname -m)" ;;
esac

case "${TIER}" in
    staging) TAG="latest-${ARCH}" ;;
    prod)    TAG="prod-${ARCH}" ;;
    *)       echo "ERROR: tier must be 'staging' or 'prod', got '${TIER}'"; exit 2 ;;
esac

IMAGE="ghcr.io/jmacd/duckpond/duckpond:${TAG}"

# Always pull newer; the other timers do --pull=newer per-run, so this
# won't typically download anything new.  IMPORTANT: run podman as the
# invoking (non-root) user, matching the rootless setup used by the
# other pond@*.timer instances -- otherwise we'd populate a separate
# root-owned image store.
podman pull "${IMAGE}"

# Create a stopped container, copy the binary out, remove the container.
CID=$(podman create "${IMAGE}")
trap 'podman rm "${CID}" >/dev/null 2>&1 || true' EXIT
TMP=$(mktemp)
podman cp "${CID}:/usr/local/bin/pond" "${TMP}"
# Final install to /usr/local/bin needs root.
sudo install -m 0755 "${TMP}" "${DEST}"
rm -f "${TMP}"

# Install sitegen vendor assets (DuckDB-WASM, Observable Plot, D3) at
# the path sitegen searches for them on a system install, so the
# native-binary `pond run /system/etc/sitegen build ...` finds them
# without needing DUCKPOND_VENDOR.  See
# duckpond/crates/sitegen/src/factory.rs:find_vendor_dir().  Only the
# staging extract owns this path -- prod and staging images carry the
# same vendor blobs, so it doesn't matter which one writes it.
if [ "${TIER}" = "staging" ]; then
    VENDOR_TMP=$(mktemp -d)
    if podman cp "${CID}:/usr/local/share/duckpond/vendor" "${VENDOR_TMP}/" 2>/dev/null; then
        sudo install -d -m 0755 /usr/local/share/duckpond
        sudo rm -rf /usr/local/share/duckpond/vendor
        sudo mv "${VENDOR_TMP}/vendor" /usr/local/share/duckpond/vendor
        sudo chown -R root:root /usr/local/share/duckpond/vendor
        sudo chmod -R a+r /usr/local/share/duckpond/vendor
        echo "Installed vendor assets -> /usr/local/share/duckpond/vendor"
    else
        echo "WARNING: image has no /usr/local/share/duckpond/vendor; charts will need network access"
    fi
    rm -rf "${VENDOR_TMP}"
fi

echo "Installed ${IMAGE} pond binary -> ${DEST}"
"${DEST}" --version 2>/dev/null || true
