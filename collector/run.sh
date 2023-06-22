#!/bin/sh

make
#scp collector.linux jmacd@linux.local:/home/jmacd/bin/collector.new
#ssh jmacd@linux.local mv /home/jmacd/bin/collector.new /home/jmacd/bin/collector
#scp config.yaml jmacd@linux.local:/home/jmacd/src/caspar.water/collector/config.yaml

scp collector.bbb debian@wellkit.local:
scp config-debug.yaml debian@wellkit.local:
ssh debian@wellkit.local killall collector.bbb
ssh debian@wellkit.local ./collector.bbb --config  config-debug.yaml
