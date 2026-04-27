#!/usr/bin/env bash
# deploy-watershop.sh -- one-shot watershop selfmon deploy.
#
# Steps:
#   1. tools/build-on-watershop.sh (push branch, remote cargo-deb on
#      watershop, dpkg -i the freshly built /usr/bin/pond).
#   2. terraform apply in terraform/station/watershop (re-pushes
#      config + env, re-runs install-duckpond.sh which always picks
#      the newest .deb in target/debian/, re-applies the
#      watershop-selfmon.yaml).
#
# This is the local-experimental selfmon deploy path.  Production
# water/noyo/septic/site ponds are NOT touched -- they run from
# GH-Actions-built podman images and are gated by separate manual
# promotion.
#
# Usage:
#   tools/deploy-watershop.sh                  # build + tf apply
#   tools/deploy-watershop.sh --no-terraform   # stop after the build
#   tools/deploy-watershop.sh --auto-approve   # pass -auto-approve
#                                              # through to terraform
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "$0")/.." && pwd)
TF_DIR="${REPO_ROOT}/terraform/station/watershop"

RUN_TF=1
TF_AUTO_APPROVE=""
for arg in "$@"; do
    case "$arg" in
        --no-terraform) RUN_TF=0 ;;
        --auto-approve) TF_AUTO_APPROVE="-auto-approve" ;;
        -h|--help)
            sed -n '2,/^$/p' "$0" | sed 's/^# \?//'
            exit 0
            ;;
        *) echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

echo "==> building duckpond on watershop"
"${REPO_ROOT}/tools/build-on-watershop.sh"

if [ "${RUN_TF}" = "0" ]; then
    echo "==> --no-terraform: skipping terraform apply"
    exit 0
fi

echo "==> terraform apply"
cd "${TF_DIR}"
terraform apply ${TF_AUTO_APPROVE}
