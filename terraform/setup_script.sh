#!/bin/sh

apt-get update
apt-get install -y software-properties-common

apt-get install -y openssl

# From https://developer.hashicorp.com/nomad/docs/install
if grep -q /usr/share/keyrings/hashicorp-archive-keyring.gpg /etc/apt/sources.list.d/hashicorp.list; then
    echo Hashicorp APT source already configured
else
    wget -O- https://apt.releases.hashicorp.com/gpg | gpg --dearmor -o /usr/share/keyrings/hashicorp-archive-keyring.gpg

    echo "deb [signed-by=/usr/share/keyrings/hashicorp-archive-keyring.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | tee /etc/apt/sources.list.d/hashicorp.list
fi

apt-get install -y nomad

if [ -d /opt/cni/bin ]; then
    echo Nomad CNI plugins already configured
else
    curl -L -o /tmp/cni-plugins.tgz "https://github.com/containernetworking/plugins/releases/download/v1.0.0/cni-plugins-linux-$( [ $(uname -m) = aarch64 ] && echo arm64 || echo amd64)"-v1.0.0.tgz
  mkdir -p /opt/cni/bin
  tar -C /opt/cni/bin -xzf /tmp/cni-plugins.tgz
  rm /tmp/cni-plugins.tgz
fi

echo Starting nomad
mkdir -p /etc/nomadclient.d

mkdir -p /opt/nomad
mkdir -p /opt/nomadclient

usermod -d /etc/nomad.d nomad
usermod -aG docker nomad

usermod -d /etc/nomad.d nomad

adduser nomadclient root
usermod -d /etc/nomadclient.d nomadclient

systemctl enable nomad
systemctl start nomad

# https://www.linode.com/docs/guides/installing-and-using-docker-on-ubuntu-and-debian/
echo Setup docker
apt install -y apt-transport-https ca-certificates curl gnupg lsb-release

if [ -f /usr/share/keyrings/docker-archive-keyring.gpg ]; then
    echo Docker GPG key installed
else 
    curl -fsSL https://download.docker.com/linux/debian/gpg | gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg
fi

echo "deb [arch=amd64 signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/debian $(lsb_release -cs) stable" | tee /etc/apt/sources.list.d/docker.list > /dev/null

apt update -y
apt install -y docker-ce docker-ce-cli containerd.io

systemctl enable docker
systemctl enable containerd
systemctl start docker
systemctl start containerd

mkdir -p /etc/caspar.d/influxdb/certs
mkdir -p /opt/influxdb

# TODO: install nomad-pack
# Currently no debian package, have installed Go 1.20.x and built in /root/nomad-pack
# See git@github.com:jmacd/nomad-pack-community-registry.git, I've modified the pack
# slightly to expose a static port 8086.
(cd /root/nomad-pack &&
     ./bin/nomad-pack run influxdb \
		      --registry=jmacd-community \
		      --ref=caspar_cloud_influx \
		      -f /etc/caspar.d/influxdb/vars.hcl)
