#!/bin/sh

systemctl daemon-reload

chmod +x /home/debian/bin/collector

systemctl start collector

echo Installation complete.
