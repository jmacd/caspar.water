variable "host" {
  description = "SSH host for staging"
  default     = "watershop.casparwater.us"
}

variable "user" {
  description = "SSH user"
  default     = "jmacd"
}

resource "null_resource" "staging" {

  connection {
    type        = "ssh"
    user        = var.user
    private_key = file("/Users/jmacd/.ssh/id_rsa")
    host        = var.host
  }

  # Teardown existing state
  provisioner "remote-exec" {
    inline = [
      "rm -rf /home/${var.user}/staging",
      "mkdir -p /home/${var.user}/staging",
    ]
  }

  # Push staging scripts and configs
  provisioner "file" {
    source      = "env.sh"
    destination = "/home/${var.user}/staging/env.sh"
  }

  provisioner "file" {
    source      = "setup-all.sh"
    destination = "/home/${var.user}/staging/setup-all.sh"
  }

  provisioner "file" {
    source      = "run-all.sh"
    destination = "/home/${var.user}/staging/run-all.sh"
  }

  provisioner "file" {
    source      = "teardown-all.sh"
    destination = "/home/${var.user}/staging/teardown-all.sh"
  }

  # Water pond
  provisioner "file" {
    source      = "water/"
    destination = "/home/${var.user}/staging/water"
  }

  # Noyo pond
  provisioner "file" {
    source      = "noyo/"
    destination = "/home/${var.user}/staging/noyo"
  }

  # Site pond
  provisioner "file" {
    source      = "site/"
    destination = "/home/${var.user}/staging/site"
  }

  # Site content from repo root
  provisioner "file" {
    source      = "../../../site/"
    destination = "/home/${var.user}/staging/site-content"
  }

  # Make scripts executable and run setup
  provisioner "remote-exec" {
    inline = [
      "chmod +x /home/${var.user}/staging/*.sh",
      "chmod +x /home/${var.user}/staging/water/*.sh",
      "chmod +x /home/${var.user}/staging/noyo/*.sh",
      "chmod +x /home/${var.user}/staging/site/*.sh",
      "/home/${var.user}/staging/teardown-all.sh || true",
      "/home/${var.user}/staging/setup-all.sh",
    ]
  }

  triggers = {
    always_run = timestamp()
  }
}
