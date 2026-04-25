#!/usr/bin/env bash
# install-duckpond.sh -- install the duckpond .deb that was previously
# built on this host (typically by tools/build-on-watershop.sh) and
# create the per-tier pond-selfmon-<tier> alias the systemd unit
# template references.
#
# Usage:
#   install-duckpond.sh <staging|prod>
#
# Reads the version for <tier> from config/duckpond-version.toml
# (relative to the caspar.water repo this script lives in), locates
# ~/duckpond/target/debian/duckpond_<ver>_arm64.deb, runs `dpkg -i`,
# and symlinks /usr/local/bin/pond-selfmon-<tier> to /usr/bin/pond.
#
# Replaces the older extract-pond-binary.sh, which copied a binary
# out of a podman image.  Now the binary (and sitegen vendor blobs)
# come from a real Debian package -- versionable, removable,
# inspectable with dpkg.
set -euo pipefail

TIER=${1:?usage: install-duckpond.sh <staging|prod>}

case "${TIER}" in
    staging|prod) ;;
    *) echo "ERROR: tier must be 'staging' or 'prod', got '${TIER}'" >&2; exit 2 ;;
esac

case "$(uname -m)" in
    aarch64|arm64) ARCH="arm64" ;;
    x86_64|amd64)  ARCH="amd64" ;;
    *) echo "ERROR: unsupported arch $(uname -m)" >&2; exit 2 ;;
esac

REPO_ROOT=$(cd "$(dirname "$0")/../.." && pwd)
VERSION_FILE="${REPO_ROOT}/config/duckpond-version.toml"
DUCKPOND_DIR=${DUCKPOND_DIR:-${HOME}/duckpond}
DEB_DIR="${DUCKPOND_DIR}/target/debian"
ALIAS="/usr/local/bin/pond-selfmon-${TIER}"

if [ ! -f "${VERSION_FILE}" ]; then
    echo "ERROR: ${VERSION_FILE} not found" >&2
    exit 1
fi

# Tiny TOML extractor: find [selfmon.<tier>] section, then the
# version = "..." line within it.  Avoids a python/toml dep.
VERSION=$(awk -v section="[selfmon.${TIER}]" '
    $0 == section { in_sec = 1; next }
    in_sec && /^\[/ { in_sec = 0 }
    in_sec && /^[[:space:]]*version[[:space:]]*=/ {
        gsub(/.*=[[:space:]]*"/, ""); gsub(/".*/, ""); print; exit
    }
' "${VERSION_FILE}")

if [ -z "${VERSION}" ]; then
    echo "ERROR: no version pinned for [selfmon.${TIER}] in ${VERSION_FILE}" >&2
    exit 1
fi

DEB="${DEB_DIR}/duckpond_${VERSION}_${ARCH}.deb"
if [ ! -f "${DEB}" ]; then
    echo "ERROR: ${DEB} not found" >&2
    echo "       Build it first:  tools/build-on-watershop.sh" >&2
    exit 1
fi

# Idempotent: only reinstall if the package is missing or at a
# different version.  Saves a few seconds on terraform reapplies.
INSTALLED=$(dpkg-query -W -f='${Version}' duckpond 2>/dev/null || echo "")
if [ "${INSTALLED}" != "${VERSION}" ]; then
    echo "Installing ${DEB} (was: ${INSTALLED:-none})"
    sudo dpkg -i "${DEB}"
else
    echo "duckpond ${VERSION} already installed"
fi

# Per-tier alias so existing pond-selfmon@<inst>.service unit (which
# ExecStart's /usr/local/bin/pond-selfmon-${TIER}) works unchanged.
sudo ln -sfn /usr/bin/pond "${ALIAS}"
echo "Linked ${ALIAS} -> /usr/bin/pond"

"${ALIAS}" --version 2>/dev/null || true
