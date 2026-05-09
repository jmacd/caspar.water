#!/usr/bin/env bash
# build-on-watershop.sh -- push the current duckpond branch and build a
# Debian package for it natively on watershop (arm64).  The resulting
# .deb lands at ~/src/duckpond/target/debian/duckpond_<ver>_arm64.deb on
# watershop.  By default also `dpkg -i`'s it.  Use --no-install to
# stop after the build.
#
# Why native: watershop is the same arch as our prod arm64 fleet, so
# building there avoids cross-compile pain and the ~3-hour PR/CI/image
# cycle.  No GH Actions, no podman, no self-hosted runner involved.
#
# This binary is for the local-experimental `watershop-selfmon` pond
# only.  Production data ponds (water/noyo/septic/site) run from
# GH-Actions-built podman images.
#
# Usage:
#   tools/build-on-watershop.sh                      # build + install
#   tools/build-on-watershop.sh --no-install         # build only
#   DUCKPOND_DIR=/path/to/duckpond tools/...         # override worktree
#   WATERSHOP_HOST=other.host    tools/...           # override target
#
set -euo pipefail

DUCKPOND_DIR=${DUCKPOND_DIR:-$(cd "$(dirname "$0")/../duckpond" && pwd)}
WATERSHOP_HOST=${WATERSHOP_HOST:-watershop.casparwater.us}
INSTALL=1
for arg in "$@"; do
    case "$arg" in
        --no-install) INSTALL=0 ;;
        -h|--help)
            sed -n '2,/^$/p' "$0" | sed 's/^# \?//'
            exit 0
            ;;
        *) echo "unknown arg: $arg" >&2; exit 2 ;;
    esac
done

cd "${DUCKPOND_DIR}"

if [ ! -e .git ]; then
    echo "ERROR: ${DUCKPOND_DIR} is not a git worktree" >&2
    exit 1
fi

BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "${BRANCH}" = "HEAD" ]; then
    echo "ERROR: detached HEAD; check out a branch first" >&2
    exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
    echo "WARNING: ${DUCKPOND_DIR} has uncommitted changes; only" >&2
    echo "         committed history on '${BRANCH}' will be built." >&2
    echo "         Press Ctrl-C within 3s to abort." >&2
    sleep 3
fi

LOCAL_SHA=$(git rev-parse HEAD)
echo "==> pushing ${BRANCH} (${LOCAL_SHA:0:12}) to origin"
git push origin "${BRANCH}"

# Remote build.  We pin to LOCAL_SHA so the build matches what we just
# pushed even if someone else races a push to the same branch.
# `dpkg -i` over an identical version is a cheap no-op overwrite, so
# we don't bother to vary the cargo-deb revision per build.
REMOTE_SCRIPT=$(cat <<EOF
set -euo pipefail
cd ~/src/duckpond
git fetch origin ${BRANCH}
git checkout ${BRANCH}
git reset --hard ${LOCAL_SHA}
. \$HOME/.cargo/env
make vendor
cargo deb -p cmd
ls -la target/debian/duckpond_*_arm64.deb | tail -1
EOF
)

echo "==> building on ${WATERSHOP_HOST}"
ssh "${WATERSHOP_HOST}" bash -s <<<"${REMOTE_SCRIPT}"

if [ "${INSTALL}" = "1" ]; then
    echo "==> installing freshly built deb on ${WATERSHOP_HOST}"
    ssh "${WATERSHOP_HOST}" \
        "DEB=\$(ls -t ~/src/duckpond/target/debian/duckpond_*_arm64.deb | head -1) && sudo dpkg -i \"\${DEB}\" && /usr/bin/pond --version"
else
    echo "==> --no-install: deb left in ~/src/duckpond/target/debian/ on ${WATERSHOP_HOST}"
fi
