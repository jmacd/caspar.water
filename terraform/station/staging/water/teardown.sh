#!/usr/bin/env bash
# teardown.sh -- Stop and remove the water staging pond.
set -ex

VOLUME=pond-water-staging

echo "Removing podman volume: ${VOLUME}"
podman volume rm "${VOLUME}" 2>/dev/null || true

echo "Water staging teardown complete."
