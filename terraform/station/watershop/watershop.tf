locals {
  home     = "/home/${var.user}"
  base_dir = "${local.home}/watertown"

  # All instances currently use MinIO on watershop.  Production previously
  # used Cloudflare R2 (see prod_s3 below), but we are running prod against
  # MinIO too while we continue to harden the remote backup feature.  Prod
  # and staging stay isolated by bucket name.
  staging_s3 = {
    endpoint   = var.minio_endpoint
    region     = "us-east-1"
    access_key = var.minio_access_key
    secret_key = var.minio_secret_key
    allow_http = "true"
  }
  # Reserved for future re-enable of R2-backed production.  Currently unused.
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
      interval   = "1h"
      boot_delay = "5min"
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/\nGIT_REF=${var.git_ref}"
    }
    noyo-prod = {
      s3         = local.staging_s3
      s3_url     = "s3://noyo-pond"
      interval   = "1h"
      boot_delay = "6min"
      extra_env  = "HYDRO_KEY_ID=${var.hydrovu_key_id}\nHYDRO_KEY_VALUE=${var.hydrovu_key_value}\nSITE_BASE_URL=/noyo-harbor/"
    }
    water-staging = {
      s3         = local.staging_s3
      s3_url     = "s3://water-staging"
      interval   = "1h"
      boot_delay = "2min"
      extra_env  = "DATA_DIR=${var.water_data_dir}\nSITE_BASE_URL=/"
    }
    water-prod = {
      s3         = local.staging_s3
      s3_url     = "s3://water-pond"
      interval   = "1h"
      boot_delay = "3min"
      extra_env  = "DATA_DIR=${var.water_data_dir}\nSITE_BASE_URL=/"
    }
    septic-staging = {
      s3         = local.staging_s3
      s3_url     = "s3://septic-staging"
      interval   = "1h"
      boot_delay = "4min"
      extra_env  = "DATA_DIR=${var.septic_data_dir}\nSITE_BASE_URL=/"
    }
    septic-prod = {
      s3         = local.staging_s3
      s3_url     = "s3://septic-pond"
      interval   = "1h"
      boot_delay = "5min"
      extra_env  = "DATA_DIR=${var.septic_data_dir}\nSITE_BASE_URL=/"
    }
    site-staging = {
      s3         = local.staging_s3
      s3_url     = ""
      interval   = "3h"
      boot_delay = "7min"
      extra_env  = "WATER_S3_URL=s3://water-staging\nNOYO_S3_URL=s3://noyo-staging\nSEPTIC_S3_URL=s3://septic-staging\nSITE_BASE_URL=/\nGIT_REF=${var.git_ref}"
    }
    site-prod = {
      s3         = local.staging_s3
      s3_url     = ""
      interval   = "3h"
      boot_delay = "8min"
      extra_env  = "WATER_S3_URL=s3://water-pond\nNOYO_S3_URL=s3://noyo-pond\nSEPTIC_S3_URL=s3://septic-pond\nSITE_BASE_URL=/\nCLOUD_HOST=cloud"
    }
    watershop-selfmon = {
      s3 = local.staging_s3
      # s3_url provisions a MinIO bucket and the S3_* env used to resolve
      # credentials, but attach-remotes.sh intentionally does NOT attach a
      # backup remote for selfmon: run-selfmon.sh prunes aggressively with
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
      lookup(local.instances[name], "selfmon", false) ||
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

  # MinIO buckets to ensure exist.  All instances now use MinIO; the only
  # ones that need a bucket are those with a non-empty s3_url (the site-*
  # instances aggregate from other ponds and have no bucket of their own).
  staging_bucket_names = [for n in local.instance_names :
    replace(local.instances[n].s3_url, "s3://", "")
    if local.instances[n].s3_url != ""
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
    "SELFMON_METRICS_DIR=/var/log/watertown-selfmon/${each.key}",
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
        # Selfmon deb auto-update units (native): hourly oras pull + dpkg -i
        # of the newest pond .deb OCI artifact.  Template units activated
        # per selfmon instance below.
        "cp ${local.base_dir}/config/systemd/pond-selfmon-update@.service ${local.home}/.config/systemd/user/",
        "cp ${local.base_dir}/config/systemd/pond-selfmon-update@.timer ${local.home}/.config/systemd/user/",
        # Drop ONLY units whose instance name is no longer in the
        # configured set (e.g. retired watershop-selfmon-{staging,prod}
        # split).  Configured-but-not-deployed instances (e.g. -prod
        # when deploy_production=false) are left alone -- toggling a
        # deploy flag must not disturb the other tier.
        "${local.base_dir}/config/scripts/cleanup-stale-pond-units.sh ${join(" ", local.all_configured_names)}",
        # Reap any leaked pond containers belonging to instances we
        # are about to (re)deploy in this apply.  `podman run --rm`
        # detaches from systemd, so a `systemctl stop` on the .service
        # does NOT kill the running container (cf. cloud Apr 30
        # bandwidth bleed).  Match by image AND volume so we don't
        # disturb containers for instances we're leaving alone.
        join(" ; ", concat(
          [for name in local.container_instance_names :
            "podman ps --format '{{.Names}}' --filter 'volume=pond-${name}' --filter 'ancestor=ghcr.io/jmacd/watertown/watertown' | xargs -r podman kill 2>/dev/null || true"
          ],
          [for name in local.container_instance_names :
            "podman ps -aq --filter 'volume=pond-${name}' --filter 'ancestor=ghcr.io/jmacd/watertown/watertown' | xargs -r podman rm -f 2>/dev/null || true"
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
      # Install the watertown .deb (built natively on watershop by
      # tools/build-on-watershop.sh).  Always installs the newest .deb
      # in target/debian/; selfmon is local-experimental, no version
      # pinning.  Skipped if there is no selfmon instance to run.
      # `|| exit 1` because remote-exec runs the whole inline list as one
      # shell WITHOUT `set -e`; without it a failed deb install (the
      # script exits non-zero) is masked by later commands and the apply
      # wrongly reports success.
      length(local.selfmon_instance_names) > 0
      ? ["${local.base_dir}/config/scripts/install-watertown.sh || exit 1"]
      : [],
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
      # (staging + prod, plus selfmon).  Uses the aws-cli container
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
        join(" && ", [
          "echo '[reset] ${name}: disabling timer'",
          "(systemctl --user disable --now pond@${name}.timer pond-selfmon@${name}.timer 2>/dev/null || true)",
          "echo '[reset] ${name}: stopping service'",
          "(systemctl --user stop pond@${name}.service pond-selfmon@${name}.service 2>/dev/null || true)",
          "echo '[reset] ${name}: killing any container holding pond-${name}'",
          "podman ps -aq --filter 'volume=pond-${name}' | xargs -r podman rm -f",
          "echo '[reset] ${name}: removing volume pond-${name}'",
          "if podman volume exists pond-${name}; then podman volume rm pond-${name}; else echo '[reset] ${name}: no volume to remove'; fi",
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
          ? "echo '[reset] ${name}: emptying bucket ${local.instances[name].s3_url}' && (podman run --rm --network=host --env-file=${local.base_dir}/env/_minio-admin.env docker.io/amazon/aws-cli --endpoint-url http://localhost:9000 s3 rm ${local.instances[name].s3_url} --recursive --region us-east-1 2>&1 || true)"
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
        ])
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
        "if podman volume exists pond-${name} && [ -d \"$(podman volume inspect pond-${name} --format '{{.Mountpoint}}')/data/_delta_log\" ]; then echo '[init] ${name}: already initialized'; else ${local.base_dir}/config/scripts/pond.sh ${name} init --birthplace ${name}; fi"
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
      # (Re)attach S3 backup/import remotes.  Post-D6 watertown removed the
      # `remote` factory; backups and cross-pond imports are now CLI
      # attachments (`pond backup add` / `pond remote add`).  Idempotent
      # via --overwrite, so this runs on every apply.  attach-remotes.sh
      # branches on instance type (producer backup vs site import) and on
      # container-vs-native (selfmon) internally.
      #
      # ORDER MATTERS: a pull-mode `remote add` refuses a bucket that is
      # not yet an initialized pond, so every producer (water/noyo/septic)
      # must attach AND push its pond_init bundle before the site pond
      # imports it.  Attach producers + selfmon first, the site last.
      [for name in local.instance_names :
        "${local.base_dir}/config/scripts/attach-remotes.sh ${name}"
        if !startswith(name, "site-")
      ],
      [for name in local.instance_names :
        "${local.base_dir}/config/scripts/attach-remotes.sh ${name}"
        if startswith(name, "site-")
      ],
      # Enable + start producer and selfmon timers.  Each timer's OnBootSec
      # is already in the past, so starting a stopped timer fires its first
      # run immediately and then settles onto the OnUnitActiveSec cadence.
      # After a reset the timer was stopped, so this kicks the first ingest
      # right away; on a routine apply the timer is already running and the
      # enable --now is a no-op.  There is no synchronous seed: producers
      # populate their buckets asynchronously, which is the same work the
      # timer does every cycle.  terraform does not wait on any of it.
      [for name in local.container_instance_names :
        "systemctl --user enable --now pond@${name}.timer"
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
      # Site timers.  Starting a stopped site timer fires an immediate build.
      # Right after a reset the producers have pushed only their empty
      # pond_init bundle and have not ingested yet, so an immediate build
      # would race them, fail "table 'source' not found", and then wait a
      # full 3h interval.  For a site that was just reset, enable the timer
      # but defer its first start by 5 minutes via a transient systemd timer
      # so the producers kicked above have populated their buckets first.
      # This is fire-and-forget: terraform schedules the deferred start and
      # returns.  Sites not in the reset set already hold data and start
      # immediately.
      [for name in local.container_instance_names :
        (contains(var.reset_instances, name)
          ? "systemctl --user enable pond@${name}.timer; systemctl --user stop pond-firstbuild-${name}.timer 2>/dev/null || true; systemd-run --user --on-active=5min --unit=pond-firstbuild-${name} systemctl --user start pond@${name}.timer"
        : "systemctl --user enable --now pond@${name}.timer")
        if startswith(name, "site-")
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
