#!/bin/sh

PLAY=wellkit.local

scp -q -r -p * debian@${PLAY}:lcd

ssh -q debian@${PLAY} '(cd lcd && $HOME/go/bin/go run ./test)'
