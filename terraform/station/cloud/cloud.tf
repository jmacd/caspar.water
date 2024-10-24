terraform {
  required_providers {
    linode = {
      source  = "linode/linode"
    }
  }
}

variable "li_token" {
  description = "The token for linode"
  type        = string
  sensitive   = true
}

provider "linode" {
  token = var.li_token
}

resource "linode_instance" "debian-us-west" {
  region = "us-west"
  type = "g6-nanode-1"
}

resource "null_resource" "setup-script" {

  connection {
      type     = "ssh"
      user     = "root"
      private_key="${file("/Users/josh.macdonald/.ssh/id_rsa")}"
      host     = linode_instance.debian-us-west.ip_address
  }

  provisioner "file" {
      source      = "setup_script.sh"
      destination = "/tmp/setup_script.sh"
  }

  provisioner "file" {
      source      = "bridge.d"
      destination = "/etc/sysctl.d/bridge.d"
  }

  provisioner "file" {
      source      = "nomadserver.service"
      destination = "/etc/systemd/system/nomadserver.service"
  }

  provisioner "file" {
      source      = "nomadclient.service"
      destination = "/etc/systemd/system/nomadclient.service"
  }

  provisioner "file" {
      source      = "nomadserver.hcl"
      destination = "/etc/nomad.d/nomadserver.hcl"
  }

  provisioner "file" {
      source      = "nomadclient.hcl"
      destination = "/etc/nomadclient.d/nomadclient.hcl"
  }

  provisioner "file" {
      source      = "influxdb-vars.hcl"
      destination = "/etc/caspar.d/influxdb/vars.hcl"
  }

  provisioner "file" {
      source      = "influxdb.yaml"
      destination = "/etc/caspar.d/influxdb/config.yaml"
  }

  provisioner "file" {
      source      = "nginx.conf"
      destination = "/etc/nginx/nginx.conf"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/setup_script.sh",
      "/tmp/setup_script.sh",
    ]
  }

  triggers = {
    always_run = timestamp()
  }
}
