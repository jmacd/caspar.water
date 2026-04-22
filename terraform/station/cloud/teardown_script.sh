#!/bin/sh

# Stop duckpond timer
systemctl --user -M jmacd@ stop pond-site.timer 2>/dev/null || true
systemctl --user -M jmacd@ disable pond-site.timer 2>/dev/null || true

# Remove duckpond volume
su - jmacd -c "podman volume rm pond-site 2>/dev/null || true"

# Stop caddy (and nginx if still around from previous deploy)
systemctl stop caddy 2>/dev/null || true
systemctl stop nginx 2>/dev/null || true
systemctl disable nginx 2>/dev/null || true

echo Teardown complete.
