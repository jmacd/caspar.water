#!/usr/bin/env bash
# deploy-watershop.sh -- one-shot watershop selfmon deploy.
#
# Runs terraform apply in terraform/station/watershop: re-pushes config +
# env, re-applies watershop-selfmon.yaml, and brings /usr/bin/pond up to
# the .deb CI published (via update-selfmon.sh, the same script the hourly
# timer runs).
#
# Binaries come from CI.  This script used to build one natively first,
# back when that was the only source; it no longer does, because the
# apply itself now pulls the CI artifact.  To test UNMERGED code on the
# box, run tools/build-on-watershop.sh explicitly -- but note the next
# apply or hourly tick will replace it with the CI build, by design.
#
# This is the local-experimental selfmon deploy path.  Production
# water/noyo/septic/site ponds are NOT touched -- they run from
# GH-Actions-built podman images and are gated by separate manual
# promotion.
#
# Usage:
#   tools/deploy-watershop.sh                          # terraform apply
#   tools/deploy-watershop.sh --auto-approve           # pass -auto-approve
#                                                      # through to terraform
#   tools/deploy-watershop.sh --reset=NAME[,NAME...]   # one-shot wipe of
#                                                      # the named instance(s)
#                                                      # (volume + S3 bucket
#                                                      # for containerized,
#                                                      # also the source jsonl
#                                                      # + rendered HTML for
#                                                      # selfmon).  Passed
#                                                      # via -var=, never
#                                                      # written to tfvars,
#                                                      # so it can NOT
#                                                      # accidentally persist.
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "$0")/.." && pwd)
TF_DIR="${REPO_ROOT}/terraform/station/watershop"

TF_AUTO_APPROVE=""
RESET_LIST=""
for arg in "$@"; do
    case "$arg" in
        --auto-approve) TF_AUTO_APPROVE="-auto-approve" ;;
        --reset=*)      RESET_LIST="${arg#--reset=}" ;;
        -h|--help)
            sed -n '2,/^$/p' "$0" | sed 's/^# \?//'
            exit 0
            ;;
        *) echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

# Build the optional `-var='reset_instances=["a","b"]'` arg from the
# comma-separated --reset= list.  Done as a bash array so the quotes
# survive correctly through `terraform apply` argv parsing.
TF_RESET_ARG=()
if [ -n "${RESET_LIST}" ]; then
    # Build a JSON-style list literal that terraform's HCL parser accepts.
    RESET_JSON=""
    IFS=','
    for name in ${RESET_LIST}; do
        if [ -z "${RESET_JSON}" ]; then
            RESET_JSON="\"${name}\""
        else
            RESET_JSON="${RESET_JSON},\"${name}\""
        fi
    done
    unset IFS
    TF_RESET_ARG=("-var=reset_instances=[${RESET_JSON}]")
    echo "==> reset requested: [${RESET_JSON}]"
fi

echo "==> terraform apply"
cd "${TF_DIR}"
# `${A[@]+"${A[@]}"}` rather than plain `"${A[@]}"`: bash 3.2 (what macOS
# ships) treats an empty array as unbound under `set -u`, so the no-reset
# path -- the common one -- would abort before ever reaching terraform.
terraform apply ${TF_AUTO_APPROVE} ${TF_RESET_ARG[@]+"${TF_RESET_ARG[@]}"}
