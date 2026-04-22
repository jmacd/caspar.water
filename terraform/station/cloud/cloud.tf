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

# Cloudflare R2 (prod buckets)
variable "r2_endpoint" {
  sensitive = true
}

variable "r2_access_key" {
  sensitive = true
}

variable "r2_secret_key" {
  sensitive = true
}

variable "reset_instance" {
  description = "Wipe and re-initialize the site pond (volume destroyed)"
  type        = bool
  default     = false
}

provider "linode" {
  token = var.li_token
}

locals {
  home     = "/home/jmacd"
  base_dir = "${local.home}/duckpond"
  instance = "site-prod"
}

resource "linode_instance" "debian12-us-west" {
  region = "us-west"
  type = "g6-nanode-1"
}

# Generate env file for site-prod instance
resource "local_file" "site_prod_env" {
  filename        = "${path.module}/env/site-prod.env"
  file_permission = "0600"
  content = join("\n", [
    "POND_VOLUME=pond-site-prod",
    "S3_URL=",
    "S3_ENDPOINT=${var.r2_endpoint}",
    "S3_REGION=auto",
    "S3_ACCESS_KEY=${var.r2_access_key}",
    "S3_SECRET_KEY=${var.r2_secret_key}",
    "S3_ALLOW_HTTP=false",
    "WATER_S3_URL=s3://water-pond",
    "NOYO_S3_URL=s3://noyo-pond",
    "SEPTIC_S3_URL=s3://septic-pond",
    "SITE_BASE_URL=/",
    "GIT_REF=main",
    "RUST_LOG=info",
    "",
  ])
}

# Generate timer file for site-prod
resource "local_file" "site_prod_timer" {
  filename        = "${path.module}/timers/pond@site-prod.timer"
  file_permission = "0644"
  content = <<-EOT
[Unit]
Description=DuckPond site-prod (every 15min)

[Timer]
OnBootSec=2min
OnUnitActiveSec=15min

[Install]
WantedBy=timers.target
EOT
}

resource "null_resource" "cloud" {
  depends_on = [local_file.site_prod_env, local_file.site_prod_timer]

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

  # Teardown previous state
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/teardown_script.sh",
      "/tmp/teardown_script.sh",
    ]
  }

  # Create directory structure
  provisioner "remote-exec" {
    inline = [
      "rm -rf ${local.base_dir}/config ${local.base_dir}/env ${local.base_dir}/timers",
      "mkdir -p ${local.base_dir}/config",
      "mkdir -p ${local.base_dir}/duckpond",
      "mkdir -p ${local.base_dir}/env",
      "mkdir -p ${local.base_dir}/timers",
      "mkdir -p ${local.base_dir}/www",
      "mkdir -p ${local.home}/.config/systemd/user",
    ]
  }

  # Push shared configs from repo root
  provisioner "file" {
    source      = "../../../config/"
    destination = "${local.base_dir}/config"
  }

  # Push duckpond VERSION (pinned version for production images)
  provisioner "file" {
    source      = "../../../duckpond/VERSION"
    destination = "${local.base_dir}/duckpond/VERSION"
  }

  # Push env file
  provisioner "file" {
    source      = "${path.module}/env/"
    destination = "${local.base_dir}/env"
  }

  # Push timer file
  provisioner "file" {
    source      = "${path.module}/timers/"
    destination = "${local.base_dir}/timers"
  }

  # Run setup (installs caddy + podman, initializes pond)
  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/setup_script.sh",
      "/tmp/setup_script.sh",
    ]
  }

  # Set up systemd and initialize pond
  #
  # Note: systemctl --user requires XDG_RUNTIME_DIR when run via su from root.
  # We set it explicitly since machinectl is not available on minimal Debian.
  provisioner "remote-exec" {
    inline = concat(
      [
        "chmod +x ${local.base_dir}/config/scripts/*.sh",
        "chmod 600 ${local.base_dir}/env/*.env",
        "chown -R jmacd:jmacd ${local.base_dir}",
        "cp ${local.base_dir}/config/systemd/pond@.service ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/timers/pond@*.timer ${local.home}/.config/systemd/user/",
        "chown -R jmacd:jmacd ${local.home}/.config/systemd",
        "su - jmacd -c 'XDG_RUNTIME_DIR=/run/user/$(id -u) systemctl --user daemon-reload'",
      ],
      # Reset instance if requested
      var.reset_instance ? [
        "su - jmacd -c 'XDG_RUNTIME_DIR=/run/user/$(id -u) systemctl --user stop pond@${local.instance}.timer 2>/dev/null || true'",
        "su - jmacd -c 'podman volume rm pond-${local.instance} 2>/dev/null || true'",
      ] : [],
      [
        # Initialize and apply config
        "su - jmacd -c '${local.base_dir}/config/scripts/pond.sh ${local.instance} init 2>/dev/null || true'",
        "su - jmacd -c '${local.base_dir}/config/scripts/pond.sh ${local.instance} apply -f /config/site.yaml'",
        # Enable and start timer
        "su - jmacd -c 'XDG_RUNTIME_DIR=/run/user/$(id -u) systemctl --user enable --now pond@${local.instance}.timer'",
      ],
    )
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
