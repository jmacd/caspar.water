#!/bin/sh
#/etc/init.d/edgemon

### BEGIN INIT INFO
# Provides:          edgemon 
# Required-Start:    $network nxtio-primer soft-pac
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: Edgemon server daemon
# Description:       Prometheus monitoring
### END INIT INFO

CONFIG=/usr/share/nxtio/services/edgemon/pm2.conf.json
NAME=edgemon

. /etc/init.d/pm2-sysv-base.sh
