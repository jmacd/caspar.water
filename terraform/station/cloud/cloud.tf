terraform {
  required_providers {
    linode = {
      source = "linode/linode"
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

locals {
  home     = "/home/jmacd"
  base_dir = "${local.home}/duckpond"
}

resource "linode_instance" "debian12-us-west" {
  region = "us-west"
  type   = "g6-nanode-1"
}

resource "null_resource" "cloud" {
  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(pathexpand("~/.ssh/id_rsa"))
    host        = tolist(linode_instance.debian12-us-west.ipv4)[0]
  }

  # Push setup/teardown scripts
  provisioner "file" {
    source      = "setup_script.sh"
    destination = "/tmp/setup_script.sh"
  }

  provisioner "file" {
    source      = "teardown_script.sh"
    destination = "/tmp/teardown_script.sh"
  }

  # Teardown previous state (stops & removes any historical pond@*
  # timers and the pond-site-prod volume that lived here before the
  # cloud host was reduced to caddy + rsync target)
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/teardown_script.sh",
      "/tmp/teardown_script.sh",
    ]
  }

  # Create directory structure for the rsync'd site builds
  provisioner "remote-exec" {
    inline = [
      "mkdir -p ${local.base_dir}/www",
      "mkdir -p ${local.home}/.ssh",
      "chown -R jmacd:jmacd ${local.base_dir}",
    ]
  }

  # Install deploy public key so watershop can rsync as jmacd
  provisioner "file" {
    source      = "${path.module}/../watershop/deploy_key.pub"
    destination = "/tmp/cloud_deploy.pub"
  }

  provisioner "remote-exec" {
    inline = [
      "cat /tmp/cloud_deploy.pub >> ${local.home}/.ssh/authorized_keys",
      "sort -u -o ${local.home}/.ssh/authorized_keys ${local.home}/.ssh/authorized_keys",
      "chmod 600 ${local.home}/.ssh/authorized_keys",
      "chown jmacd:jmacd ${local.home}/.ssh/authorized_keys",
      "rm /tmp/cloud_deploy.pub",
    ]
  }

  # Run setup (installs caddy + rsync)
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/setup_script.sh",
      "/tmp/setup_script.sh",
    ]
  }

  # Push Caddyfile AFTER setup (caddy install writes default config)
  provisioner "file" {
    source      = "Caddyfile"
    destination = "/etc/caddy/Caddyfile"
  }

  provisioner "remote-exec" {
    inline = [
      "systemctl restart caddy",
    ]
  }

  triggers = {
    always_run = timestamp()
  }
}
