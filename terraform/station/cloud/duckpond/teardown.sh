#!/usr/bin/env bash
# teardown.sh -- Stop and remove the site pond.
set -ex

VOLUME=pond-site
podman volume rm "${VOLUME}" 2>/dev/null || true
echo "Cloud site pond teardown complete."
