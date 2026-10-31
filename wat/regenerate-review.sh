#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 || ! $1 =~ ^[0-9]{4}$ ]]; then
    echo "Usage: $0 NNNN" >&2
    exit 2
fi

run=$1
script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "${script_dir}/.." && pwd)"
review_dir="${script_dir}/review"
inbox="${WAT_QUICKBOOKS_INBOX:-${repo_root}/../caspar_water_bank}"
original="${review_dir}/2021-ai-original-${run}.journal"
log="${review_dir}/2021-generation-${run}.log"
working="${review_dir}/2021-working-${run}.journal"

for path in "$original" "$log" "$working"; do
    if [[ -e $path ]]; then
        echo "Refusing to overwrite existing review file: $path" >&2
        exit 1
    fi
done

mkdir -p "$review_dir"
echo "Generating 2021 review run ${run} with the current Watertown integration runner." >&2
if (
    cd "$repo_root"
    ./cmd/wat/run.sh --azure-cli --inbox "$inbox" \
        quickbooks --year 2021
) >"$original" 2>"$log"; then
    cp -p "$original" "$working"
    printf 'Created:\n  %s\n  %s\n  %s\n' "$original" "$log" "$working"
else
    status=$?
    echo "Generation failed with status ${status}; inspect $log" >&2
    exit "$status"
fi
