#!/bin/bash

# Modbus Test Runner
# Usage: ./build.sh [local|remote] [args...]
#
# Examples:
#   ./build.sh local -url tcp://192.168.1.100:502 -register 1 -type uint16 -v
#   ./build.sh remote -url tcp://192.168.1.100:502 -register 1 -v
#   ./build.sh local -url rtu:///dev/ttyUSB0 -baud 9600 -register 1 -v

set -e

MODE=${1:-local}
shift

BINARY=modbus-test

if [ "$MODE" = "remote" ]; then
    PLAY=${PLAY:-septicplaystation.local}
    REMOTE_USER=${REMOTE_USER:-debian}
    REMOTE_DIR=${REMOTE_DIR:-modbus-test}
    
    echo "Cross-compiling for ARM Linux..."
    GOOS=linux GOARCH=arm GOARM=7 go build -o ${BINARY} .
    
    echo "Deploying to ${REMOTE_USER}@${PLAY}..."
    ssh -q ${REMOTE_USER}@${PLAY} "mkdir -p ${REMOTE_DIR}"
    scp -q ${BINARY} ${REMOTE_USER}@${PLAY}:${REMOTE_DIR}/
    
    echo "Running on remote..."
    ssh -t ${REMOTE_USER}@${PLAY} "cd ${REMOTE_DIR} && ./${BINARY} $@"
    
    rm -f ${BINARY}
else
    echo "Running locally..."
    go run . "$@"
fi
