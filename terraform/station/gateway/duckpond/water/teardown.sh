#!/usr/bin/env bash
# teardown.sh -- Stop and remove the water pond.
set -ex

VOLUME=pond-water
podman volume rm "${VOLUME}" 2>/dev/null || true
echo "Water pond teardown complete."
