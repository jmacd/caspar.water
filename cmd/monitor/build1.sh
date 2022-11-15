#!/bin/sh

LINUX=192.168.0.40

scp -q -p * jmacd@${LINUX}:monitor

ssh -q jmacd@${LINUX} '(cd monitor && ./build2.sh)'
