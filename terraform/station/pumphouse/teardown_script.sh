#!/bin/sh

systemctl stop supruglue
systemctl stop edgemon
systemctl stop collector

echo Teardown complete.
