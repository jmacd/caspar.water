#!/usr/bin/env bash
# install-oras.sh -- install a pinned `oras` binary to /usr/local/bin.
#
# Idempotent: a no-op if the pinned version is already present.  Used by the
# selfmon deb auto-update path: update-selfmon.sh pulls the pond .deb OCI
# artifact (ghcr.io/jmacd/watertown/pond-deb) with oras.  The version and
# checksums are pinned to avoid supply-chain drift; bump them together with
# the ORAS_VERSION env in the watertown repo's rust-ci.yml so the box and CI
# agree on the tool that pushes/pulls the artifact.
set -euo pipefail

ORAS_VERSION="${ORAS_VERSION:-1.3.2}"
SHA_AMD64="9229ccc6d17bb282039ad4a69abb16dcb887a5bce567c075d731d9b3c7ad8eaf"
SHA_ARM64="8db4a223bd6034deff198e791ea7cb3af0840df25b7e9f370e2f1f3fd20d389b"

case "$(uname -m)" in
    aarch64|arm64) ARCH="arm64"; SHA="${SHA_ARM64}" ;;
    x86_64|amd64)  ARCH="amd64"; SHA="${SHA_AMD64}" ;;
    *) echo "install-oras: unsupported arch $(uname -m)" >&2; exit 1 ;;
esac

if command -v oras >/dev/null 2>&1 && oras version 2>/dev/null | grep -qw "${ORAS_VERSION}"; then
    echo "install-oras: oras ${ORAS_VERSION} already installed"
    exit 0
fi

TARBALL="oras_${ORAS_VERSION}_linux_${ARCH}.tar.gz"
TMP=$(mktemp -d)
trap 'rm -rf "${TMP}"' EXIT

curl -fsSL -o "${TMP}/${TARBALL}" \
    "https://github.com/oras-project/oras/releases/download/v${ORAS_VERSION}/${TARBALL}"
echo "${SHA}  ${TMP}/${TARBALL}" | sha256sum -c -
tar -xzf "${TMP}/${TARBALL}" -C "${TMP}" oras
sudo install -m 0755 "${TMP}/oras" /usr/local/bin/oras
/usr/local/bin/oras version
