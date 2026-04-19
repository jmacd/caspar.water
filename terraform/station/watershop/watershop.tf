locals {
  home     = "/home/${var.user}"
  base_dir = "${local.home}/duckpond"

  # Staging instances use MinIO, production uses R2
  staging_s3 = {
    endpoint   = var.minio_endpoint
    region     = "us-east-1"
    access_key = var.minio_access_key
    secret_key = var.minio_secret_key
    allow_http = "true"
  }
  prod_s3 = {
    endpoint   = var.r2_endpoint
    region     = "auto"
    access_key = var.r2_access_key
    secret_key = var.r2_secret_key
    allow_http = "false"
  }

  # Instance definitions
  instances = {
    noyo-staging = {
      s3         = local.staging_s3
      s3_url     = "s3://noyo-staging"
      interval   = "30min"
      boot_delay = "5min"
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/\nNOYO_ARCHIVE_DIR=${var.noyo_archive_dir}\nGIT_REF=${var.git_ref}"
    }
    noyo-prod = {
      s3         = local.prod_s3
      s3_url     = "s3://noyo-pond"
      interval   = "30min"
      boot_delay = "6min"
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/\nNOYO_ARCHIVE_DIR=${var.noyo_archive_dir}\nGIT_REF=main"
    }
    water-staging = {
      s3         = local.staging_s3
      s3_url     = "s3://water-staging"
      interval   = "10min"
      boot_delay = "2min"
      extra_env  = "DATA_DIR=${var.water_data_dir}\nSITE_BASE_URL=/"
    }
    water-prod = {
      s3         = local.prod_s3
      s3_url     = "s3://water-pond"
      interval   = "10min"
      boot_delay = "3min"
      extra_env  = "DATA_DIR=${var.water_data_dir}\nSITE_BASE_URL=/"
    }
    septic-staging = {
      s3         = local.staging_s3
      s3_url     = "s3://septic-staging"
      interval   = "10min"
      boot_delay = "4min"
      extra_env  = "DATA_DIR=${var.septic_data_dir}\nSITE_BASE_URL=/"
    }
    septic-prod = {
      s3         = local.prod_s3
      s3_url     = "s3://septic-pond"
      interval   = "10min"
      boot_delay = "5min"
      extra_env  = "DATA_DIR=${var.septic_data_dir}\nSITE_BASE_URL=/"
    }
    site-staging = {
      s3         = local.staging_s3
      s3_url     = ""
      interval   = "15min"
      boot_delay = "7min"
      extra_env  = "WATER_S3_URL=s3://water-staging\nNOYO_S3_URL=s3://noyo-staging\nSEPTIC_S3_URL=s3://septic-staging\nSITE_BASE_URL=/\nGIT_REF=${var.git_ref}"
    }
  }

  # All instance names for iteration, filtered by deploy flags
  instance_names = [for name in keys(local.instances) :
    name if (
      (var.deploy_staging && endswith(name, "-staging")) ||
      (var.deploy_production && endswith(name, "-prod"))
    )
  ]
}

# Generate env files locally for upload
resource "local_file" "env_files" {
  for_each = local.instances

  filename        = "${path.module}/env/${each.key}.env"
  file_permission = "0600"
  content = join("\n", [
    "POND_VOLUME=pond-${each.key}",
    "S3_URL=${each.value.s3_url}",
    "S3_ENDPOINT=${each.value.s3.endpoint}",
    "S3_REGION=${each.value.s3.region}",
    "S3_ACCESS_KEY=${each.value.s3.access_key}",
    "S3_SECRET_KEY=${each.value.s3.secret_key}",
    "S3_ALLOW_HTTP=${each.value.s3.allow_http}",
    each.value.extra_env,
    "RUST_LOG=info",
    "",
  ])
}

# Generate timer files locally for upload
resource "local_file" "timer_files" {
  for_each = local.instances

  filename        = "${path.module}/timers/pond@${each.key}.timer"
  file_permission = "0644"
  content = <<-EOT
[Unit]
Description=DuckPond ${each.key} (every ${each.value.interval})

[Timer]
OnBootSec=${each.value.boot_delay}
OnUnitActiveSec=${each.value.interval}

[Install]
WantedBy=timers.target
EOT
}

resource "null_resource" "watershop" {
  depends_on = [local_file.env_files, local_file.timer_files]

  connection {
    type  = "ssh"
    user  = var.user
    agent = true
    host  = var.host
  }

  # Clean and create directory structure (preserve podman volumes)
  provisioner "remote-exec" {
    inline = [
      "rm -rf ${local.base_dir}/config ${local.base_dir}/env ${local.base_dir}/timers",
      "rm -f ${local.base_dir}/*.sh",
      "mkdir -p ${local.base_dir}/config",
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

  # Push generated env files
  provisioner "file" {
    source      = "${path.module}/env/"
    destination = "${local.base_dir}/env"
  }

  # Push generated timer files
  provisioner "file" {
    source      = "${path.module}/timers/"
    destination = "${local.base_dir}/timers"
  }

  # Set up systemd and initialize ponds
  provisioner "remote-exec" {
    inline = concat(
      [
        # Ensure user services survive logout
        "sudo loginctl enable-linger ${var.user}",

        # Make scripts executable
        "chmod +x ${local.base_dir}/config/scripts/*.sh",
        "chmod 600 ${local.base_dir}/env/*.env",

        # Install systemd units
        "cp ${local.base_dir}/config/systemd/pond@.service ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/timers/pond@*.timer ${local.home}/.config/systemd/user/",
        "systemctl --user daemon-reload",
      ],
      # Reset specified instances: stop timer, remove volume
      [for name in var.reset_instances :
        "systemctl --user stop pond@${name}.timer 2>/dev/null || true"
      ],
      [for name in var.reset_instances :
        "podman volume rm pond-${name} 2>/dev/null || true"
      ],
      # Initialize and apply config for each instance
      [for name in local.instance_names :
        "${local.base_dir}/config/scripts/pond.sh ${name} init 2>/dev/null || true"
      ],
      [for name in local.instance_names :
        "${local.base_dir}/config/scripts/pond.sh ${name} apply -f /config/${replace(replace(name, "-staging", ""), "-prod", "")}.yaml"
      ],
      # Enable and start all timers
      [for name in local.instance_names :
        "systemctl --user enable --now pond@${name}.timer"
      ],
    )
  }

  triggers = {
    always_run = timestamp()
  }
}
