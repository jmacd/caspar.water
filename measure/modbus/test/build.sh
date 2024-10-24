#!/bin/sh

PLAY=septicstation.local

scp -q -r -p * debian@${PLAY}:serial

ssh -q debian@${PLAY} '(cd serial && $HOME/go/bin/go run ./test)'
