#!/bin/sh

LINUX=192.168.0.40

scp -q -r -p * jmacd@${LINUX}:ui1203

ssh -q jmacd@${LINUX} '(cd ui1203 && ./build2.sh)'
