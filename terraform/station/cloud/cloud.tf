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
  base_dir = "${local.home}/watertown"
  ssh_key  = pathexpand("~/.ssh/id_rsa")

  # Source files in this module, paired with the absolute destination on the host.
  caddyfile_src       = "${path.module}/Caddyfile"
  caddyfile_dst       = "/etc/caddy/Caddyfile"
  influxdb_config_src = "${path.module}/influxdb.toml"
  influxdb_config_dst = "/etc/influxdb/config.toml"
  deploy_key_src      = "${path.module}/../watershop/deploy_key.pub"
  setup_src           = "${path.module}/setup_script.sh"
  teardown_src        = "${path.module}/teardown_script.sh"

  host = tolist(linode_instance.debian12-us-west.ipv4)[0]
}

resource "linode_instance" "debian12-us-west" {
  region = "us-west"
  type   = "g6-nanode-1"
}

# DNS for the apex `casparwater.us` is managed elsewhere; we only own the
# `influx` subdomain used by InfluxDB clients (Phase 1 of the TLS termination
# migration documented in Caddyfile).
data "linode_domain" "casparwater" {
  domain = "casparwater.us"
}

resource "linode_domain_record" "influx" {
  domain_id   = data.linode_domain.casparwater.id
  name        = "influx"
  record_type = "A"
  target      = local.host
  ttl_sec     = 300
}

# Reaps historical pond@*.timer units and leaked podman containers (cf.
# remote-bandwidth-bug.md).  Re-runs only when the teardown script itself
# changes, or when the underlying host is replaced.
resource "null_resource" "teardown" {
  triggers = {
    script_hash = filesha256(local.teardown_src)
    host_id     = linode_instance.debian12-us-west.id
  }

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(local.ssh_key)
    host        = local.host
  }

  provisioner "file" {
    source      = local.teardown_src
    destination = "/tmp/teardown_script.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/teardown_script.sh",
      "/tmp/teardown_script.sh",
    ]
  }
}

# Ensures /home/jmacd/watertown/www exists, jmacd owns it, and the watershop
# deploy key is in authorized_keys so site rsyncs land cleanly.
resource "null_resource" "user_setup" {
  triggers = {
    deploy_key_hash = filesha256(local.deploy_key_src)
    host_id         = linode_instance.debian12-us-west.id
  }

  depends_on = [null_resource.teardown]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(local.ssh_key)
    host        = local.host
  }

  provisioner "remote-exec" {
    inline = [
      "mkdir -p ${local.base_dir}/www",
      "mkdir -p ${local.home}/.ssh",
      "chown -R jmacd:jmacd ${local.base_dir}",
    ]
  }

  provisioner "file" {
    source      = local.deploy_key_src
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
}

# Installs caddy + rsync if missing.  Idempotent; re-runs only when the
# setup script changes.
#
# Depends on teardown DIRECTLY, not just transitively through user_setup.
# teardown ends by stopping caddy; system_setup and caddyfile then start it
# back up.  When user_setup is not in a given change set its dependency edge
# is already satisfied, so a transitive-only order lets teardown race the
# start steps and its trailing `systemctl stop caddy` can win, leaving caddy
# down.  A direct edge forces teardown to finish first.
resource "null_resource" "system_setup" {
  triggers = {
    script_hash = filesha256(local.setup_src)
    host_id     = linode_instance.debian12-us-west.id
  }

  depends_on = [
    null_resource.user_setup,
    null_resource.teardown,
  ]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(local.ssh_key)
    host        = local.host
  }

  provisioner "file" {
    source      = local.setup_src
    destination = "/tmp/setup_script.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/setup_script.sh",
      "/tmp/setup_script.sh",
      # teardown_script.sh stops caddy; setup may have just installed it but
      # not started it.  Ensure it's enabled+running before the Caddyfile
      # resource tries to reload.
      "systemctl enable --now caddy",
    ]
  }
}

# Uploads /etc/influxdb/config.toml.  Backs up the live file before
# overwriting so a manual rollback is one cp away.  Re-runs only when
# the influxdb.toml content changes.
resource "null_resource" "influxdb_config" {
  triggers = {
    config_hash = filesha256(local.influxdb_config_src)
    host_id     = linode_instance.debian12-us-west.id
  }

  depends_on = [null_resource.system_setup]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(local.ssh_key)
    host        = local.host
  }

  # cp -n keeps the first backup from being overwritten by later applies.
  provisioner "remote-exec" {
    inline = [
      "cp -n ${local.influxdb_config_dst} ${local.influxdb_config_dst}.pre-caddy-terminated || true",
    ]
  }

  provisioner "file" {
    source      = local.influxdb_config_src
    destination = local.influxdb_config_dst
  }

  provisioner "remote-exec" {
    inline = [
      "systemctl restart influxdb",
    ]
  }
}

# Uploads /etc/caddy/Caddyfile.  Validates before reload to avoid breaking
# the apex site; reload-or-restart handles the case where teardown stopped
# caddy.  Re-runs only when the Caddyfile content changes.  Ordered AFTER
# influxdb_config so a Phase 2-style apply (where both files change) brings
# InfluxDB to its new port before Caddy retargets its upstream.
resource "null_resource" "caddyfile" {
  triggers = {
    caddyfile_hash = filesha256(local.caddyfile_src)
    host_id        = linode_instance.debian12-us-west.id
  }

  depends_on = [
    null_resource.system_setup,
    null_resource.influxdb_config,
  ]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(local.ssh_key)
    host        = local.host
  }

  provisioner "file" {
    source      = local.caddyfile_src
    destination = local.caddyfile_dst
  }

  provisioner "remote-exec" {
    inline = [
      "caddy validate --config ${local.caddyfile_dst} --adapter caddyfile",
      "systemctl reload-or-restart caddy",
    ]
  }
}
