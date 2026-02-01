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
      source      = "casparwater_certs/casparwater_us.key"
      destination = "/etc/casparwater/casparwater_us.key"
  }

  provisioner "file" {
      source      = "casparwater_certs/casparwater_us.crt"
      destination = "/etc/casparwater/casparwater_us.crt"
  }

  provisioner "file" {
      source      = "nginx.casparwater.conf"
      destination = "/etc/nginx/sites-enabled/casparwater"
  }

  provisioner "file" {
      source      = "../../../site/"
      destination = "/var/www/html/"
  }

  provisioner "file" {
      source      = "influxdb.toml"
      destination = "/etc/influxdb/config.toml"
  }

  provisioner "file" {
      source      = "casparwater_certs/casparwater_us.pem"
      destination = "/etc/influxdb/casparwater_us.pem"
  }

  provisioner "file" {
      source      = "casparwater_certs/casparwater_us.key"
      destination = "/etc/influxdb/casparwater_us.key"
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
