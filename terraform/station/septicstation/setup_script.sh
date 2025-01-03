#!/bin/sh

# When to create the root ssh authorized_keys file? when to create the bin and etc dirs?

systemctl daemon-reload

chmod +x /home/debian/bin/collector

systemctl start collector

echo Installation complete.
