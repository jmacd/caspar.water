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
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/\nNOYO_ARCHIVE_DIR=${var.noyo_archive_dir}"
    }
    noyo-prod = {
      s3         = local.prod_s3
      s3_url     = "s3://noyo-pond"
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/\nNOYO_ARCHIVE_DIR=${var.noyo_archive_dir}"
    }
    water-staging = {
      s3         = local.staging_s3
      s3_url     = "s3://water-staging"
      extra_env  = "DATA_DIR=${var.water_data_dir}\nSITE_BASE_URL=/"
    }
    water-prod = {
      s3         = local.prod_s3
      s3_url     = "s3://water-pond"
      extra_env  = "DATA_DIR=${var.water_data_dir}\nSITE_BASE_URL=/"
    }
    septic-staging = {
      s3         = local.staging_s3
      s3_url     = "s3://septic-staging"
      extra_env  = "DATA_DIR=${var.septic_data_dir}\nSITE_BASE_URL=/"
    }
    septic-prod = {
      s3         = local.prod_s3
      s3_url     = "s3://septic-pond"
      extra_env  = "DATA_DIR=${var.septic_data_dir}\nSITE_BASE_URL=/"
    }
    site-staging = {
      s3         = local.staging_s3
      s3_url     = ""
      extra_env  = "WATER_S3_URL=s3://water-staging\nNOYO_S3_URL=s3://noyo-staging\nSEPTIC_S3_URL=s3://septic-staging\nSITE_BASE_URL=/"
    }
  }

  # All instance names for iteration
  instance_names = keys(local.instances)
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

resource "null_resource" "watershop" {
  depends_on = [local_file.env_files]

  connection {
    type  = "ssh"
    user  = var.user
    agent = true
    host  = var.host
  }

  # Clean and create directory structure (preserve podman volumes)
  provisioner "remote-exec" {
    inline = [
      "rm -rf ${local.base_dir}/config ${local.base_dir}/site ${local.base_dir}/env",
      "rm -f ${local.base_dir}/*.sh ${local.base_dir}/pond@*",
      "mkdir -p ${local.base_dir}/config",
      "mkdir -p ${local.base_dir}/site",
      "mkdir -p ${local.base_dir}/env",
      "mkdir -p ${local.base_dir}/www",
      "mkdir -p ${local.home}/.config/systemd/user",
    ]
  }

  # Push shared configs from repo root
  provisioner "file" {
    source      = "../../../config/"
    destination = "${local.base_dir}/config"
  }

  # Push site content and configs
  provisioner "file" {
    source      = "../../../site/"
    destination = "${local.base_dir}/site"
  }

  # Push duckpond scripts and systemd units
  provisioner "file" {
    source      = "duckpond/"
    destination = local.base_dir
  }

  # Push generated env files
  provisioner "file" {
    source      = "${path.module}/env/"
    destination = "${local.base_dir}/env"
  }

  # Set up systemd and initialize ponds
  provisioner "remote-exec" {
    inline = concat(
      [
        # Ensure user services survive logout
        "sudo loginctl enable-linger ${var.user}",

        # Make scripts executable
        "chmod +x ${local.base_dir}/*.sh",
        "chmod 600 ${local.base_dir}/env/*.env",

        # Create MinIO buckets for staging
        "${local.base_dir}/setup-minio.sh",

        # Install systemd units
        "cp ${local.base_dir}/pond@.service ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/pond@*.timer ${local.home}/.config/systemd/user/",
        "systemctl --user daemon-reload",
      ],
      # Initialize and apply config for each instance
      [for name in local.instance_names :
        "${local.base_dir}/pond.sh ${name} init 2>/dev/null || true"
      ],
      [for name in local.instance_names :
        "${local.base_dir}/pond.sh ${name} apply -f /config/${replace(replace(name, "-staging", ""), "-prod", "")}.yaml"
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
