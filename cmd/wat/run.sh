#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../.." && pwd)"
test_dir="$repo_root/local/wat-test"

if [[ ! -e "$test_dir" ]]; then
    mkdir -p "$repo_root/local"
    cp -R "$script_dir/example" "$test_dir"
    mkdir -p "$test_dir/sources"
fi

if [[ ! -f "$test_dir/wat.yaml" ]]; then
    printf 'Test workspace has no wat.yaml: %s\n' "$test_dir" >&2
    exit 1
fi

printf 'Test workspace: %s\n' "$test_dir" >&2
cd -- "$repo_root"
exec go run ./cmd/wat --config "$test_dir/wat.yaml" "$@"
