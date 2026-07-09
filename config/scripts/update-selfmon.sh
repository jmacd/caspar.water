#!/usr/bin/env bash
# update-selfmon.sh -- hourly auto-update of the natively-installed selfmon
# `pond` binary from its ghcr OCI .deb artifact.
#
# Usage: update-selfmon.sh <instance>     (e.g. watershop-selfmon)
#
# Pulls ghcr.io/jmacd/watertown/pond-deb:<DEB_CHANNEL>-<arch> with oras and
# installs it only when strictly newer than the installed `watertown` package.
# The package is public, so the pull is anonymous.  This mirrors how container
# ponds auto-update via `pond.sh --pull-image` on every timer tick, but on a
# separate hourly cadence (the selfmon tick itself fires every minute, far too
# often for a registry pull).
#
# Idempotent and NON-FATAL by design: a registry hiccup or a failed install
# must never wedge the box, so every failure path warns and exits 0, leaving
# the currently-installed binary in place (mirrors run-selfmon.sh's style).
set -uo pipefail

INSTANCE=${1:?usage: update-selfmon.sh <instance>}
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"

DEB_PACKAGE="ghcr.io/jmacd/watertown/pond-deb"

# DEB_CHANNEL selects the promotion channel to track (latest vs prod).  It is
# set per-instance in the env file; default latest, matching the selfmon
# charter of exercising the newest code first.
DEB_CHANNEL=latest
if [ -f "${ENV_FILE}" ]; then
    # shellcheck disable=SC1090
    set -a
    source "${ENV_FILE}"
    set +a
fi
DEB_CHANNEL=${DEB_CHANNEL:-latest}

case "$(uname -m)" in
    aarch64|arm64) ARCH="arm64" ;;
    x86_64|amd64)  ARCH="amd64" ;;
    *) echo "update-selfmon: unsupported arch $(uname -m); skipping" >&2; exit 0 ;;
esac

if ! command -v oras >/dev/null 2>&1; then
    echo "update-selfmon: oras not installed; skipping (run install-oras.sh)" >&2
    exit 0
fi

REF="${DEB_PACKAGE}:${DEB_CHANNEL}-${ARCH}"
TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

echo "update-selfmon: pulling ${REF}"
if ! oras pull "${REF}" --output "${TMP}"; then
    echo "update-selfmon: oras pull failed for ${REF}; skipping" >&2
    exit 0
fi

DEB=$(ls -t "${TMP}"/watertown_*_"${ARCH}".deb 2>/dev/null | head -1)
if [ -z "${DEB}" ] || [ ! -f "${DEB}" ]; then
    echo "update-selfmon: no watertown_*_${ARCH}.deb in pulled artifact; skipping" >&2
    exit 0
fi

NEW_VER=$(dpkg-deb -f "${DEB}" Version)
INSTALLED=$(dpkg-query -W -f='${Version}' watertown 2>/dev/null || echo "")

if [ -n "${INSTALLED}" ] && dpkg --compare-versions "${NEW_VER}" le "${INSTALLED}"; then
    echo "update-selfmon: installed ${INSTALLED} >= pulled ${NEW_VER} (${DEB_CHANNEL}); no-op"
    exit 0
fi

echo "update-selfmon: installing $(basename "${DEB}") (${INSTALLED:-none} -> ${NEW_VER})"
if ! sudo dpkg -i "${DEB}"; then
    echo "update-selfmon: dpkg -i failed; leaving previous binary in place" >&2
    exit 0
fi
/usr/bin/pond --version 2>/dev/null || true
