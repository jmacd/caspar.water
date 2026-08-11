#!/bin/sh
# teardown_script.sh -- Remove only the obsolete pre-Watertown site runner.
#
# The former DuckPond unit ran every 15 minutes from ~/duckpond.  It must not
# coexist with the native site-prod pond, but Watertown units and pond state
# are deliberately preserved so routine Terraform applies are non-destructive.
set -e

su - jmacd -c '
    XDG_RUNTIME_DIR=/run/user/$(id -u)
    export XDG_RUNTIME_DIR
    systemctl --user disable --now pond-site.timer 2>/dev/null || true
    systemctl --user stop pond-site.service 2>/dev/null || true
    rm -f "$HOME/.config/systemd/user/pond-site.timer" \
          "$HOME/.config/systemd/user/pond-site.service"
    systemctl --user daemon-reload
' || true

# setup_script.sh starts Caddy again after this resource completes.
systemctl stop caddy 2>/dev/null || true
systemctl stop nginx 2>/dev/null || true
systemctl disable nginx 2>/dev/null || true

echo Teardown complete.
