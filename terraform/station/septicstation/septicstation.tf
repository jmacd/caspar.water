variable "ipaddr" {
  description = "IP address of BBB host"
  default = "192.168.70.70"
}

resource "null_resource" "setup-script" {

  connection {
    type     = "ssh"
    user     = "root"
    private_key ="${file("/Users/josh.macdonald/.ssh/id_rsa")}"
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
      destination = "/home/debian/etc/config.yaml"
  }

  provisioner "file" {
      source      = "collector.service"
      destination = "/etc/systemd/system/collector.service"
  }

  provisioner "file" {
      source      = "../../../collector/collector.bbb"
      destination = "/home/debian/bin/collector"
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
