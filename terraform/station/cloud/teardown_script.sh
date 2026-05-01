#!/bin/sh
# teardown_script.sh -- Stop services before redeployment, and remove
# any residual duckpond state from previous deployments that ran a
# pond on this host.  The cloud host is now caddy + rsync target only;
# the pond@site-prod that used to live here was orphaned and was the
# source of the R2 bandwidth bleed (cf. remote-bandwidth-bug.md).
set -e

# Stop and disable any pond@*.timer left over from older deployments.
# Use list-units so this is a no-op on a clean host.
su - jmacd -c "
    XDG_RUNTIME_DIR=/run/user/\$(id -u)
    export XDG_RUNTIME_DIR
    systemctl --user list-units --all --no-legend 'pond@*.timer' \
        | awk '{print \$1}' \
        | xargs -r systemctl --user disable --now 2>/dev/null || true
    systemctl --user list-units --all --no-legend 'pond@*.service' \
        | awk '{print \$1}' \
        | xargs -r systemctl --user stop 2>/dev/null || true
    rm -f \$HOME/.config/systemd/user/pond@*.timer \
          \$HOME/.config/systemd/user/pond@*.service
    systemctl --user daemon-reload 2>/dev/null || true
" || true

# Remove the pond data volume if present.
su - jmacd -c "podman volume rm pond-site-prod 2>/dev/null || true"

# Stop web servers
systemctl stop caddy 2>/dev/null || true
systemctl stop nginx 2>/dev/null || true
systemctl disable nginx 2>/dev/null || true

echo Teardown complete.
