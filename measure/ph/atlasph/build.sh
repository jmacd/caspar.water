#!/bin/sh

BONE=pumphouse.local

GOOS=linux GOARCH=arm go build ./cmd/atlasph

scp -q -r -p atlasph debian@${BONE}:atlasph

#ssh -q debian@${BONE} ./atlasph calibrate
