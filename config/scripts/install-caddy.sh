#!/usr/bin/env bash
# Pinned upstream ARM64 package for the Watershop beacon directives.
set -euo pipefail
VERSION=2.10.2
SHA=b3974f22bc624fa56926caab03e56031a94c2a14cba5b9d1d31c2c128ce0f75a
case "$(uname -m)" in
    aarch64|arm64) ;;
    *) echo "ERROR: Watershop Caddy package requires ARM64" >&2; exit 1 ;;
esac
if caddy version | grep -Eq "^v?${VERSION//./\\.}([[:space:]]|$)"; then
    exit 0
fi
WORK=$(mktemp -d)
trap 'rm -r -- "${WORK}"' EXIT
PACKAGE="caddy_${VERSION}_linux_arm64.deb"
curl --fail --silent --show-error --location \
    "https://github.com/caddyserver/caddy/releases/download/v${VERSION}/${PACKAGE}" \
    -o "${WORK}/${PACKAGE}"
printf '%s  %s\n' "${SHA}" "${WORK}/${PACKAGE}" | sha256sum -c -
sudo dpkg --force-confold -i "${WORK}/${PACKAGE}"
caddy version
