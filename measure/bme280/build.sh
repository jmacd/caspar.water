#!/bin/sh

PLAY=wellkit.local

scp -q -r -p * debian@${PLAY}:measure

ssh -q debian@${PLAY} '(cd measure && $HOME/go/bin/go run ./test)'
