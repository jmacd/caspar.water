variable "ipaddr" {
  description = "Hostname or IP of the septic station BBB"
  default     = "septicplaystation"
}

variable "ssh_user" {
  description = "SSH user on the BBB"
  default     = "root"
}

locals {
  ssh_key = pathexpand("~/.ssh/id_rsa")

  # Small config files: filename -> absolute destination path on the BBB.
  config_files = {
    "config.yaml"       = "/home/debian/etc/config.yaml"
    "collector.service" = "/etc/systemd/system/collector.service"
  }

  # Binaries: name -> source path relative to this module.  Destination
  # is always /home/debian/bin/${name}.
  binaries = {
    "collector" = "${path.module}/../../../collector/collector.bbb"
  }

  # Combined hash of every artifact: any change triggers stop + restart.
  artifact_hash = sha256(join("\n", concat(
    [for f in keys(local.config_files) : filesha256("${path.module}/${f}")],
    [for b in values(local.binaries) : filesha256(b)],
  )))
}

# Stop services before any update.  Re-runs whenever any artifact changes.
resource "null_resource" "stop_services" {
  triggers = {
    artifact_hash = local.artifact_hash
  }

  connection {
    type        = "ssh"
    user        = var.ssh_user
    host        = var.ipaddr
    private_key = file(local.ssh_key)
  }

  provisioner "remote-exec" {
    inline = [
      "systemctl stop collector || true",
    ]
  }
}

# Upload each small config file.  Re-runs only when that specific file changes.
resource "null_resource" "config_file" {
  for_each = local.config_files

  triggers = {
    src_hash = filesha256("${path.module}/${each.key}")
    dst      = each.value
  }

  depends_on = [null_resource.stop_services]

  connection {
    type        = "ssh"
    user        = var.ssh_user
    host        = var.ipaddr
    private_key = file(local.ssh_key)
  }

  provisioner "file" {
    source      = "${path.module}/${each.key}"
    destination = each.value
  }
}

# Upload binaries via rsync.  Re-runs only when that specific binary changes.
# rsync gives real progress, resumes through brief network blips via TCP
# backpressure, and skips unchanged blocks via its delta algorithm.
resource "null_resource" "binary" {
  for_each = local.binaries

  triggers = {
    src_hash = filesha256(each.value)
    name     = each.key
  }

  depends_on = [null_resource.stop_services]

  provisioner "local-exec" {
    command = <<-EOT
      set -euo pipefail
      rsync -tv -P --inplace --timeout=120 --no-owner --no-group \
        -e 'ssh -i ${local.ssh_key} -o ConnectTimeout=15 -o ServerAliveInterval=30 -o StrictHostKeyChecking=accept-new' \
        ${each.value} \
        ${var.ssh_user}@${var.ipaddr}:/home/debian/bin/${each.key}
    EOT
  }
}

# Final setup: permissions, daemon-reload, start.  Re-runs whenever any
# artifact changes (because we stopped the service).
resource "null_resource" "start_services" {
  triggers = {
    artifact_hash = local.artifact_hash
  }

  depends_on = [
    null_resource.config_file,
    null_resource.binary,
  ]

  connection {
    type        = "ssh"
    user        = var.ssh_user
    host        = var.ipaddr
    private_key = file(local.ssh_key)
  }

  provisioner "remote-exec" {
    inline = [
      "chown root:root /home/debian/bin/collector",
      "chmod +x /home/debian/bin/collector",
      "systemctl daemon-reload",
      "systemctl start collector",
    ]
  }
}
