#!/bin/sh
# setup_script.sh -- Install cloud host system dependencies.
#
# Caddy serves the site and proxies InfluxDB.  The native site-prod pond
# builds directly into ${HOME}/watertown/www and is installed separately
# from the promoted Watertown deb artifact.
set -e

# Install caddy if not present
if ! command -v caddy >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | tee /etc/apt/sources.list.d/caddy-stable.list
    apt-get update -y
    apt-get install -y caddy
fi

# Install rsync for the one-time pond migration and rollback copies.
if ! command -v rsync >/dev/null 2>&1; then
    apt-get update -y
    apt-get install -y rsync
fi

# The recovery kit uses AzCopy to create complete read-only Azure Blob
# snapshots. Pin both the release and checksum so the migration runner does
# not trust a mutable download.
AZCOPY_VERSION="10.32.8"
AZCOPY_SHA256_AMD64="a95277dbc265912cefdddbaf251aa99ec648cb18ba657e8788066357a9022dc3"

case "$(uname -m)" in
    x86_64|amd64) AZCOPY_ARCH="amd64"; AZCOPY_SHA256="${AZCOPY_SHA256_AMD64}" ;;
    *) echo "Unsupported AzCopy architecture: $(uname -m)" >&2; exit 1 ;;
esac

if ! command -v azcopy >/dev/null 2>&1 || ! azcopy --version 2>/dev/null | grep -q "${AZCOPY_VERSION}"; then
    AZCOPY_TARBALL="azcopy_linux_${AZCOPY_ARCH}_${AZCOPY_VERSION}.tar.gz"
    AZCOPY_TMP=$(mktemp -d)

    curl -fsSL -o "${AZCOPY_TMP}/${AZCOPY_TARBALL}" \
      "https://github.com/Azure/azure-storage-azcopy/releases/download/v${AZCOPY_VERSION}/${AZCOPY_TARBALL}" \
      || { rm -rf "${AZCOPY_TMP}"; exit 1; }
    echo "${AZCOPY_SHA256}  ${AZCOPY_TMP}/${AZCOPY_TARBALL}" | sha256sum -c - \
      || { rm -rf "${AZCOPY_TMP}"; exit 1; }
    tar -xzf "${AZCOPY_TMP}/${AZCOPY_TARBALL}" -C "${AZCOPY_TMP}" \
      || { rm -rf "${AZCOPY_TMP}"; exit 1; }
    AZCOPY_BIN=$(find "${AZCOPY_TMP}" -type f -name azcopy -print -quit)
    test -n "${AZCOPY_BIN}" || { rm -rf "${AZCOPY_TMP}"; exit 1; }
    install -m 0755 "${AZCOPY_BIN}" /usr/local/bin/azcopy \
      || { rm -rf "${AZCOPY_TMP}"; exit 1; }
    rm -rf "${AZCOPY_TMP}"
fi

/usr/local/bin/azcopy --version

# Production source ponds remain on MinIO during this migration. Install the
# final official mc release with its published checksum so cloud-hosted
# recovery can mirror the frozen source buckets directly.
MC_RELEASE="RELEASE.2025-08-13T08-35-41Z"
MC_SHA256_AMD64="01f866e9c5f9b87c2b09116fa5d7c06695b106242d829a8bb32990c00312e891"

case "$(uname -m)" in
  x86_64|amd64) MC_ARCH="amd64"; MC_SHA256="${MC_SHA256_AMD64}" ;;
  *) echo "Unsupported mc architecture: $(uname -m)" >&2; exit 1 ;;
esac

if ! command -v mc >/dev/null 2>&1 || ! mc --version 2>/dev/null | grep -q "${MC_RELEASE}"; then
  MC_TMP=$(mktemp -d)

  curl -fsSL -o "${MC_TMP}/mc" \
    "https://github.com/minio/mc/releases/download/${MC_RELEASE}/mc.linux-${MC_ARCH}.${MC_RELEASE}" \
    || { rm -rf "${MC_TMP}"; exit 1; }
  echo "${MC_SHA256}  ${MC_TMP}/mc" | sha256sum -c - \
    || { rm -rf "${MC_TMP}"; exit 1; }
  install -m 0755 "${MC_TMP}/mc" /usr/local/bin/mc \
    || { rm -rf "${MC_TMP}"; exit 1; }
  rm -rf "${MC_TMP}"
fi

/usr/local/bin/mc --version

# Allow Caddy to traverse /home/jmacd to serve site files
chmod 711 /home/jmacd

echo Setup complete.
