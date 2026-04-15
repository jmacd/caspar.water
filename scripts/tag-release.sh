#!/usr/bin/env bash
# tag-release.sh -- Tag the duckpond submodule with the version from DUCKPOND_VERSION.
#
# Reads config/DUCKPOND_VERSION, creates a git tag in the duckpond
# submodule, and pushes it to trigger CI to build versioned images.
#
# Usage: ./scripts/tag-release.sh
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO_ROOT=$(cd "${SCRIPTS}/.." && pwd)
VERSION=$(cat "${REPO_ROOT}/config/DUCKPOND_VERSION" | tr -d '[:space:]')

if [ -z "${VERSION}" ]; then
    echo "ERROR: config/DUCKPOND_VERSION is empty"
    exit 1
fi

TAG="v${VERSION}"

cd "${REPO_ROOT}/duckpond"

# Check if tag already exists
if git rev-parse "${TAG}" &>/dev/null; then
    echo "Tag ${TAG} already exists in duckpond"
    echo "To re-tag, delete it first: git -C duckpond tag -d ${TAG} && git -C duckpond push origin :${TAG}"
    exit 1
fi

echo "Tagging duckpond at $(git rev-parse --short HEAD) as ${TAG}"
git tag "${TAG}"
git push origin "${TAG}"

echo
echo "=== Tagged ${TAG} ==="
echo "CI will build and push:"
echo "  ghcr.io/jmacd/duckpond/duckpond:${TAG}-amd64"
echo "  ghcr.io/jmacd/duckpond/duckpond:${TAG}-arm64"
echo
echo "Monitor CI: https://github.com/jmacd/duckpond/actions"
