#!/bin/sh

make
scp collector.linux jmacd@linux.local:/home/jmacd/bin/collector.new
ssh jmacd@linux.local mv /home/jmacd/bin/collector.new /home/jmacd/bin/collector
scp config.yaml jmacd@linux.local:/home/jmacd/src/caspar.water/collector/config.yaml
