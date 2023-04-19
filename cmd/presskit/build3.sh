#!/bin/sh

LINUX=presskit.local

scp -r -q -p * debian@${LINUX}:monitor

ssh -q debian@${LINUX} '(cd monitor && ./build2.sh)'
