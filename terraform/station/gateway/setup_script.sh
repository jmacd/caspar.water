#!/bin/sh

systemctl daemon-reload

chmod +x /home/jmacd/bin/collector

systemctl start collector

echo Installation complete.
