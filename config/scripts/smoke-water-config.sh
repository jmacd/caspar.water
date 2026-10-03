#!/usr/bin/env bash
# Apply the checked-in water configuration to a disposable local pond.

set -euo pipefail

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
POND_BIN="${POND_BIN:-${BASE_DIR}/watertown/target/debug/pond}"
PYTHON_BIN="${PYTHON_BIN:-python3}"

if [ ! -x "${POND_BIN}" ]; then
    echo "ERROR: pond binary not found at ${POND_BIN}" >&2
    echo "       Run: make test-water-config" >&2
    exit 1
fi
if ! command -v "${PYTHON_BIN}" >/dev/null 2>&1; then
    echo "ERROR: ${PYTHON_BIN} is required to validate the monitor report" >&2
    exit 1
fi

TMP_ROOT=${TMPDIR:-/tmp}
TMP_ROOT=${TMP_ROOT%/}
WORK_DIR=$(mktemp -d "${TMP_ROOT}/water-config-smoke.XXXXXX")
cleanup() {
    if [ "${KEEP_WATER_SMOKE:-0}" = "1" ]; then
        echo "[water-config-smoke] retained ${WORK_DIR}"
    else
        rm -rf -- "${WORK_DIR}"
    fi
}
trap cleanup EXIT

export POND="${WORK_DIR}/pond"
export POND_INSTANCE="water-config-smoke"
export MONITOR_OUTPUT_DIR="${WORK_DIR}/monitor"
export RUST_LOG="${RUST_LOG:-warn}"
mkdir -p "${MONITOR_OUTPUT_DIR}"

"${POND_BIN}" init --birthplace water-config-smoke
"${POND_BIN}" apply -f "${BASE_DIR}/config/water.yaml"
"${POND_BIN}" apply -f "${BASE_DIR}/config/water.yaml"
STATUS=$("${POND_BIN}" status)
"${POND_BIN}" fsck --quick >/dev/null

PUMP_SCHEMA=$("${POND_BIN}" describe '/pump-state/*')
USAGE_SCHEMA=$("${POND_BIN}" describe '/usage/*')

require_line() {
    local output=$1
    local expected=$2
    if ! grep -Fq -- "${expected}" <<<"${output}"; then
        echo "ERROR: expected schema output to contain: ${expected}" >&2
        printf '%s\n' "${output}" >&2
        return 1
    fi
}

require_line "${PUMP_SCHEMA}" "Files found: 1"
require_line "${PUMP_SCHEMA}" "/pump-state/well-pump-state"
require_line "${PUMP_SCHEMA}" "phase: Utf8"
require_line "${STATUS}" "Recovery:        OK (no incomplete transactions)"
require_line "${USAGE_SCHEMA}" "Files found: 2"
require_line "${USAGE_SCHEMA}" "/usage/well-usage-rate"
require_line "${USAGE_SCHEMA}" "usage_gpm: Float64"
require_line "${USAGE_SCHEMA}" "/usage/well-usage-daily"
require_line "${USAGE_SCHEMA}" "gallons: Float64"
require_line "${USAGE_SCHEMA}" "pump_minutes: Int64"

"${PYTHON_BIN}" - "${MONITOR_OUTPUT_DIR}/status.json" <<'PY'
import json
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
with path.open(encoding="utf-8") as source:
    report = json.load(source)

if report.get("pond") != "water-config-smoke":
    raise SystemExit(f"unexpected monitor pond: {report.get('pond')!r}")
if report.get("state") != "unknown":
    raise SystemExit(f"empty pond monitor state must be unknown: {report.get('state')!r}")
checks = report.get("checks")
if not checks:
    raise SystemExit("monitor report contains no checks")
unexpected = [
    check.get("id")
    for check in checks
    if check.get("state") != "unknown" or check.get("sample_count") != 0
]
if unexpected:
    raise SystemExit(f"empty pond checks must be unknown with zero samples: {unexpected}")
PY

echo "[water-config-smoke] PASS"
