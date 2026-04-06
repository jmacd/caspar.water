#!/usr/bin/env bash
# teardown.sh -- Stop and remove the site staging pond.
set -ex

VOLUME=pond-site-staging

echo "Removing podman volume: ${VOLUME}"
podman volume rm "${VOLUME}" 2>/dev/null || true

echo "Site staging teardown complete."
