#!/usr/bin/env bash
# attach-remotes.sh -- (re)attach S3 backup/import remotes for an instance.
#
# Post-D6 watertown removed the `factory: remote` config node and replaced
# it with two top-level CLI verbs:
#
#   pond backup add <name> <url> [s3 opts]          (push mirror backup)
#   pond remote add <name> <url> <mount> [s3 opts]  (pull cross-pond import)
#
# This script (re)creates those attachments after `pond init` / `pond
# apply`.  It is idempotent: every attach uses --overwrite so it can run
# on every deploy.  The secret is passed as a single-quoted
# `${env:S3_SECRET_KEY}` reference (never a literal) -- the attachment
# YAML is replicated, so a literal secret would leak to all replicas; the
# pond binary resolves the reference from its own environment at use time.
#
# Usage: attach-remotes.sh <instance>
set -e

INSTANCE=$1
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
BASE_DIR=$(cd "${SCRIPTS}/../.." && pwd)
ENV_FILE="${BASE_DIR}/env/${INSTANCE}.env"

if [ ! -f "${ENV_FILE}" ]; then
    echo "ERROR: No env file for instance '${INSTANCE}' at ${ENV_FILE}"
    exit 1
fi

# Native (selfmon) instances run /usr/bin/pond directly with the env
# exported into this shell so the binary can resolve ${env:S3_SECRET_KEY}
# at attach time; containerized instances go through pond.sh, which
# injects the env via podman --env-file.  Select the runner up front.
if [[ "${INSTANCE}" == *-selfmon ]]; then
    set -a
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    set +a
    pond() { /usr/bin/pond "$@"; }
else
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
    pond() { "${SCRIPTS}/pond.sh" "${INSTANCE}" "$@"; }
fi

# Common S3 attach options.  --allow-http only when the endpoint is plain
# HTTP (MinIO on watershop).  secret-access-key MUST be an env reference;
# the single quotes keep the literal `${env:...}` text intact through the
# bash array so the pond binary (not the shell) resolves it.
s3_opts=(
    --region "${S3_REGION}"
    --endpoint "${S3_ENDPOINT}"
    --access-key-id "${S3_ACCESS_KEY}"
    --secret-access-key '${env:S3_SECRET_KEY}'
)
if [ "${S3_ALLOW_HTTP}" = "true" ]; then
    s3_opts+=(--allow-http)
fi

TYPE="${INSTANCE%-staging}"
TYPE="${TYPE%-prod}"

case "${TYPE}" in
    water|septic|noyo)
        # Producer: push-mirror this pond to its own bucket.
        echo "[attach] ${INSTANCE}: backup add origin ${S3_URL}"
        pond backup add origin "${S3_URL}" "${s3_opts[@]}" --overwrite
        # Seed the remote with the pond_init bundle so consumers (site)
        # can pull immediately, before the first data-collection tick.
        pond push origin
        ;;
    watershop-selfmon)
        # selfmon is local-experimental and deliberately has NO backup
        # remote.  run-selfmon.sh maintains it with `--prune
        # --allow-no-remote`, which aggressively vacuums local history
        # every tick; that is fundamentally incompatible with a push
        # backup, because the post-commit auto-push then tries to read
        # files the prune already deleted and fails, and the concurrent
        # push holds the control write.lock so the next maintain cannot
        # compact -- which lets the delta log grow until resolving
        # /logs/journal exceeds the DataFusion memory pool and the pond
        # wedges.  Attaching no remote keeps maintenance lock-free.
        echo "[attach] ${INSTANCE}: local-experimental, no backup remote"
        ;;
    site)
        # Consumer: cross-pond import each producer's bucket read-through
        # at /sources/<name>, matching the sitegen export/subsite paths.
        echo "[attach] ${INSTANCE}: remote add water/noyo/septic"
        pond remote add water  "${WATER_S3_URL}"  /sources/water  "${s3_opts[@]}" --overwrite
        pond remote add noyo   "${NOYO_S3_URL}"   /sources/noyo   "${s3_opts[@]}" --overwrite
        pond remote add septic "${SEPTIC_S3_URL}" /sources/septic "${s3_opts[@]}" --overwrite
        ;;
    *)
        echo "ERROR: Unknown pond type '${TYPE}' for instance '${INSTANCE}'"
        exit 1
        ;;
esac
