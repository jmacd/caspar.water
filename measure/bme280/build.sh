#!/bin/sh

PLAY=beagleplay.local
GO=

scp -q -r -p * debian@${PLAY}:measure

ssh -q debian@${PLAY} '(cd measure && ~/go/bin/go run ./test)'
