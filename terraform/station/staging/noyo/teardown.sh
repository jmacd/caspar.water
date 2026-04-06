#!/usr/bin/env bash
# teardown.sh -- Stop and remove the noyo staging pond.
set -ex

VOLUME=pond-noyo-staging

echo "Removing podman volume: ${VOLUME}"
podman volume rm "${VOLUME}" 2>/dev/null || true

echo "Noyo staging teardown complete."
