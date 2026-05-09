#!/bin/sh

systemctl daemon-reload

# Collector
chmod +x /home/jmacd/bin/collector
systemctl restart collector

echo Installation complete.
