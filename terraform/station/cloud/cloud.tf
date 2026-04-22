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

resource "linode_instance" "debian12-us-west" {
  region = "us-west"
  type = "g6-nanode-1"
}

resource "null_resource" "setup-script" {

  connection {
      type     = "ssh"
      user     = "root"
      private_key="${file("/Users/jmacd/.ssh/id_rsa")}"
      host     = linode_instance.debian12-us-west.ip_address
  }

  provisioner "file" {
      source      = "setup_script.sh"
      destination = "/tmp/setup_script.sh"
  }

  provisioner "file" {
      source      = "teardown_script.sh"
      destination = "/tmp/teardown_script.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/teardown_script.sh",
      "/tmp/teardown_script.sh",
    ]
  }

  # Systemd user units for duckpond timer
  provisioner "remote-exec" {
    inline = [
      "mkdir -p /home/jmacd/.config/systemd/user",
    ]
  }

  provisioner "file" {
      source      = "pond-site.service"
      destination = "/home/jmacd/.config/systemd/user/pond-site.service"
  }

  provisioner "file" {
      source      = "pond-site.timer"
      destination = "/home/jmacd/.config/systemd/user/pond-site.timer"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/setup_script.sh",
      "/tmp/setup_script.sh",
    ]
  }

  # Push Caddyfile AFTER setup (which installs caddy and its default config)
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
