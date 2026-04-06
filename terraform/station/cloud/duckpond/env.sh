#!/bin/bash
# Shared environment for cloud duckpond instance.
# Sourced by other scripts, not run directly.
#
# R2 credentials must be in ~/.bashrc.private:
#   export R2_ENDPOINT=https://...
#   export R2_KEY=...
#   export R2_SECRET=...

export IMAGE=ghcr.io/jmacd/duckpond/duckpond:latest-amd64
export RUST_LOG=info

if [ -f ~/.bashrc.private ]; then
    source ~/.bashrc.private
fi
