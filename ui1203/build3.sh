#!/bin/sh

trap cleanup 1 2 3 6

PID=""

GPIOs="gpio114 gpio15 gpio3 gpio36 gpio46 gpio60 gpio68 gpio74 gpio80 gpio10 gpio115 gpio2 gpio30 gpio37 gpio47 gpio61 gpio69 gpio75 gpio81 gpio11 gpio116 gpio20 gpio31 gpio38 gpio48 gpio62 gpio7 gpio76 gpio86 gpio110 gpio117 gpio22 gpio32 gpio39 gpio49 gpio63 gpio70 gpio77 gpio87 gpio111 gpio12 gpio23 gpio33 gpio4 gpio5 gpio65 gpio71 gpio78 gpio88 gpio112 gpio13 gpio26 gpio34 gpio44 gpio50 gpio66 gpio72 gpio79 gpio89 gpio113 gpio14 gpio27 gpio35 gpio45 gpio51 gpio67 gpio73 gpio8 gpio9 "

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
#rm -rf output
#mkdir output

make output/pru.o PROC=pru TARGET=pru CHIP=AM335x
make output/pru.out PROC=pru TARGET=pru CHIP=AM335x

cp output/pru.out /lib/firmware/ui1203-fw

echo ui1203-fw > /sys/class/remoteproc/remoteproc1/firmware
#echo ui1203-fw > /sys/class/remoteproc/remoteproc2/firmware

configPins() {
    echo "Configuring user LEDs"
    for led in ${LEDs}; do
	echo none > /sys/class/leds/${led}/trigger
    done
    echo "Configuring GPIO pins"
    for gpio in ${GPIOs}; do
	echo out > /sys/class/gpio/${gpio}/direction
    done
}

#sleep 1

configPins
#sleep 1

echo "Starting ..."
#echo start > /sys/class/remoteproc/remoteproc1/state
#echo start > /sys/class/remoteproc/remoteproc2/state

#echo "Building ui1203ctrl"

#export GO=/home/debian/go/bin/go
#${GO} build -o ui1203ctrl ./control

# Note control has to run with super privileges
