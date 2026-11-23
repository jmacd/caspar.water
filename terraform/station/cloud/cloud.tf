terraform {
  required_providers {
    linode = {
      source = "linode/linode"
    }
    local = {
      source = "hashicorp/local"
    }
  }
}

variable "li_token" {
  description = "The token for linode"
  type        = string
  sensitive   = true
}

variable "azure_storage_account" {
  description = "Azure storage account holding production pond mirrors"
  type        = string
}

variable "azure_tenant_id" {
  description = "Azure tenant for the site-prod read-only service principal"
  type        = string
  sensitive   = true
}

variable "azure_site_credentials" {
  description = "Read-only Azure service-principal credentials for site-prod"
  type = object({
    client_id     = string
    client_secret = string
  })
  sensitive = true
}

variable "azure_analytics_credentials" {
  description = "Azure service-principal credentials for the analytics-prod backup container"
  type = object({
    client_id     = string
    client_secret = string
  })
  sensitive = true
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
  config_src          = "${path.module}/../../../config"
  beacons_src         = "${local.config_src}/caddy/beacons.caddy"
  beacon_log_root     = "/var/log/caddy/beacons/prod"
  analytics_sites     = { caspar = "Caspar Water", noyo = "Noyo Harbor" }

  site_env_content = join("\n", [
    "POND=${local.home}/pond-site-prod",
    "POND_RUNTIME=native",
    "POND_MEMORY_LIMIT_MB=512",
    "DEB_CHANNEL=prod",
    "AZURE_STORAGE_ACCOUNT=${var.azure_storage_account}",
    "AZURE_TENANT_ID=${var.azure_tenant_id}",
    "AZURE_CLIENT_ID=${var.azure_site_credentials.client_id}",
    "AZURE_CLIENT_SECRET=${var.azure_site_credentials.client_secret}",
    "WATER_AZURE_URL=az://water-prod-0003",
    "NOYO_AZURE_URL=az://noyo-prod-0003",
    "SEPTIC_AZURE_URL=az://septic-prod-0003",
    "SITE_BASE_URL=/",
    "SITE_DEPLOY_BASE=${local.base_dir}/www",
    "SKIP_REMOTE_PULLS=0",
    "GIT_REF=main",
    "RUST_LOG=info",
    "",
  ])

  # analytics-prod reads the beacon logs written by this host's Caddy and the
  # page inventories in the site build that Watershop rsyncs here.
  analytics_env_content = join("\n", [
    "POND=${local.home}/pond-analytics-prod",
    "POND_RUNTIME=native",
    "POND_MEMORY_LIMIT_MB=256",
    "AZURE_STORAGE_ACCOUNT=${var.azure_storage_account}",
    "AZURE_TENANT_ID=${var.azure_tenant_id}",
    "AZURE_CLIENT_ID=${var.azure_analytics_credentials.client_id}",
    "AZURE_CLIENT_SECRET=${var.azure_analytics_credentials.client_secret}",
    "AZURE_URL=az://analytics-prod-0001",
    "ANALYTICS_LOG_ROOT=${local.beacon_log_root}",
    "ANALYTICS_SITE_ROOT=${local.base_dir}/www/current",
    "RUST_LOG=info",
    "",
  ])

  analytics_config_content = join("\n", concat([
    for site, title in local.analytics_sites :
    templatefile("${local.config_src}/analytics/site-pond.yaml.tftpl", {
      site = site, title = title, environment = "prod", log_root = local.beacon_log_root
    })
    ], [<<-YAML
version: v1
kind: copy
metadata:
  path: /analytics/templates/overview.md
spec:
  source: host://${local.base_dir}/config/analytics/site/overview.md
  overwrite: true
YAML
  ]))

  # An instance-specific unit overrides the 3h pond-native@.timer template.
  analytics_timer_content = <<-EOT
[Unit]
Description=Watertown native analytics-prod (every 5min)

[Timer]
OnBootSec=9min
OnUnitActiveSec=5min

[Install]
WantedBy=timers.target
EOT

  analytics_pond = "${local.base_dir}/config/scripts/pond-native.sh analytics-prod"

  host = tolist(linode_instance.debian12-us-west.ipv4)[0]
}

resource "local_sensitive_file" "site_prod_env" {
  filename        = "${path.module}/env/site-prod.env"
  file_permission = "0600"
  content         = local.site_env_content
}

resource "local_sensitive_file" "analytics_prod_env" {
  filename        = "${path.module}/env/analytics-prod.env"
  file_permission = "0600"
  content         = local.analytics_env_content
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

# Install the promoted native Watertown package and the site-prod runtime.
# The pond itself is copied separately from Watershop so its identity and
# remote watermarks survive the host move; this resource never initializes,
# resets, or deletes pond state.
resource "null_resource" "site_runtime" {
  triggers = {
    runtime_hash = sha256(join("", [
      filesha256("${local.config_src}/scripts/install-oras.sh"),
      filesha256("${local.config_src}/scripts/pond-native.sh"),
      filesha256("${local.config_src}/scripts/run.sh"),
      filesha256("${local.config_src}/scripts/update-selfmon.sh"),
      filesha256("${local.config_src}/site.yaml"),
      filesha256("${local.config_src}/remotes/site-azure.yaml"),
      filesha256("${local.config_src}/systemd/pond-native@.service"),
      filesha256("${local.config_src}/systemd/pond-native@.timer"),
      filesha256("${local.config_src}/systemd/pond-native-update@.service"),
      filesha256("${local.config_src}/systemd/pond-native-update@.timer"),
    ]))
    env_hash = nonsensitive(sha256(local.site_env_content))
    host_id  = linode_instance.debian12-us-west.id
  }

  depends_on = [
    null_resource.system_setup,
    local_sensitive_file.site_prod_env,
  ]

  connection {
    type        = "ssh"
    user        = "root"
    private_key = file(local.ssh_key)
    host        = local.host
  }

  provisioner "remote-exec" {
    inline = [
      "install -d -o jmacd -g jmacd -m 0755 ${local.base_dir}/config ${local.base_dir}/env ${local.base_dir}/www",
      "install -d -o jmacd -g jmacd -m 0700 ${local.home}/.config/systemd/user",
    ]
  }

  provisioner "file" {
    source      = "${local.config_src}/"
    destination = "${local.base_dir}/config"
  }

  provisioner "file" {
    source      = local_sensitive_file.site_prod_env.filename
    destination = "/tmp/site-prod.env"
  }

  provisioner "remote-exec" {
    inline = [
      "install -o jmacd -g jmacd -m 0600 /tmp/site-prod.env ${local.base_dir}/env/site-prod.env",
      "rm /tmp/site-prod.env",
      "chmod +x ${local.base_dir}/config/scripts/*.sh",
      "install -o jmacd -g jmacd -m 0644 ${local.base_dir}/config/systemd/pond-native@.service ${local.home}/.config/systemd/user/",
      "install -o jmacd -g jmacd -m 0644 ${local.base_dir}/config/systemd/pond-native@.timer ${local.home}/.config/systemd/user/",
      "install -m 0644 ${local.base_dir}/config/systemd/pond-native-update@.service /etc/systemd/system/",
      "install -m 0644 ${local.base_dir}/config/systemd/pond-native-update@.timer /etc/systemd/system/",
      "${local.base_dir}/config/scripts/install-oras.sh",
      "${local.base_dir}/config/scripts/update-selfmon.sh site-prod",
      "systemctl daemon-reload",
      "systemctl enable --now pond-native-update@site-prod.timer",
      "su - jmacd -c 'XDG_RUNTIME_DIR=/run/user/$(id -u); export XDG_RUNTIME_DIR; systemctl --user daemon-reload'",
      # Deliberately do not start the site timer until the preserved pond has
      # been copied and its Azure remotes have been verified.
      "chown -R jmacd:jmacd ${local.base_dir}",
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
    beacons_hash   = filesha256(local.beacons_src)
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

  # Caddy writes the beacon logs; the analytics-prod pond reads them as jmacd.
  provisioner "remote-exec" {
    inline = [
      "set -e",
      "install -d -o caddy -g jmacd -m 2750 ${join(" ", [for site in keys(local.analytics_sites) : "${local.beacon_log_root}/${site}"])}",
    ]
  }

  provisioner "file" {
    source      = local.beacons_src
    destination = "/tmp/beacons.caddy"
  }

  provisioner "file" {
    source      = local.caddyfile_src
    destination = "/tmp/Caddyfile"
  }

  # Validate as caddy so any log file opened during validation has the
  # service's ownership rather than root's.
  provisioner "remote-exec" {
    inline = [
      "set -e",
      "install -o root -g root -m 0644 /tmp/beacons.caddy /etc/caddy/beacons.caddy",
      "runuser -u caddy -- caddy validate --config /tmp/Caddyfile --adapter caddyfile",
      "install -o root -g root -m 0644 /tmp/Caddyfile ${local.caddyfile_dst}",
      "rm /tmp/beacons.caddy /tmp/Caddyfile",
      "systemctl reload-or-restart caddy",
    ]
  }
}

# Native analytics-prod pond: ingests this host's beacon logs every 5 minutes,
# publishes /analytics/{caspar,noyo}/, and backs up to its own Azure container.
# Initialization is idempotent; this resource never resets pond state.  Until
# the first beacon arrives (and the site build carries page inventories) each
# run publishes an "unavailable" report and exits non-zero.
resource "null_resource" "analytics_runtime" {
  triggers = {
    runtime_hash = sha256(join("", [
      filesha256("${local.config_src}/scripts/pond-native.sh"),
      filesha256("${local.config_src}/scripts/run.sh"),
      filesha256("${local.config_src}/scripts/run-analytics.sh"),
      filesha256("${local.config_src}/remotes/producer-azure.yaml"),
      filesha256("${local.config_src}/analytics/site/overview.md"),
    ]))
    config_hash = sha256(local.analytics_config_content)
    timer_hash  = sha256(local.analytics_timer_content)
    env_hash    = nonsensitive(sha256(local.analytics_env_content))
    host_id     = linode_instance.debian12-us-west.id
  }

  depends_on = [
    null_resource.site_runtime,
    null_resource.caddyfile,
    local_sensitive_file.analytics_prod_env,
  ]

  connection {
    type        = "ssh"
    user        = "jmacd"
    private_key = file(local.ssh_key)
    host        = local.host
  }

  provisioner "remote-exec" {
    inline = [
      "set -e",
      "install -d -m 0755 ${local.base_dir}/config ${local.base_dir}/env ${local.base_dir}/www/analytics-prod",
      "install -d -m 0700 ${local.base_dir}/incoming-env",
    ]
  }

  provisioner "file" {
    source      = "${local.config_src}/"
    destination = "${local.base_dir}/config"
  }

  provisioner "file" {
    content     = local.analytics_config_content
    destination = "${local.base_dir}/config/analytics-prod.yaml"
  }

  provisioner "file" {
    source      = local_sensitive_file.analytics_prod_env.filename
    destination = "${local.base_dir}/incoming-env/analytics-prod.env"
  }

  provisioner "file" {
    content     = local.analytics_timer_content
    destination = "${local.home}/.config/systemd/user/pond-native@analytics-prod.timer"
  }

  provisioner "remote-exec" {
    inline = [
      "set -e",
      "install -m 0600 ${local.base_dir}/incoming-env/analytics-prod.env ${local.base_dir}/env/analytics-prod.env",
      "rm -r ${local.base_dir}/incoming-env",
      "chmod +x ${local.base_dir}/config/scripts/*.sh",
      "pond_dir=$(. ${local.base_dir}/env/analytics-prod.env && echo \"$POND\"); if [ -d \"$pond_dir/data/_delta_log\" ]; then echo '[init] analytics-prod: already initialized'; else ${local.analytics_pond} init --birthplace analytics-prod; fi",
      "${local.analytics_pond} apply -f ${local.base_dir}/config/analytics-prod.yaml",
      # Same attach-and-verify sequence as Watershop's production producers.
      "raw_listing=$(${local.analytics_pond} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); unexpected=$(printf '%s\\n' \"$raw_names\" | awk '$1 != \"azure\" { print $1 }'); for remote in $unexpected; do echo \"[remote] analytics-prod: detaching unexpected $remote\"; ${local.analytics_pond} remote remove \"$remote\"; done; ${local.analytics_pond} apply -f ${local.base_dir}/config/remotes/producer-azure.yaml; ${local.analytics_pond} push azure; raw_listing=$(${local.analytics_pond} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); remotes=$(${local.analytics_pond} remote list) || exit 1; [ \"$raw_names\" = azure ] && printf '%s\\n' \"$remotes\" | awk -v url='az://analytics-prod-0001' 'NR == 1 { next } $1 == \"azure\" && $2 == url && $3 == \"push\" && $4 == \"-\" { found++; next } { unexpected=1 } END { exit !(found == 1 && !unexpected) }' || exit 1; echo '[remote] analytics-prod: Azure remote converged'",
      "systemctl --user daemon-reload",
      "systemctl --user enable --now pond-native@analytics-prod.timer",
    ]
  }
}
