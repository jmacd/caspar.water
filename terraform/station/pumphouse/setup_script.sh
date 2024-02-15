#!/bin/sh

systemctl daemon-reload

systemctl start collector

echo Installation complete.
