#!/usr/bin/env bash
# Usage: sync-selfmon-templates.sh <pond binary> <absolute host template directory>
set -e

pond=$1
shopt -s nullglob
templates=("$2"/*.md)
((${#templates[@]})) || exit 0

work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
# Include the directory itself: copy-out skips directories, but needs at
# least one match on the first tick, before any templates exist in the pond.
"$pond" copy --strip-prefix /system/site /system/site '/system/site/*.md' "host://$work"

changed=()
for src in "${templates[@]}"; do
    [[ -f $src ]] || continue
    previous="$work/${src##*/}"
    if [[ -f $previous ]]; then
        if cmp -s "$previous" "$src"; then
            continue
        elif [[ $? != 1 ]]; then
            echo "ERROR: could not compare selfmon template: $src" >&2
            exit 1
        fi
    fi
    changed+=("host://$src")
done

if ((${#changed[@]})); then
    "$pond" copy "${changed[@]}" /system/site/
fi
printf 'selfmon_templates copied=%d elapsed_s=%d\n' "${#changed[@]}" "$SECONDS"
