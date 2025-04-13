#!/bin/sh

apt-get update -y
apt-get upgrade -y

apt-get install -y software-properties-common

apt-get install -y openssl
apt-get install -y gpg

# Note some manual setup for deb source
apt-get install influxdb2

systemctl stop nginx
systemctl enable nginx
systemctl start nginx

systemctl stop influxdb
systemctl enable influxdb
systemctl start influxdb
