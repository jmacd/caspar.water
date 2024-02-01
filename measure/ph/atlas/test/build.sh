#!/bin/sh

BONE=beaglebone.local

GOOS=linux GOARCH=arm go build .

scp -q -r -p test debian@${BONE}:atlas

ssh -q debian@${BONE} ./atlas
