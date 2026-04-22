variable "ipaddr" {
  description = "IP address of Linux host"
  default = "192.168.80.40"
}

resource "null_resource" "setup-script" {

  connection {
    type     = "ssh"
    user     = "root"
    private_key ="${file("/Users/jmacd/.ssh/id_rsa")}"
    host     = "${var.ipaddr}"
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

  provisioner "file" {
      source      = "config.yaml"
      destination = "/home/jmacd/etc/config.yaml"
  }

  provisioner "file" {
      source      = "collector.service"
      destination = "/etc/systemd/system/collector.service"
  }

  provisioner "file" {
      source      = "../../../collector/collector.linux"
      destination = "/home/jmacd/bin/collector"
  }

  # Duckpond scripts and configs
  provisioner "file" {
      source      = "duckpond/"
      destination = "/home/jmacd/duckpond"
  }

  # Site content (templates, content pages, images)
  provisioner "file" {
      source      = "../../../site/"
      destination = "/home/jmacd/duckpond/site"
  }

  # Systemd user units for duckpond timers
  provisioner "remote-exec" {
    inline = [
      "mkdir -p /home/jmacd/.config/systemd/user",
    ]
  }

  provisioner "file" {
      source      = "pond-water.service"
      destination = "/home/jmacd/.config/systemd/user/pond-water.service"
  }

  provisioner "file" {
      source      = "pond-water.timer"
      destination = "/home/jmacd/.config/systemd/user/pond-water.timer"
  }

  provisioner "file" {
      source      = "pond-noyo.service"
      destination = "/home/jmacd/.config/systemd/user/pond-noyo.service"
  }

  provisioner "file" {
      source      = "pond-noyo.timer"
      destination = "/home/jmacd/.config/systemd/user/pond-noyo.timer"
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
