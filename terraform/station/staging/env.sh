#!/bin/bash
# Shared environment for staging on watershop.casparwater.us (ARM).
# Sourced by other scripts, not run directly.

export IMAGE=ghcr.io/jmacd/duckpond/duckpond:latest-arm64
export MINIO_ENDPOINT=http://localhost:9000
export MINIO_ACCESS_KEY=caspar
export MINIO_SECRET_KEY=watertown
export RUST_LOG=info

# HydroVu credentials (source from ~/.bashrc.private on watershop)
if [ -f ~/.bashrc.private ]; then
    source ~/.bashrc.private
fi
