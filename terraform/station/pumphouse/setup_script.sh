#!/bin/sh

systemctl daemon-reload

chmod +x /home/debian/bin/collector
chmod +x /home/debian/bin/edgemon

/sbin/setcap cap_net_raw=+ep /home/debian/bin/edgemon

systemctl start collector
systemctl start edgemon

echo Installation complete.
