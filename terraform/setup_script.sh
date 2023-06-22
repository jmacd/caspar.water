#!/bin/sh

apt-get update
apt-get install -y software-properties-common
apt-get install -y influxdb
apt-get install -y influxdb-client
apt-get install -y wget gpg coreutils

# From https://developer.hashicorp.com/nomad/docs/install
if grep -q /usr/share/keyrings/hashicorp-archive-keyring.gpg /etc/apt/sources.list.d/hashicorp.list; then
    echo Hashicorp APT source already configured
else
    wget -O- https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg

    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/hashicorp.list
fi

sudo apt-get install -y nomad

if [ -d /opt/cni/bin ]; then
    echo Nomad CNI plugins already configured
else
    curl -L -o /tmp/cni-plugins.tgz "https://github.com/containernetworking/plugins/releases/download/v1.0.0/cni-plugins-linux-$( [ $(uname -m) = aarch64 ] && echo arm64 || echo amd64)"-v1.0.0.tgz
  mkdir -p /opt/cni/bin
  tar -C /opt/cni/bin -xzf /tmp/cni-plugins.tgz
  rm /tmp/cni-plugins.tgz
fi
