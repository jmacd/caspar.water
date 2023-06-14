#!/bin/sh

trap cleanup 1 2 3 6

PID=""

export PRU_CGT=/usr/share/ti/cgt-pru
#or /usr/lib/ti/pru-software-support-package-v6.0

OUT_GPIOs="gpio117 gpio115"

# should have gpio115, testing w/ it as out
IN_GPIOs="" 

LEDs="beaglebone:green:usr0 beaglebone:green:usr1 beaglebone:green:usr2 beaglebone:green:usr3"

cleanup()
{
  kill -9 $PID
  exit 1
}

ps ax | grep user.out | awk '{print $1}' | xargs kill -9

echo "Stopping ..."
#echo stop > /sys/class/remoteproc/remoteproc1/state
#echo stop > /sys/class/remoteproc/remoteproc2/state

#make clean
#rm -rf gen
#mkdir gen

configPins() {
    echo "Configuring user LEDs"
    for led in ${LEDs}; do
	echo none > /sys/class/leds/${led}/trigger
    done
    echo "Configuring GPIO pins"
    for gpio in ${OUT_GPIOs}; do
	echo out > /sys/class/gpio/${gpio}/direction
    done
    for gpio in ${IN_GPIOs}; do
	echo in > /sys/class/gpio/${gpio}/direction
    done
}

#sleep 1

configPins
#sleep 1

make gen/ui1203.object PROC=pru TARGET=ui1203 CHIP=AM335x
make gen/ui1203.out PROC=pru TARGET=ui1203 CHIP=AM335x

cp gen/ui1203.out /lib/firmware/ui1203-fw

echo ui1203-fw > /sys/class/remoteproc/remoteproc1/firmware
#echo ui1203-fw > /sys/class/remoteproc/remoteproc2/firmware

echo "Starting ..."
echo start > /sys/class/remoteproc/remoteproc1/state
#echo start > /sys/class/remoteproc/remoteproc2/state

#echo "Building ui1203ctrl"

#export GO=/home/debian/go/bin/go
#${GO} build -o ui1203ctrl ./control

# Note control has to run with super privileges
