#!/usr/bin/env bash
# install-watertown.sh -- install the watertown .deb that was previously
# built on this host (typically by tools/build-on-watershop.sh).
#
# Usage:
#   install-watertown.sh
#
# Always installs the newest .deb in ~/src/watertown/target/debian/,
# regardless of currently-installed version.  Each run of
# tools/build-on-watershop.sh produces a `.deb` that the next
# `terraform apply` picks up unconditionally.  No version pinning,
# no manual bump.  dpkg -i over an identical version is a cheap
# no-op overwrite.
#
# This binary (`/usr/bin/pond`) is meant for the local-experimental
# `watershop-selfmon` pond only.  Production data ponds run from
# GH-Actions-built podman images (see config/scripts/pond.sh).
set -euo pipefail

case "$(uname -m)" in
    aarch64|arm64) ARCH="arm64" ;;
    x86_64|amd64)  ARCH="amd64" ;;
    *) echo "ERROR: unsupported arch $(uname -m)" >&2; exit 2 ;;
esac

WATERTOWN_DIR=${WATERTOWN_DIR:-${HOME}/src/watertown}
DEB_DIR="${WATERTOWN_DIR}/target/debian"

DEB=$(ls -t "${DEB_DIR}"/watertown_*_"${ARCH}".deb 2>/dev/null | head -1)
if [ -z "${DEB}" ] || [ ! -f "${DEB}" ]; then
    echo "ERROR: no watertown_*_${ARCH}.deb in ${DEB_DIR}" >&2
    echo "       Build it first:  tools/build-on-watershop.sh" >&2
    exit 1
fi

INSTALLED=$(dpkg-query -W -f='${Version}' watertown 2>/dev/null || echo none)
NEW_VER=$(dpkg-deb -f "${DEB}" Version)
echo "Installing $(basename "${DEB}") (${INSTALLED} -> ${NEW_VER})"
sudo dpkg -i "${DEB}"

/usr/bin/pond --version 2>/dev/null || true
