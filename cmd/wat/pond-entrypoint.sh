#!/bin/bash
set -euo pipefail

cd /work
cargo build --quiet --manifest-path watertown/Cargo.toml --bin pond
go build -o /usr/local/bin/wat ./cmd/wat

pond_bin="${CARGO_TARGET_DIR}/debug/pond"
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
    run init --birthplace wat-local
    run mkdir -p /system/etc

    seed="$(mktemp -d /tmp/wat-seed.XXXXXX)"
    cp -a /work/local/wat-test/. "${seed}/"
    find "${seed}" -type f -name '.wat-backup-*' -delete
    rm -f "${seed}/.wat"
    run copy "host://${seed}" /accounting
fi

sync_input() {
    local source=$1
    local target=$2
    local current
    current="$(mktemp /tmp/wat-input.XXXXXX)"
    if ! run cat "$target" >"$current" 2>/dev/null || ! cmp -s "$source" "$current"; then
        echo "Updating Pond input from ${source#/work/}." >&2
        run copy "host://${source}" "$target"
    fi
    rm -f "$current"
}

sync_input /work/local/wat-test/wat.yaml /accounting/wat.yaml
sync_input /work/local/wat-test/books/chart.journal /accounting/books/chart.journal
run mknod exec /system/etc/wat --config-path /tmp/wat-exec.yaml --overwrite

exec "${pond_bin}" --pond /pond -d /wat-host-inbox run /system/etc/wat -- "$@"
