#!/bin/sh

# Stop duckpond timers
systemctl --user -M jmacd@ stop pond-water.timer 2>/dev/null || true
systemctl --user -M jmacd@ stop pond-noyo.timer 2>/dev/null || true
systemctl --user -M jmacd@ disable pond-water.timer 2>/dev/null || true
systemctl --user -M jmacd@ disable pond-noyo.timer 2>/dev/null || true

# Remove duckpond volumes
su - jmacd -c "podman volume rm pond-water 2>/dev/null || true"
su - jmacd -c "podman volume rm pond-noyo 2>/dev/null || true"

# Stop collector
systemctl stop collector

echo Teardown complete.
