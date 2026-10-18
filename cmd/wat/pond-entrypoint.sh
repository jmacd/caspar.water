#!/bin/bash
set -euo pipefail

cd /work
cargo build --quiet --manifest-path watertown/Cargo.toml --bin pond
go build -o /usr/local/bin/wat ./cmd/wat

pond_bin="${CARGO_TARGET_DIR}/debug/pond"
run() {
    "${pond_bin}" --pond /pond "$@"
}

if [[ ! -d /pond/data/_delta_log ]]; then
    echo "Initializing persistent wat accounting Pond." >&2
    run init --birthplace wat-local
    run mkdir -p /system/etc

    seed="$(mktemp -d /tmp/wat-seed.XXXXXX)"
    cp -a /work/local/wat-test/. "${seed}/"
    find "${seed}" -type f -name '.wat-backup-*' -delete
    rm -f "${seed}/.wat"
    run copy "host://${seed}" /accounting

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
    run mknod exec /system/etc/wat --config-path /tmp/wat-exec.yaml
elif ! run describe /system/etc/wat >/dev/null 2>&1; then
    echo "Accounting Pond initialization is incomplete; move aside /pond and retry." >&2
    exit 1
fi

exec "${pond_bin}" --pond /pond -d /wat-host-inbox run /system/etc/wat -- "$@"
