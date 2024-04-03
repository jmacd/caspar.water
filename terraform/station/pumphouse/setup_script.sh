#!/bin/sh

systemctl daemon-reload

# Note! Supructl and firmware are installed from supruglue repo, not included
# in this terraform.

chmod +x /home/debian/bin/collector
chmod +x /home/debian/bin/edgemon
chmod +x /home/debian/bin/supruglue

/sbin/setcap cap_net_raw=+ep /home/debian/bin/edgemon

systemctl start collector
systemctl start edgemon
systemctl start supruglue

echo Installation complete.
