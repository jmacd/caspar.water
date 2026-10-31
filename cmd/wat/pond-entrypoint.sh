#!/bin/bash
set -euo pipefail

cd /work
cargo build --quiet --manifest-path watertown/Cargo.toml --bin pond
go build -o /usr/local/bin/wat ./cmd/wat

pond_bin="${CARGO_TARGET_DIR}/debug/pond"
install -m 0755 "${pond_bin}" /usr/local/bin/pond
run() {
    "${pond_bin}" --pond /pond "$@"
}

cat >/tmp/wat-exec.yaml <<'EOF'
program: /usr/local/bin/wat-sandbox
args: ["--config", "accounting/wat.yaml", "--statement-inbox", "/inbox"]
inputs: []
outputs: ["/accounting/"]
host_inputs:
  - source: /
    target: /inbox
interactive: true
network: true
EOF

if [[ ! -d /pond/data/_delta_log ]]; then
    echo "Initializing persistent wat accounting Pond." >&2
    run init --birthplace wat-local >&2
    run mkdir -p /system/etc >&2

    seed="$(mktemp -d /tmp/wat-seed.XXXXXX)"
    cp -a /work/local/wat-test/. "${seed}/"
    find "${seed}" -type f -name '.wat-backup-*' -delete
    rm -f "${seed}/.wat"
    run copy "host://${seed}" /accounting >&2
fi

sync_input() {
    local source=$1
    local target=$2
    local current
    current="$(mktemp /tmp/wat-input.XXXXXX)"
    if ! run cat "$target" >"$current" 2>/dev/null || ! cmp -s "$source" "$current"; then
        echo "Updating Pond input from ${source#/work/}." >&2
        run copy "host://${source}" "$target" >&2
    fi
    rm -f "$current"
}

sync_input /work/local/wat-test/wat.yaml /accounting/wat.yaml
chart_paths_file="$(mktemp /tmp/wat-chart-defs.XXXXXX)"
if ! /usr/local/bin/wat \
    --config /work/local/wat-test/wat.yaml \
    config-path chart-defs >"${chart_paths_file}"; then
    rm -f "${chart_paths_file}"
    exit 1
fi
mapfile -t chart_paths <"${chart_paths_file}"
rm -f "${chart_paths_file}"
if [[ ${#chart_paths[@]} -eq 0 ]]; then
    echo "Wat configuration returned no chart_defs." >&2
    exit 1
fi
for chart_path in "${chart_paths[@]}"; do
    run mkdir -p "$(dirname "/accounting/${chart_path}")" >&2
    sync_input \
        "/work/local/wat-test/${chart_path}" \
        "/accounting/${chart_path}"
done
run mknod exec /system/etc/wat --config-path /tmp/wat-exec.yaml --overwrite >&2

exec "${pond_bin}" --pond /pond -d /wat-host-inbox run /system/etc/wat -- "$@"
