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
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/\nGIT_REF=${var.git_ref}"
    }
    noyo-prod = {
      s3         = local.prod_s3
      s3_url     = "s3://noyo-pond"
      interval   = "30min"
      boot_delay = "6min"
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/"
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
    site-prod = {
      s3         = local.prod_s3
      s3_url     = ""
      interval   = "15min"
      boot_delay = "8min"
      extra_env  = "WATER_S3_URL=s3://water-pond\nNOYO_S3_URL=s3://noyo-pond\nSEPTIC_S3_URL=s3://septic-pond\nSITE_BASE_URL=/\nCLOUD_HOST=cloud"
    }
    watershop-selfmon-staging = {
      s3         = local.staging_s3
      s3_url     = "s3://watershop-selfmon-staging"
      interval   = "1min"
      boot_delay = "30s"
      extra_env  = "SELFMON=1"
      selfmon    = true
    }
    watershop-selfmon-prod = {
      s3         = local.prod_s3
      s3_url     = "s3://watershop-selfmon-prod"
      interval   = "1min"
      boot_delay = "1min"
      extra_env  = "SELFMON=1"
      selfmon    = true
    }
  }

  # All instance names for iteration, filtered by deploy flags
  instance_names = [for name in keys(local.instances) :
    name if (
      (var.deploy_staging && endswith(name, "-staging")) ||
      (var.deploy_production && endswith(name, "-prod"))
    )
  ]

  # Containerized vs native (selfmon) instance partitions
  container_instance_names = [for n in local.instance_names :
    n if !lookup(local.instances[n], "selfmon", false)
  ]
  selfmon_instance_names = [for n in local.instance_names :
    n if lookup(local.instances[n], "selfmon", false)
  ]
  # Selfmon tiers actually scheduled (used to decide which binaries to extract).
  selfmon_tiers = toset(compact([
    contains([for n in local.selfmon_instance_names : true if endswith(n, "-staging")], true) ? "staging" : "",
    contains([for n in local.selfmon_instance_names : true if endswith(n, "-prod")],    true) ? "prod"    : "",
  ]))

  # MinIO buckets to ensure exist for staging instances.
  # Production instances target R2 buckets which are managed elsewhere.
  staging_bucket_names = [for n in local.instance_names :
    replace(local.instances[n].s3_url, "s3://", "")
    if endswith(n, "-staging")
  ]
}

# Generate env files locally for upload
resource "local_file" "env_files" {
  for_each = local.instances

  filename        = "${path.module}/env/${each.key}.env"
  file_permission = "0600"
  content = join("\n", [
    "POND_VOLUME=pond-${each.key}",
    "POND=${local.home}/pond-${each.key}",
    "SELFMON_METRICS_DIR=/var/log/duckpond-selfmon/${each.key}",
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

# Generate timer files locally for upload.  Container instances use the
# `pond@.service` template; selfmon instances use `pond-selfmon@.service`
# (native, no podman).
resource "local_file" "timer_files" {
  for_each = local.instances

  filename        = "${path.module}/timers/${lookup(each.value, "selfmon", false) ? "pond-selfmon" : "pond"}@${each.key}.timer"
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

  # Push deploy key for watershop → cloud rsync (site-prod)
  provisioner "file" {
    source      = "${path.module}/deploy_key"
    destination = "${local.home}/.ssh/cloud_deploy"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod 600 ${local.home}/.ssh/cloud_deploy",
      # Configure SSH to use the deploy key for the cloud host
      "grep -q 'Host cloud' ${local.home}/.ssh/config 2>/dev/null || cat >> ${local.home}/.ssh/config <<EOF\n\nHost cloud\n  HostName ${var.cloud_ip}\n  User jmacd\n  IdentityFile ${local.home}/.ssh/cloud_deploy\n  StrictHostKeyChecking no\nEOF",
    ]
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

        # Install systemd unit templates (container + selfmon native)
        "cp ${local.base_dir}/config/systemd/pond@.service ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/config/systemd/pond-selfmon@.service ${local.home}/.config/systemd/user/",
        # Install both timer styles (pond@*.timer and pond-selfmon@*.timer)
        "cp ${local.base_dir}/timers/pond@*.timer ${local.home}/.config/systemd/user/ 2>/dev/null || true",
        "cp ${local.base_dir}/timers/pond-selfmon@*.timer ${local.home}/.config/systemd/user/ 2>/dev/null || true",
        "systemctl --user daemon-reload",
      ],
      # Install the duckpond .deb (built natively on watershop by
      # tools/build-on-watershop.sh) and create per-tier
      # /usr/local/bin/pond-selfmon-<tier> aliases.
      [for tier in local.selfmon_tiers :
        "${local.base_dir}/config/scripts/install-duckpond.sh ${tier}"
      ],
      # Provision per-instance metrics dir (writable by the user that
      # runs the selfmon timer).
      [for name in local.selfmon_instance_names :
        "sudo install -d -o ${var.user} -g ${var.user} -m 0755 /var/log/duckpond-selfmon/${name}"
      ],
      # Sitegen output dirs served by Caddy at /selfmon/.  Owned by
      # ${var.user} so the per-tick render writes here without sudo.
      [for name in local.selfmon_instance_names :
        "sudo install -d -o ${var.user} -g ${var.user} -m 0755 /var/www/selfmon/${name}"
      ],
      # Ensure MinIO buckets exist for all staging instances (idempotent).
      # Uses the aws-cli container against localhost:9000.  The container
      # auto-pulls on first use; subsequent invocations are fast.
      # `mb` returns non-zero if the bucket already exists, so we swallow.
      [for bucket in local.staging_bucket_names :
        "podman run --rm --network=host -e AWS_ACCESS_KEY_ID=${var.minio_access_key} -e AWS_SECRET_ACCESS_KEY=${var.minio_secret_key} docker.io/amazon/aws-cli --endpoint-url http://localhost:9000 s3 mb s3://${bucket} --region us-east-1 2>&1 | grep -v 'BucketAlreadyOwnedByYou\\|already exists' || true"
      ],
      # Reset specified instances: stop timer, remove volume / pond dir
      [for name in var.reset_instances :
        "systemctl --user stop pond@${name}.timer pond-selfmon@${name}.timer 2>/dev/null || true"
      ],
      [for name in var.reset_instances :
        "podman volume rm pond-${name} 2>/dev/null || true"
      ],
      [for name in var.reset_instances :
        "rm -rf ${local.home}/pond-${name}"
      ],
      # Initialize containerized instances
      [for name in local.container_instance_names :
        "${local.base_dir}/config/scripts/pond.sh ${name} init 2>/dev/null || true"
      ],
      # Apply containerized instance configs
      [for name in local.container_instance_names :
        "${local.base_dir}/config/scripts/pond.sh ${name} apply -f /config/${replace(replace(name, "-staging", ""), "-prod", "")}.yaml"
      ],
      # Initialize selfmon instances natively (POND comes from env file)
      [for name in local.selfmon_instance_names :
        "set -a; . ${local.base_dir}/env/${name}.env; set +a; /usr/local/bin/pond-selfmon-${endswith(name, "-staging") ? "staging" : "prod"} init 2>/dev/null || true"
      ],
      # Apply selfmon configs natively
      [for name in local.selfmon_instance_names :
        "set -a; . ${local.base_dir}/env/${name}.env; set +a; /usr/local/bin/pond-selfmon-${endswith(name, "-staging") ? "staging" : "prod"} apply -f ${local.base_dir}/config/${replace(replace(name, "-staging", ""), "-prod", "")}.yaml"
      ],
      # Enable and start container timers
      [for name in local.container_instance_names :
        "systemctl --user enable --now pond@${name}.timer"
      ],
      # Enable and start selfmon timers
      [for name in local.selfmon_instance_names :
        "systemctl --user enable --now pond-selfmon@${name}.timer"
      ],
    )
  }

  # Push Caddyfile and reload caddy
  provisioner "file" {
    source      = "Caddyfile"
    destination = "/tmp/Caddyfile"
  }

  provisioner "remote-exec" {
    inline = [
      "sudo install -d -o caddy -g caddy -m 0755 /var/log/caddy",
      "sudo install -o root -g root -m 0644 /tmp/Caddyfile /etc/caddy/Caddyfile",
      "rm /tmp/Caddyfile",
      "sudo caddy validate --config /etc/caddy/Caddyfile",
      "sudo systemctl reload caddy",
    ]
  }

  triggers = {
    always_run = timestamp()
  }
}
