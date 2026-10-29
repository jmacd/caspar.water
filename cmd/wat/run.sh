#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd -- "$script_dir/../.." && pwd)"
test_dir="$repo_root/local/wat-test"
pond_dir="$repo_root/local/wat-pond"
inbox="${WAT_STATEMENT_INBOX:-$repo_root/local/wat-inbox}"
image="${WAT_LOCAL_IMAGE:-caspar-wat-pond-local}"
azure_cli=false

while [[ $# -gt 0 ]]; do
    case "$1" in
        --azure-cli)
            azure_cli=true
            shift
            ;;
        --inbox)
            if [[ $# -lt 2 ]]; then
                echo "Usage: $0 [--azure-cli] [--inbox DIRECTORY] [wat arguments...]" >&2
                exit 2
            fi
            inbox=$2
            shift 2
            ;;
        *)
            break
            ;;
    esac
done

if [[ ! -e "$test_dir" ]]; then
    mkdir -p "$repo_root/local"
    cp -R "$script_dir/example" "$test_dir"
    mkdir -p "$test_dir/sources"
fi

if [[ ! -f "$test_dir/wat.yaml" ]]; then
    printf 'Test workspace has no wat.yaml: %s\n' "$test_dir" >&2
    exit 1
fi

mkdir -p "$pond_dir" "$inbox"
inbox="$(cd -- "$inbox" && pwd -P)"

runtime="${WAT_CONTAINER_RUNTIME:-docker}"
if ! command -v "$runtime" >/dev/null 2>&1; then
    printf 'Container runtime not found: %s\n' "$runtime" >&2
    exit 1
fi

if [[ "$azure_cli" == true ]]; then
    if ! command -v az >/dev/null 2>&1; then
        echo "Azure CLI not found; install az or omit --azure-cli for non-Azure extraction." >&2
        exit 1
    fi
    AZURE_AD_TOKEN="$(az account get-access-token \
        --resource https://cognitiveservices.azure.com \
        --query accessToken -o tsv)"
    if [[ -z "$AZURE_AD_TOKEN" ]]; then
        echo "Azure CLI returned an empty Cognitive Services access token." >&2
        exit 1
    fi
    export AZURE_AD_TOKEN
fi

printf 'Accounting Pond: %s\n' "$pond_dir" >&2
printf 'Editable config: %s\n' "$test_dir/wat.yaml" >&2
printf 'Import inbox: %s (read-only)\n' "$inbox" >&2
"$runtime" build --quiet -t "$image" -f "$script_dir/Dockerfile.local" "$script_dir" >&2

run_args=(run --rm -i --privileged --network=host)
if [[ -t 0 && -t 1 ]]; then
    run_args+=(-t)
fi
if [[ -n "${AZURE_AD_TOKEN:-}" ]]; then
    run_args+=(--env AZURE_AD_TOKEN)
fi
run_args+=(
    -v "$repo_root:/work:ro"
    -v "$pond_dir:/pond"
    -v "$inbox:/wat-host-inbox:ro"
    -v wat-cargo-target:/cargo-target
    -v wat-cargo-registry:/usr/local/cargo/registry
    -v wat-go-build:/go-cache
    -v wat-go-mod:/go-mod-cache
    "$image"
)

exec "$runtime" "${run_args[@]}" "$@"
