#!/bin/sh

BONE=192.168.6.2

scp -q -r -p * debian@${BONE}:ui1203

ssh -q debian@${BONE} '(cd ui1203 && ./build3.sh)'
