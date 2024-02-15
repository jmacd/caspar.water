#!/bin/sh

LEDs="beaglebone:green:usr0 beaglebone:green:usr1 beaglebone:green:usr2 beaglebone:green:usr3"

BONE=${1:-debian@beaglebone.local}

ssh ${BONE} "for led in ${LEDs}; do echo none > /sys/class/leds/\${led}/trigger; done"
