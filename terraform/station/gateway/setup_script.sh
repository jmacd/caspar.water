#!/bin/sh
set -e

systemctl daemon-reload

# Collector
chmod +x /home/jmacd/bin/collector
systemctl enable collector.service
systemctl enable --now collector-retry.timer
systemctl restart collector

echo Installation complete.
