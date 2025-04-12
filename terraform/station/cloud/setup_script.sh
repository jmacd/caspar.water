#!/bin/sh

apt-get update -y
apt-get upgrade -y

apt-get install -y software-properties-common

apt-get install -y openssl
apt-get install -y gpg

systemctl enable nginx
systemctl start nginx
