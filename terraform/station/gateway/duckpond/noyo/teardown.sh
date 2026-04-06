#!/usr/bin/env bash
# teardown.sh -- Stop and remove the noyo pond.
set -ex

VOLUME=pond-noyo
podman volume rm "${VOLUME}" 2>/dev/null || true
echo "Noyo pond teardown complete."
