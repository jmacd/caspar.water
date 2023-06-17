#!/bin/sh

BONE=presskit.local

scp -q -r -p * debian@${BONE}:ui1203

ssh -q debian@${BONE} '(cd ui1203 && ./build3.sh)'
