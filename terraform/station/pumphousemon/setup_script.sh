#!/bin/sh

systemctl daemon-reload

chmod +x /home/debian/bin/collector

/sbin/setcap cap_net_raw=+ep /home/debian/bin/edgemon

systemctl start collector

echo Installation complete.
