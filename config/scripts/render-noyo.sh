#!/usr/bin/env bash
# render-noyo.sh -- Build and preview the Noyo Harbor site locally from the
# in-repo config, with no deploy and no remote pond.
#
# It stands up a throwaway local *producer* pond (init + `pond apply
# config/noyo.yaml`) at root, renders `/system/etc/90-sitegen`, and serves
# the result under its `/noyo-harbor/` base_url. Source data comes from the
# git-ingested HydroVu archive (history); pass --collect to also pull the
# live tail. This exercises the local sitegen config, the site markdown
# under config/noyo/site/, and the locally built `pond` binary together.
#
# Usage:
#   config/scripts/render-noyo.sh [--fresh] [--collect] [--no-serve] [--port N]
#
#   --fresh      Wipe the work pond and re-init+apply (default: reuse if present)
#   --collect    Run `20-hydrovu collect` to back-fill the recent tail (needs
#                HYDRO_KEY_* in the env file)
#   --no-serve   Build only; do not start the preview HTTP server
#   --port N     Preview port (default: 8802)
#
# Environment overrides (all optional):
#   POND_BIN     Path to the pond binary (default: watertown/target/release/pond,
#                falling back to `pond` on PATH)
#   GIT_REF      Branch/ref git-ingest pulls from (default: current branch).
#                NOTE: git-ingest fetches from the remote, so commit and PUSH
#                the branch for your config/noyo/site/*.md edits to appear.
#   NOYO_RENDER_DIR  Work directory (default: /tmp/noyo-render)
#   NOYO_ENV_FILE    Env file sourced for HydroVu creds / GIT_REF
#                    (default: terraform/station/watershop/env/noyo-staging.env)
set -e

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)

# --- args ---
FRESH=0
COLLECT=0
SERVE=1
PORT=8802
while [ $# -gt 0 ]; do
    case "$1" in
        --fresh)    FRESH=1 ;;
        --collect)  COLLECT=1 ;;
        --no-serve) SERVE=0 ;;
        --port)     PORT=$2; shift ;;
        -h|--help)  sed -n '2,30p' "$0"; exit 0 ;;
        *) echo "ERROR: unknown arg '$1'" >&2; exit 2 ;;
    esac
    shift
done

# --- capture caller overrides before sourcing the env file (which sets
# POND/GIT_REF for the deployed instance, not for this local render) ---
CALLER_GIT_REF="${GIT_REF:-}"

# --- locate the pond binary ---
POND_BIN="${POND_BIN:-${BASE_DIR}/watertown/target/release/pond}"
if [ ! -x "${POND_BIN}" ]; then
    if command -v pond >/dev/null 2>&1; then
        POND_BIN=$(command -v pond)
    else
        echo "ERROR: pond binary not found at ${POND_BIN} and not on PATH." >&2
        echo "       Build it: (cd ${BASE_DIR}/watertown && cargo build -p cmd --release)" >&2
        exit 1
    fi
fi
pond() { "${POND_BIN}" "$@"; }

# --- env: HydroVu creds + GIT_REF default (optional) ---
ENV_FILE="${NOYO_ENV_FILE:-${BASE_DIR}/terraform/station/watershop/env/noyo-staging.env}"
if [ -f "${ENV_FILE}" ]; then
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a
else
    echo "[render-noyo] note: ${ENV_FILE} not found; --collect will be unavailable" >&2
fi

# --- local render config (override whatever the env file set) ---
WORKDIR="${NOYO_RENDER_DIR:-/tmp/noyo-render}"
export POND="${WORKDIR}/pond"
export SITE_BASE_URL="/noyo-harbor/"
GIT_REF="${CALLER_GIT_REF:-${GIT_REF:-$(git -C "${BASE_DIR}" rev-parse --abbrev-ref HEAD)}}"
export GIT_REF
OUT="${WORKDIR}/out"

echo "[render-noyo] pond=${POND_BIN}"
echo "[render-noyo] GIT_REF=${GIT_REF}  (git-ingest fetches this ref from the remote; push your branch)"
echo "[render-noyo] work=${WORKDIR}"

# --- init + apply (skip if reusing an existing pond) ---
if [ "${FRESH}" -eq 1 ]; then
    rm -rf "${WORKDIR}"
fi
if [ ! -d "${POND}/data" ]; then
    mkdir -p "${WORKDIR}"
    echo "[render-noyo] init + apply config/noyo.yaml"
    pond init --birthplace noyo-render
    pond apply -f "${BASE_DIR}/config/noyo.yaml"
else
    echo "[render-noyo] reusing existing pond (use --fresh to rebuild); re-applying config"
    pond apply -f "${BASE_DIR}/config/noyo.yaml"
fi

# --- optional live data ---
if [ "${COLLECT}" -eq 1 ]; then
    echo "[render-noyo] 20-hydrovu collect (live tail)"
    pond run /system/etc/20-hydrovu collect
fi

# --- render ---
echo "[render-noyo] building site -> ${OUT}"
pond run /system/etc/90-sitegen build "${OUT}"

# --- serve under /noyo-harbor/ (matches base_url so links resolve) ---
if [ "${SERVE}" -eq 1 ]; then
    SERVE_DIR="${WORKDIR}/serve"
    mkdir -p "${SERVE_DIR}"
    ln -sfn "${OUT}" "${SERVE_DIR}/noyo-harbor"
    URL="http://localhost:${PORT}/noyo-harbor/index.html"
    echo "[render-noyo] serving at ${URL}  (Ctrl-C to stop)"
    case "$(uname)" in
        Darwin) (sleep 1; open "${URL}") >/dev/null 2>&1 & ;;
        Linux)  (sleep 1; xdg-open "${URL}") >/dev/null 2>&1 & ;;
    esac
    cd "${SERVE_DIR}" && exec python3 -m http.server "${PORT}"
else
    echo "[render-noyo] built ${OUT} (serving skipped)"
fi
