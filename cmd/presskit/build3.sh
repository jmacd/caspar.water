#!/bin/sh

LINUX=presskit.local

scp -q -p * debian@${LINUX}:monitor

ssh -q debian@${LINUX} '(cd monitor && ./build2.sh)'
