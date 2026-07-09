#!/bin/bash
# cleanup-stale-pond-units.sh -- Remove pond systemd units that no
# longer correspond to a configured instance.
#
# Usage: cleanup-stale-pond-units.sh <name1> <name2> ...
#
# All arguments are "keep" names: any installed pond@<name>.timer or
# pond-selfmon@<name>.timer whose <name> is NOT among the arguments
# is stopped, disabled, and its unit files removed.  Units for keep
# names are left alone -- terraform's deploy_staging/deploy_production
# flags decide which keep-named timers are subsequently (re-)enabled,
# but timers for keep-named instances that aren't being deployed in
# this apply are NOT touched (so toggling one tier off does not
# disturb the other tier's running state).
set -e

is_kept() {
    local name="$1"
    shift
    for k in "$@"; do
        [ "$name" = "$k" ] && return 0
    done
    return 1
}

KEEP=("$@")

systemctl --user list-units --all --no-legend 'pond@*.timer' 'pond-selfmon@*.timer' 'pond-selfmon-update@*.timer' 2>/dev/null \
    | awk '{print $1}' \
    | while read -r unit; do
        # Strip prefix and .timer suffix to recover the instance name.
        name=$(echo "$unit" | sed -E 's/^pond(-selfmon(-update)?)?@(.*)\.timer$/\3/')
        if ! is_kept "$name" "${KEEP[@]}"; then
            echo "Disabling stale unit: $unit (name=$name)"
            systemctl --user disable --now "$unit" 2>/dev/null || true
        fi
    done

# Remove unit files for stale names, leaving the kept ones in place.
shopt -s nullglob
for f in "$HOME/.config/systemd/user"/pond@*.timer \
         "$HOME/.config/systemd/user"/pond-selfmon@*.timer \
         "$HOME/.config/systemd/user"/pond-selfmon-update@*.timer \
         "$HOME/.config/systemd/user"/pond@*.service \
         "$HOME/.config/systemd/user"/pond-selfmon@*.service \
         "$HOME/.config/systemd/user"/pond-selfmon-update@*.service; do
    base=$(basename "$f")
    # pond@.service / pond-selfmon@.service / pond-selfmon-update@.{service,timer}
    # are the templates -- keep them.
    case "$base" in
        pond@.service|pond-selfmon@.service) continue ;;
        pond-selfmon-update@.service|pond-selfmon-update@.timer) continue ;;
    esac
    name=$(echo "$base" | sed -E 's/^pond(-selfmon(-update)?)?@(.*)\.(timer|service)$/\3/')
    if ! is_kept "$name" "${KEEP[@]}"; then
        echo "Removing stale unit file: $base (name=$name)"
        rm -f "$f"
    fi
done

systemctl --user daemon-reload 2>/dev/null || true
