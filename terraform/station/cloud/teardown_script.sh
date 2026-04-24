#!/bin/sh
# teardown_script.sh -- Stop services before redeployment.
set -e

# Stop duckpond timer (uses common pond@.service template)
su - jmacd -c "systemctl --user stop 'pond@site-prod.timer' 2>/dev/null || true"
su - jmacd -c "systemctl --user disable 'pond@site-prod.timer' 2>/dev/null || true"

# Stop web servers
systemctl stop caddy 2>/dev/null || true
systemctl stop nginx 2>/dev/null || true
systemctl disable nginx 2>/dev/null || true

echo Teardown complete.
