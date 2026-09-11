locals {
  home     = "/home/${var.user}"
  base_dir = "${local.home}/watertown"

  # Staging uses the local MinIO service. Production uses Azure exclusively.
  staging_s3 = {
    endpoint   = var.minio_endpoint
    region     = "us-east-1"
    access_key = var.minio_access_key
    secret_key = var.minio_secret_key
    allow_http = "true"
  }
  no_s3 = {
    endpoint   = ""
    region     = ""
    access_key = ""
    secret_key = ""
    allow_http = "false"
  }
  empty_azure_credentials = {
    client_id     = ""
    client_secret = ""
  }
  azure_credentials = merge(
    var.azure_producer_credentials,
    { site-prod = var.azure_site_credentials },
  )

  # Instance definitions
  instances = {
    noyo-staging = {
      s3             = local.staging_s3
      s3_url         = "s3://noyo-staging-0002"
      volume         = "pond-noyo-staging-0002"
      remote_backend = "minio"
      azure_mirror   = false
      interval       = "1h"
      boot_delay     = "5min"
      extra_env      = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/\nNOYO_GIT_REF=${var.noyo_git_ref}"
    }
    noyo-prod = {
      s3         = local.no_s3
      s3_url     = ""
      interval   = "1h"
      boot_delay = "6min"
      volume     = "pond-noyo-prod-0003"
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/\nAZURE_URL=az://noyo-prod-0003"
    }
    water-staging = {
      s3             = local.staging_s3
      s3_url         = "s3://water-staging-0002"
      volume         = "pond-water-staging-0002"
      remote_backend = "minio"
      azure_mirror   = false
      interval       = "1h"
      boot_delay     = "2min"
      extra_env      = "DATA_DIR=${var.water_data_dir}\nSITE_BASE_URL=/"
    }
    water-prod = {
      s3         = local.no_s3
      s3_url     = ""
      interval   = "1h"
      boot_delay = "3min"
      volume     = "pond-water-prod-0003"
      extra_env  = "DATA_DIR=${var.water_data_dir}\nSITE_BASE_URL=/\nAZURE_URL=az://water-prod-0003"
    }
    septic-staging = {
      s3             = local.staging_s3
      s3_url         = "s3://septic-staging-0002"
      volume         = "pond-septic-staging-0002"
      remote_backend = "minio"
      azure_mirror   = false
      interval       = "1h"
      boot_delay     = "4min"
      extra_env      = "DATA_DIR=${var.septic_data_dir}\nSITE_BASE_URL=/"
    }
    septic-prod = {
      s3         = local.no_s3
      s3_url     = ""
      interval   = "1h"
      boot_delay = "5min"
      volume     = "pond-septic-prod-0003"
      extra_env  = "DATA_DIR=${var.septic_data_dir}\nSITE_BASE_URL=/\nAZURE_URL=az://septic-prod-0003"
    }
    site-staging = {
      s3             = local.staging_s3
      s3_url         = ""
      volume         = "pond-site-staging-0002"
      remote_backend = "minio"
      interval       = "3h"
      boot_delay     = "7min"
      email_report   = contains(var.weekly_report_email_instances, "site-staging")
      extra_env      = "WATER_S3_URL=s3://water-staging-0002\nNOYO_S3_URL=s3://noyo-staging-0002\nSEPTIC_S3_URL=s3://septic-staging-0002\nSITE_BASE_URL=/\nGIT_REF=${var.git_ref}\nNOYO_GIT_REF=${var.noyo_git_ref}\nPOND_MEMORY_LIMIT_MB=1024"
    }
    site-prod = {
      s3             = local.no_s3
      s3_url         = ""
      volume         = "pond-site-prod-0003"
      remote_backend = "azure"
      interval       = "3h"
      boot_delay     = "8min"
      email_report   = contains(var.weekly_report_email_instances, "site-prod")
      extra_env      = "WATER_AZURE_URL=az://water-prod-0003\nNOYO_AZURE_URL=az://noyo-prod-0003\nSEPTIC_AZURE_URL=az://septic-prod-0003\nSITE_BASE_URL=/\nGIT_REF=main\nNOYO_GIT_REF=main\nCLOUD_HOST=cloud\nPOND_MEMORY_LIMIT_MB=1024"
    }
    watershop-selfmon = {
      s3             = local.staging_s3
      remote_backend = "minio"
      # s3_url provisions a MinIO bucket and the S3_* env used to resolve
      # credentials, but selfmon intentionally gets NO backup remote -- there
      # is no config/remotes/*.yaml applied to it: run-selfmon.sh prunes with
      # --allow-no-remote, which is incompatible with a push backup because
      # the post-commit push reads already-vacuumed files and holds the
      # write.lock, blocking compaction.  The bucket therefore stays
      # unused; kept only so the reset path can still empty it.
      s3_url     = "s3://watershop-selfmon"
      interval   = "1min"
      boot_delay = "30s"
      # POND_MEMORY_LIMIT_MB raises the tlogfs FairSpillPool above its
      # 512MiB default so resolving /logs/journal, which holds hundreds of
      # per-unit jsonl files, has room for its ~207MiB external sort instead
      # of OOMing.  Requires a pond binary built on or after 2026-06-30,
      # commit a84fb9bc; older binaries ignore this and stay at 512MiB.
      extra_env = "SELFMON=1\nPOND_MEMORY_LIMIT_MB=2048\nDEB_CHANNEL=latest"
      selfmon   = true
    }
  }

  # All instance names for iteration, filtered by deploy flags.
  # Selfmon is local-experimental: it always deploys (it's tied to
  # this host, not to a staging/prod tier of any pond).
  instance_names = [for name in keys(local.instances) :
    name if(
      (var.deploy_selfmon && lookup(local.instances[name], "selfmon", false)) ||
      (var.deploy_staging && endswith(name, "-staging")) ||
      (var.deploy_production && endswith(name, "-prod"))
    )
  ]

  # Every configured instance name regardless of deploy flag, used
  # to decide which existing systemd units the cleanup step should
  # leave alone.  Toggling deploy_production=false should NOT disable
  # the running -prod timers; only retired/renamed instances should
  # be cleaned up.
  all_configured_names = keys(local.instances)

  # Containerized vs native (selfmon) instance partitions
  container_instance_names = [for n in local.instance_names :
    n if !lookup(local.instances[n], "selfmon", false)
  ]
  selfmon_instance_names = [for n in local.instance_names :
    n if lookup(local.instances[n], "selfmon", false)
  ]
  instance_volumes = {
    for name, instance in local.instances :
    name => lookup(instance, "volume", "pond-${name}")
  }

  # MinIO buckets are staging-only. Production uses Azure.
  staging_bucket_names = [for n in local.instance_names :
    replace(local.instances[n].s3_url, "s3://", "")
    if local.instances[n].s3_url != ""
  ]
  staging_site_reseed = var.deploy_staging && length(setintersection(
    toset(var.reset_instances),
    toset(["noyo-staging", "septic-staging", "water-staging", "site-staging"]),
  )) > 0
}

# Generate env files locally for upload
resource "local_file" "env_files" {
  for_each = local.instances

  filename        = "${path.module}/env/${each.key}.env"
  file_permission = "0600"
  content = join("\n", [
    "POND_VOLUME=${local.instance_volumes[each.key]}",
    "POND=${local.home}/pond-${each.key}",
    "SELFMON_METRICS_DIR=/var/log/watertown-selfmon/${each.key}",
    "S3_URL=${each.value.s3_url}",
    "S3_ENDPOINT=${each.value.s3.endpoint}",
    "S3_REGION=${each.value.s3.region}",
    "S3_ACCESS_KEY=${each.value.s3.access_key}",
    "S3_SECRET_KEY=${each.value.s3.secret_key}",
    "S3_ALLOW_HTTP=${each.value.s3.allow_http}",
    "AZURE_STORAGE_ACCOUNT=${var.azure_storage_account}",
    "AZURE_TENANT_ID=${var.azure_tenant_id}",
    "AZURE_CLIENT_ID=${lookup(local.azure_credentials, each.key, local.empty_azure_credentials).client_id}",
    "AZURE_CLIENT_SECRET=${lookup(local.azure_credentials, each.key, local.empty_azure_credentials).client_secret}",
    lookup(each.value, "email_report", false) ? "ACS_EMAIL_ENDPOINT=${var.weekly_report_email_credentials.endpoint}" : "",
    lookup(each.value, "email_report", false) ? "ACS_EMAIL_ACCESS_KEY=${var.weekly_report_email_credentials.access_key}" : "",
    lookup(each.value, "email_report", false) ? "WEEKLY_REPORT_RECIPIENT=${var.weekly_report_email_credentials.recipient}" : "",
    each.value.extra_env,
    "RUST_LOG=info",
    "",
  ])
}

# Generate a single MinIO admin env file used only for the bucket-create
# step in the remote-exec provisioner.  We push the credentials via a
# `file` provisioner (encrypted in transit by SSH, file mode 0600 on
# disk) and reference them with podman's `--env-file` rather than
# `-e KEY=${var...}`.  This keeps `var.minio_access_key` /
# `var.minio_secret_key` out of the inline= [...] string list, which
# would otherwise force terraform to mark every line of remote-exec
# output for this resource as "(output suppressed due to sensitive
# value in config)" -- an all-or-nothing per-resource gate that hides
# unrelated init / podman / sitegen lines too.
resource "local_file" "minio_admin_env" {
  filename        = "${path.module}/env/_minio-admin.env"
  file_permission = "0600"
  content = join("\n", [
    "AWS_ACCESS_KEY_ID=${var.minio_access_key}",
    "AWS_SECRET_ACCESS_KEY=${var.minio_secret_key}",
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
  content         = <<-EOT
[Unit]
Description=Watertown ${each.key} (every ${each.value.interval})

[Timer]
OnBootSec=${each.value.boot_delay}
OnUnitActiveSec=${each.value.interval}

[Install]
WantedBy=timers.target
EOT
}

resource "null_resource" "watershop" {
  depends_on = [
    local_file.env_files,
    local_file.minio_admin_env,
    local_file.timer_files,
  ]

  lifecycle {
    precondition {
      condition = !var.deploy_production || (
        var.azure_storage_account != "" &&
        var.azure_tenant_id != "" &&
        var.azure_site_credentials.client_id != "" &&
        var.azure_site_credentials.client_secret != "" &&
        alltrue([
          for name in ["noyo-prod", "septic-prod", "water-prod"] :
          var.azure_producer_credentials[name].client_id != "" &&
          var.azure_producer_credentials[name].client_secret != ""
        ])
      )
      error_message = "Azure production deployment requires the storage account, tenant, producer credentials, and site credentials."
    }

    precondition {
      condition = length(var.weekly_report_email_instances) == 0 || (
        var.weekly_report_email_credentials.endpoint != "" &&
        var.weekly_report_email_credentials.access_key != "" &&
        var.weekly_report_email_credentials.recipient != ""
      )
      error_message = "Weekly report email instances require a private ACS endpoint, access key, and recipient."
    }

    precondition {
      condition = alltrue([
        for name in var.reset_instances : contains(local.instance_names, name)
      ])
      error_message = "Every reset target must be enabled by the current deployment flags."
    }
  }

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
      "mkdir -p ${local.base_dir}/state",
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
        # Terraform's remote-exec wrapper does not enable fail-fast shell
        # behavior. Without this, a failed pond apply/push can be hidden by
        # later successful commands and the overall apply reports success.
        "set -e",

        # Ensure user services survive logout
        "sudo loginctl enable-linger ${var.user}",

        # Make scripts executable
        "chmod +x ${local.base_dir}/config/scripts/*.sh",
        "chmod 600 ${local.base_dir}/env/*.env",

        # Install systemd unit templates (container + selfmon native)
        "cp ${local.base_dir}/config/systemd/pond@.service ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/config/systemd/pond-selfmon@.service ${local.home}/.config/systemd/user/",
        # Selfmon deb auto-update units (native): hourly oras pull + dpkg -i
        # of the newest pond .deb OCI artifact.  Template units activated
        # per selfmon instance below.
        "cp ${local.base_dir}/config/systemd/pond-selfmon-update@.service ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/config/systemd/pond-selfmon-update@.timer ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/config/systemd/pond-email-report@.service ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/config/systemd/pond-email-report@.timer ${local.home}/.config/systemd/user/",
        # Drop ONLY units whose instance name is no longer in the
        # configured set (e.g. retired watershop-selfmon-{staging,prod}
        # split).  Configured-but-not-deployed instances (e.g. -prod
        # when deploy_production=false) are left alone -- toggling a
        # deploy flag must not disturb the other tier.
        "${local.base_dir}/config/scripts/cleanup-stale-pond-units.sh ${join(" ", local.all_configured_names)}",
        # Quiesce every deployed pond before applying configuration. Timers
        # otherwise can start a transaction between commands and either race
        # a reset or hold the write lock. The desired timer state is restored
        # after all ponds and remotes have converged.
        join(" ; ", concat(
          [for name in local.container_instance_names :
            "systemctl --user stop pond@${name}.timer pond@${name}.service 2>/dev/null || true"
          ],
          [for name in local.selfmon_instance_names :
            "systemctl --user stop pond-selfmon@${name}.timer pond-selfmon@${name}.service 2>/dev/null || true"
          ],
          [for name in ["site-staging", "site-prod"] :
            "systemctl --user stop pond-email-report@${name}.timer pond-email-report@${name}.service 2>/dev/null || true"
          ],
          [for name in ["site-staging", "site-prod"] :
            "systemctl --user stop pond-firstbuild-${name}.timer pond-firstbuild-${name}.service 2>/dev/null || true"
          ],
        )),
        # Reap any leaked pond containers belonging to instances we
        # are about to (re)deploy in this apply.  `podman run --rm`
        # detaches from systemd, so a `systemctl stop` on the .service
        # does NOT kill the running container (cf. cloud Apr 30
        # bandwidth bleed). The per-instance pond volume uniquely scopes the
        # container; filtering by a mutable image tag can miss a running
        # container after that tag is promoted.
        join(" ; ", concat(
          [for name in local.container_instance_names :
            "podman ps --format '{{.Names}}' --filter 'volume=${local.instance_volumes[name]}' | xargs -r podman kill 2>/dev/null || true"
          ],
          [for name in local.container_instance_names :
            "podman ps -aq --filter 'volume=${local.instance_volumes[name]}' | xargs -r podman rm -f 2>/dev/null || true"
          ],
        )),
        # Install both timer styles (pond@*.timer and pond-selfmon@*.timer)
        "cp ${local.base_dir}/timers/pond@*.timer ${local.home}/.config/systemd/user/ 2>/dev/null || true",
        "cp ${local.base_dir}/timers/pond-selfmon@*.timer ${local.home}/.config/systemd/user/ 2>/dev/null || true",
        "systemctl --user daemon-reload",
      ],
      # Install the pinned `oras` client used by update-selfmon.sh to pull
      # the pond .deb OCI artifact.  Only needed where a selfmon instance
      # runs the hourly auto-update timer.
      length(local.selfmon_instance_names) > 0
      ? ["${local.base_dir}/config/scripts/install-oras.sh"]
      : [],
      # Bring the selfmon binary up to the CI-published .deb.  This is the
      # SAME script the hourly timer runs, so an apply and a timer tick
      # converge on the same thing: pull the OCI artifact rust-ci.yml
      # publishes and install it only when strictly newer.  On a fresh box
      # nothing is installed yet, so the version comparison is skipped and
      # this bootstraps the binary.
      #
      # This replaces install-watertown.sh, which `dpkg -i`'d whatever
      # cargo-deb had last left in ~/src/watertown/target/debian ON THE BOX,
      # unconditionally and without a version check.  Once CI took over
      # building .debs nothing refreshed that directory, so every apply
      # DOWNGRADED the pond to a months-old build and the next hourly tick
      # silently put it back -- see the alternating pairs in dpkg.log.  The
      # box must have exactly one source of binaries, and it is CI.
      [for name in local.selfmon_instance_names :
        "${local.base_dir}/config/scripts/update-selfmon.sh ${name}"
      ],
      # Provision per-instance metrics dir (writable by the user that
      # runs the selfmon timer).
      [for name in local.selfmon_instance_names :
        "sudo install -d -o ${var.user} -g ${var.user} -m 0755 /var/log/watertown-selfmon/${name}"
      ],
      # Sitegen output dirs served by Caddy at /selfmon/.  Owned by
      # ${var.user} so the per-tick render writes here without sudo.
      [for name in local.selfmon_instance_names :
        "sudo install -d -o ${var.user} -g ${var.user} -m 0755 /var/www/selfmon/${name}"
      ],
      # Ensure MinIO buckets exist for all instances that have an s3_url
      # (staging plus selfmon). Uses the aws-cli container
      # against localhost:9000.  `mb` returns non-zero when the bucket
      # already exists; we check the message to distinguish that
      # benign case from a real failure.
      #
      # Credentials come from the env file uploaded above
      # (`env/_minio-admin.env`) rather than `-e KEY=${var...}` so that
      # `var.minio_*` does NOT appear in the inline= [...] string list.
      # Inlining a sensitive var would force terraform to mark every
      # line of remote-exec output for this resource as "(output
      # suppressed due to sensitive value in config)" -- an
      # all-or-nothing per-resource gate that would also hide unrelated
      # init / podman / sitegen lines.
      [for bucket in local.staging_bucket_names :
        "out=$(podman run --rm --network=host --env-file=${local.base_dir}/env/_minio-admin.env docker.io/amazon/aws-cli --endpoint-url http://localhost:9000 s3 mb s3://${bucket} --region us-east-1 2>&1) || { echo \"$out\" | grep -qE 'BucketAlreadyOwnedByYou|already exists' || { echo \"$out\" >&2; exit 1; }; }; true"
      ],
      # Reset specified instances.  The reset must FAIL LOUD: the only
      # way the volume rm can refuse is if a pond/sitegen container is
      # still using it (cf. site-prod 2026-05-02 silent reset failure
      # caused by a 90-min sitegen container surviving the global kill
      # earlier in this apply).  We therefore (1) disable the timer so
      # it cannot fire mid-apply, (2) stop the service, (3) kill any
      # container holding THIS volume, (4) rm the volume with no error
      # suppression so terraform aborts if the kill missed something.
      [for name in var.reset_instances :
        "${join(" && ", [
          "echo '[reset] ${name}: disabling timer'",
          "(systemctl --user disable --now pond@${name}.timer pond-selfmon@${name}.timer 2>/dev/null || true)",
          "echo '[reset] ${name}: stopping service'",
          "(systemctl --user stop pond@${name}.service pond-selfmon@${name}.service 2>/dev/null || true)",
          "echo '[reset] ${name}: killing any container holding ${local.instance_volumes[name]}'",
          "podman ps -aq --filter 'volume=${local.instance_volumes[name]}' | xargs -r podman rm -f",
          "echo '[reset] ${name}: removing volume ${local.instance_volumes[name]}'",
          "if podman volume exists ${local.instance_volumes[name]}; then podman volume rm ${local.instance_volumes[name]}; else echo '[reset] ${name}: no volume to remove'; fi",
          "echo '[reset] ${name}: removing host dir'",
          "rm -rf ${local.home}/pond-${name}",
          # Empty this instance's S3 backup bucket.  Post-D6 `pond backup
          # add` refuses a bucket whose store_id does not match the local
          # pond_id ("refusing to push into a foreign pond"); a reset
          # gives the local pond a NEW pond_id, so the stale old-format
          # pond left in the bucket would block the re-attach.  Emptying
          # the bucket makes `backup add` see "not a Delta table" and
          # re-initialize cleanly.  Instances with no bucket of their own
          # (site-*: s3_url == "") have nothing to empty.  The bucket
          # itself is (re)created by the `mb` step earlier in this apply.
          local.instances[name].s3_url != ""
          ? "echo '[reset] ${name}: emptying bucket ${local.instances[name].s3_url}' && podman run --rm --network=host --env-file=${local.base_dir}/env/_minio-admin.env docker.io/amazon/aws-cli --endpoint-url http://localhost:9000 s3 rm ${local.instances[name].s3_url} --recursive --region us-east-1"
          : "echo '[reset] ${name}: no S3 bucket to empty'",
          # Selfmon-only: also wipe the per-pond JSONL source dir and
          # the rendered HTML output dir.  Both are no-ops for
          # containerized data ponds (path won't exist), but for a
          # native selfmon reset they are required -- otherwise the
          # next ingest tick replays old JSONL rows whose schema may
          # not match what the current pond/sitegen code expects, and
          # Caddy keeps serving stale HTML files (e.g. orphan
          # status.html after a route rename) from prior runs.
          "echo '[reset] ${name}: wiping selfmon metrics source'",
          "rm -rf /var/log/watertown-selfmon/${name}",
          "echo '[reset] ${name}: wiping selfmon rendered output'",
          "rm -rf /var/www/selfmon/${name}",
          "echo '[reset] ${name}: done'",
        ])} || exit 1"
      ],
      # Refresh the container image once at deploy time.  pond.sh now uses
      # --pull=missing, so a terraform apply after a new image is promoted must
      # explicitly pull it; timer ticks then keep it current via --pull-image.
      [for name in local.container_instance_names :
        "${local.base_dir}/config/scripts/pond.sh ${name} --pull-image"
      ],
      # Initialize containerized instances if not already initialized.
      # `pond init` errors with "Pond already exists" on a populated pond;
      # we detect that condition on the host (via the volume's mountpoint)
      # and skip init in the no-op case.  Real init failures still surface.
      [for name in local.container_instance_names :
        "if podman volume exists ${local.instance_volumes[name]} && [ -d \"$(podman volume inspect ${local.instance_volumes[name]} --format '{{.Mountpoint}}')/data/_delta_log\" ]; then echo '[init] ${name}: already initialized'; else ${local.base_dir}/config/scripts/pond.sh ${name} init --birthplace ${name}; fi"
      ],
      # Apply containerized instance configs
      [for name in local.container_instance_names :
        "${local.base_dir}/config/scripts/pond.sh ${name} apply -f /config/${replace(replace(name, "-staging", ""), "-prod", "")}.yaml"
      ],
      # Initialize selfmon instances natively (POND comes from env file).
      # Same idempotent pattern as containerized instances above.
      [for name in local.selfmon_instance_names :
        "set -a; . ${local.base_dir}/env/${name}.env; set +a; if [ -d \"$POND/data/_delta_log\" ]; then echo '[init] ${name}: already initialized'; else /usr/bin/pond init --birthplace ${name}; fi"
      ],
      # Apply selfmon configs natively
      [for name in local.selfmon_instance_names :
        "set -a; . ${local.base_dir}/env/${name}.env; set +a; /usr/bin/pond apply -f ${local.base_dir}/config/${name}.yaml"
      ],
      # selfmon commits every minute, so bound the DATA _delta_log too (not just
      # control): 1 day of retention keeps the journal table from external-
      # sorting weeks of commit JSONs into the spill pool and OOMing.  selfmon
      # is fully pushed every tick, so its commit log is never needed for diffs.
      [for name in local.selfmon_instance_names :
        "set -a; . ${local.base_dir}/env/${name}.env; set +a; /usr/bin/pond config set maintenance.data_log_retention_minutes 1440"
      ],
      # (Re)attach S3 backup/import remotes, declaratively.  These were shell
      # invocations of `pond backup add` / `pond remote add` wrapped in
      # attach-remotes.sh; they are now `pond apply` documents like every other
      # piece of pond configuration, so the endpoint and credentials are
      # written once as a storage profile instead of being rebuilt as CLI flags
      # per attach.  Idempotent via `overwrite: true` in the documents.
      #
      # Applied here rather than folded into config/<type>.yaml because ORDER
      # MATTERS: a pull-mode remote refuses a bucket that is not yet an
      # initialized pond, so every producer (water/noyo/septic) must attach AND
      # push its pond_init bundle before the site pond imports it.  The
      # per-type configs are applied in one unordered pass above; these are
      # deliberately two ordered passes.
      #
      # Producers only -- selfmon is native and deliberately has no remote at
      # all (see watershop-selfmon above), so it appears in neither pass.
      [for name in local.container_instance_names :
        "raw_listing=$(${local.base_dir}/config/scripts/pond.sh ${name} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); unexpected=$(printf '%s\\n' \"$raw_names\" | awk '$1 != \"origin\" { print $1 }'); for remote in $unexpected; do echo \"[remote] ${name}: detaching unexpected $remote\"; ${local.base_dir}/config/scripts/pond.sh ${name} remote remove \"$remote\"; done; ${local.base_dir}/config/scripts/pond.sh ${name} apply -f /config/remotes/producer.yaml; ${local.base_dir}/config/scripts/pond.sh ${name} push origin; raw_listing=$(${local.base_dir}/config/scripts/pond.sh ${name} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); remotes=$(${local.base_dir}/config/scripts/pond.sh ${name} remote list) || exit 1; [ \"$raw_names\" = origin ] && printf '%s\\n' \"$remotes\" | awk -v url='${local.instances[name].s3_url}' 'NR == 1 { next } $1 == \"origin\" && $2 == url && $3 == \"push\" && $4 == \"-\" { found++; next } { unexpected=1 } END { exit !(found == 1 && !unexpected) }' || exit 1; echo '[remote] ${name}: MinIO origin converged'"
        if !startswith(name, "site-") && !endswith(name, "-prod")
      ],
      # Production producers publish exclusively to Azure. Raw attachment
      # inventory below removes the former MinIO origin, including a malformed
      # attachment that the parsed remote listing cannot read.
      [for name in local.container_instance_names :
        "raw_listing=$(${local.base_dir}/config/scripts/pond.sh ${name} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); unexpected=$(printf '%s\\n' \"$raw_names\" | awk '$1 != \"azure\" { print $1 }'); for remote in $unexpected; do echo \"[remote] ${name}: detaching unexpected $remote\"; ${local.base_dir}/config/scripts/pond.sh ${name} remote remove \"$remote\"; done; ${local.base_dir}/config/scripts/pond.sh ${name} apply -f /config/remotes/producer-azure.yaml; ${local.base_dir}/config/scripts/pond.sh ${name} push azure; raw_listing=$(${local.base_dir}/config/scripts/pond.sh ${name} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); remotes=$(${local.base_dir}/config/scripts/pond.sh ${name} remote list) || exit 1; [ \"$raw_names\" = azure ] && printf '%s\\n' \"$remotes\" | awk -v url='az://${name}-0002' 'NR == 1 { next } $1 == \"azure\" && $2 == url && $3 == \"push\" && $4 == \"-\" { found++; next } { unexpected=1 } END { exit !(found == 1 && !unexpected) }' || exit 1; echo '[remote] ${name}: Azure remote converged'"
        if !startswith(name, "site-") && endswith(name, "-prod")
      ],
      # A reset producer must complete its first source replay and seed without
      # ordinary burst limits before site-staging can import it. The override
      # is scoped to this one explicit reset run and is never persisted.
      [for name in local.container_instance_names :
        "POND_IGNORE_LIMITS=1 ${local.base_dir}/config/scripts/run.sh ${name}; POND_IGNORE_LIMITS=1 ${local.base_dir}/config/scripts/pond.sh ${name} push origin"
        if !startswith(name, "site-") && !endswith(name, "-prod") && contains(var.reset_instances, name)
      ],
      [for name in local.container_instance_names :
        "raw_listing=$(${local.base_dir}/config/scripts/pond.sh ${name} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); unexpected=$(printf '%s\\n' \"$raw_names\" | awk '$1 != \"water\" && $1 != \"noyo\" && $1 != \"septic\" { print $1 }'); for remote in $unexpected; do echo \"[remote] ${name}: detaching unexpected $remote\"; ${local.base_dir}/config/scripts/pond.sh ${name} remote remove \"$remote\"; done; ${local.base_dir}/config/scripts/pond.sh ${name} apply -f /config/remotes/site.yaml; raw_listing=$(${local.base_dir}/config/scripts/pond.sh ${name} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); remotes=$(${local.base_dir}/config/scripts/pond.sh ${name} remote list) || exit 1; printf '%s\\n' \"$raw_names\" | awk '$1 == \"water\" { water++; next } $1 == \"noyo\" { noyo++; next } $1 == \"septic\" { septic++; next } { unexpected=1 } END { exit !(water == 1 && noyo == 1 && septic == 1 && !unexpected) }' && printf '%s\\n' \"$remotes\" | awk 'NR == 1 { next } $1 == \"water\" && $2 == \"s3://water-staging-0002\" && $3 == \"pull\" && $4 == \"/sources/water\" { water++; next } $1 == \"noyo\" && $2 == \"s3://noyo-staging-0002\" && $3 == \"pull\" && $4 == \"/sources/noyo\" { noyo++; next } $1 == \"septic\" && $2 == \"s3://septic-staging-0002\" && $3 == \"pull\" && $4 == \"/sources/septic\" { septic++; next } { unexpected=1 } END { exit !(water == 1 && noyo == 1 && septic == 1 && !unexpected) }' || exit 1; echo '[remote] ${name}: MinIO imports converged'"
        if startswith(name, "site-") && !endswith(name, "-prod")
      ],
      [for name in local.container_instance_names :
        "raw_listing=$(${local.base_dir}/config/scripts/pond.sh ${name} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); unexpected=$(printf '%s\\n' \"$raw_names\" | awk '$1 != \"water\" && $1 != \"noyo\" && $1 != \"septic\" { print $1 }'); for remote in $unexpected; do echo \"[remote] ${name}: detaching unexpected $remote\"; ${local.base_dir}/config/scripts/pond.sh ${name} remote remove \"$remote\"; done; ${local.base_dir}/config/scripts/pond.sh ${name} apply -f /config/remotes/site-azure.yaml; raw_listing=$(${local.base_dir}/config/scripts/pond.sh ${name} list /sys/remotes/) || exit 1; raw_names=$(printf '%s\\n' \"$raw_listing\" | awk '{ name=$NF; sub(\"^.*/\", \"\", name); print name }'); remotes=$(${local.base_dir}/config/scripts/pond.sh ${name} remote list) || exit 1; printf '%s\\n' \"$raw_names\" | awk '$1 == \"water\" { water++; next } $1 == \"noyo\" { noyo++; next } $1 == \"septic\" { septic++; next } { unexpected=1 } END { exit !(water == 1 && noyo == 1 && septic == 1 && !unexpected) }' && printf '%s\\n' \"$remotes\" | awk 'NR == 1 { next } $1 == \"water\" && $2 == \"az://water-prod-0003\" && $3 == \"pull\" && $4 == \"/sources/water\" { water++; next } $1 == \"noyo\" && $2 == \"az://noyo-prod-0003\" && $3 == \"pull\" && $4 == \"/sources/noyo\" { noyo++; next } $1 == \"septic\" && $2 == \"az://septic-prod-0003\" && $3 == \"pull\" && $4 == \"/sources/septic\" { septic++; next } { unexpected=1 } END { exit !(water == 1 && noyo == 1 && septic == 1 && !unexpected) }' || exit 1; echo '[remote] ${name}: Azure imports converged'"
        if startswith(name, "site-") && lookup(local.instances[name], "remote_backend", "minio") == "azure"
      ],
      # A staging reset changes at least one source identity or removes the
      # consumer itself. Complete the full initial import and build under the
      # same explicit one-shot authorization used for producer seeding.
      local.staging_site_reseed
      ? ["POND_IGNORE_LIMITS=1 ${local.base_dir}/config/scripts/run.sh site-staging"]
      : [],
      # Enable + start producer and selfmon timers.  Each timer's OnBootSec
      # is already in the past, so starting a stopped timer fires its first
      # run immediately and then settles onto the OnUnitActiveSec cadence.
      # Reset staging producers already completed their synchronous seed
      # above; subsequent timer runs use normal limiter enforcement.
      [for name in local.container_instance_names :
        endswith(name, "-prod") && !var.activate_production_timers
        ? "systemctl --user disable --now pond@${name}.timer 2>/dev/null || true"
        : "systemctl --user enable --now pond@${name}.timer"
        if !startswith(name, "site-")
      ],
      [for name in local.selfmon_instance_names :
        "systemctl --user enable --now pond-selfmon@${name}.timer"
      ],
      # Hourly deb auto-update timer for each selfmon instance.  OnBootSec is
      # in the past, so enable --now fires a first update check immediately
      # and then settles onto the 1h cadence.
      [for name in local.selfmon_instance_names :
        "systemctl --user enable --now pond-selfmon-update@${name}.timer"
      ],
      # A reset site was built synchronously above. Enable its persistent timer
      # now, but defer activation for one normal 3h interval so OnBootSec does
      # not trigger an immediate duplicate build. On reboot, the enabled timer
      # starts normally even if the transient delay has not yet elapsed.
      [for name in local.container_instance_names :
        endswith(name, "-prod") && !var.activate_production_timers
        ? "systemctl --user disable --now pond@${name}.timer 2>/dev/null || true"
        : (name == "site-staging" && local.staging_site_reseed
          ? "systemctl --user enable pond@${name}.timer; systemctl --user stop pond-firstbuild-${name}.timer 2>/dev/null || true; systemd-run --user --on-active=3h --unit=pond-firstbuild-${name} systemctl --user start pond@${name}.timer"
        : "systemctl --user enable --now pond@${name}.timer")
        if startswith(name, "site-")
      ],
      [for name in ["site-staging", "site-prod"] :
        contains(var.weekly_report_email_instances, name) && (!endswith(name, "-prod") || var.activate_production_timers)
        ? "systemctl --user enable --now pond-email-report@${name}.timer"
        : "systemctl --user disable --now pond-email-report@${name}.timer 2>/dev/null || true"
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
