# Caspar Water Operations

## Pond Configs

Four canonical configs in `config/`, one per pond type:

| File | Pond | What it does |
|------|------|-------------|
| `config/water.yaml` | Water | Dirs, logfile-ingest, backup, temporal-reduce, analysis |
| `config/noyo.yaml` | Noyo | Dirs, git-ingest (noyo site templates + laketech data), backup, hydrovu, column-rename, combine/single/reduce, sitegen |
| `config/septic.yaml` | Septic | Dirs, logfile-ingest, backup, temporal-reduce |
| `config/site.yaml` | Site | Dirs, git-ingest (content, templates, images), cross-pond imports (water/noyo/septic), sitegen |

Each uses `${env:VAR}` for deployment-specific values (S3 credentials, data paths, git ref).

## Site Content

Site content (markdown pages, templates, images) lives in the git repo and is
pulled into ponds via the `git-ingest` factory.  Each git-ingest mknod is a
dynamic directory — content is served directly from git objects after a
`pond run <path> pull` fetches the repo.  No host-copy or file-push needed.

| Repo path | Used by | Pond mknod path | git-ingest prefix |
|-----------|---------|-----------------|-------------------|
| `site/content/` | Site pond | `/content` | `site/content` |
| `site/templates/` | Site pond | `/templates` | `site/templates` |
| `site/img/` | Site pond | `/img` | `site/img` |
| `config/noyo/site/` | Noyo pond | `/system/site` | `config/noyo/site` |

Laketech archive data (HydroVu HTML exports) is pulled from a separate repo:

| Repo | Pond mknod path | git-ingest prefix |
|------|-----------------|-------------------|
| `jmacd/noyo-blue-econ` | `/laketech/data` | `laketech` |

Each mknod has its own bare repo cache (`{pond}/git/{node-id}.git`), so each
must be pulled individually.  The `prefix` field filters the repo tree so
only the relevant subtree is served.  Staging ponds track the configured
`GIT_REF` branch; production ponds always track `main`.

The noyo pond's sitegen config at `/system/etc/90-sitegen` is used by the
site pond's `subsites:` directive — the site pond imports the full noyo tree
and builds the Noyo Harbor subsite from it.

## Shared Scripts

| File | Purpose |
|------|---------|
| `config/scripts/pond.sh` | Podman wrapper — runs `pond` in a container with the right volumes, env, and image |
| `config/scripts/run.sh` | Systemd timer entrypoint — dispatches by pond type (git-pull/ingest/collect/pull/sitegen) |
| `config/scripts/reset.sh` | Erases S3 bucket for an instance — reads credentials from an env file |
| `config/systemd/pond@.service` | Systemd template unit — runs `run.sh %i` for each instance |

## Environment Variables

Used by `${env:VAR}` in configs. Set in env files (terraform-generated) or `local/env.sh`.

| Variable | Used by | Example |
|----------|---------|---------|
| `S3_ENDPOINT` | All backup/import | `http://watershop.casparwater.us:9000` |
| `S3_REGION` | All backup/import | `us-east-1` |
| `S3_ACCESS_KEY` | All backup/import | MinIO or R2 key |
| `S3_SECRET_KEY` | All backup/import | MinIO or R2 secret |
| `S3_ALLOW_HTTP` | All backup/import | `true` for MinIO |
| `S3_URL` | Backup push URL | `s3://water-staging` |
| `GIT_REF` | Git-ingest ref | `main` or branch name |
| `SITE_BASE_URL` | Sitegen base URL | `/` or `/noyo-harbor/` |
| `DATA_DIR` | Water/septic ingest | Host path to NFS data, mounted at `/data` |
| `HYDRO_KEY_ID` | HydroVu API | OAuth client ID |
| `HYDRO_KEY_VALUE` | HydroVu API | OAuth client secret |
| `WATER_S3_URL` | Site imports | `s3://water-staging` |
| `NOYO_S3_URL` | Site imports | `s3://noyo-staging` |
| `SEPTIC_S3_URL` | Site imports | `s3://septic-staging` |

## Local Development

```bash
cd local
./setup.sh          # pond init + pond apply -f config/site.yaml
./sync.sh           # pull content from git + data from staging MinIO
./generate.sh       # run sitegen
./serve.sh          # serve locally
./refresh.sh        # quick content iteration (git pull + rebuild)
```

Env vars come from `local/env.sh` (MinIO on watershop, staging buckets, GIT_REF from current branch).
Note: `refresh.sh` only sees committed changes (git-ingest reads from the repo).

## Watershop Staging

### Deploy / Update configs

```bash
cd terraform/station/watershop
terraform apply                    # staging only (default)
terraform apply -var deploy_production=true   # + production
terraform apply -var git_ref=my-branch        # staging with custom branch
```

Terraform pushes `config/` and env files to the machine.
For each instance: `pond init` (no-op if exists) + `pond apply -f /config/<type>.yaml`.
Site content is pulled from git at runtime by `run.sh` — no file push needed.

### Reset an instance

```bash
# 1. Erase S3 bucket
config/scripts/reset.sh terraform/station/watershop/env/water-staging.env

# 2. Wipe volume + re-init (terraform)
cd terraform/station/watershop
terraform apply -var 'reset_instances=["water-staging"]'
```

### Full clean reset (recover from stale data)

Site-* instances aggregate from the producer ponds (water/noyo/septic) by
importing each producer's S3 bucket. The cross-pond import is isolated per
`pond_id` with a per-remote watermark (duckpond post-D6 remote model, #80), so
a slow producer is never masked by a faster one during normal operation.

Reset all four prod ponds together. A reset gives each producer a new `pond_id`
and restarts its `txn_seq` at 1; wiping site-prod alongside them clears its
import state so it re-bootstraps cleanly from each producer's fresh bucket
(testsuite `542-import-watermark-restore.sh`).

```bash
# 1. Erase prod producer S3 buckets so each re-inits into an empty bucket.
#    A reset gives the pond a new pond_id, and `pond backup add` refuses a
#    bucket whose store_id still belongs to the old pond.
config/scripts/reset.sh \
    terraform/station/watershop/env/water-prod.env \
    terraform/station/watershop/env/noyo-prod.env \
    terraform/station/watershop/env/septic-prod.env

# 2. Wipe all four prod volumes and re-init (terraform).
cd terraform/station/watershop
terraform apply -var deploy_production=true \
    -var 'reset_instances=["water-prod","noyo-prod","septic-prod","site-prod"]'

# 3. Verify: all four prod timers active and first ticks outcome=ok.
ssh watershop.casparwater.us 'systemctl --user list-timers "pond@*-prod*" --all'
```

The manual fix-up steps this runbook used to list are no longer needed:

- `pond apply` for site-prod runs unconditionally on every terraform apply
  (`watershop.tf`), and the reset path seeds each producer's ingest+push and
  builds the site once before any timer is enabled (commit `039e013`), so
  site-prod comes up with factories and a built site without a manual apply or
  an early manual trigger.
- noyo-prod's first hydrovu collect no longer crawls HydroVu from epoch on a
  fresh pond: live collection resumes from the git-ingested seed archives, and
  a missing resume point now hard-fails with a clear message instead of issuing
  the unbounded `startTime=0` query that timed the API out. This requires the
  prod image to include duckpond jmacd/61 (or later).

Total time to fresh data on cloud: ~10 min (bucket wipe + terraform) +
~1 min producer first ingest + ~10 min site-prod pull/build/rsync.

The same recipe works for staging: substitute `-staging` for `-prod`
throughout, and drop `-var deploy_production=true`.

### Instances

| Instance | Type | Timer interval | S3 bucket |
|----------|------|---------------|-----------|
| water-staging | water | 1h | s3://water-staging |
| noyo-staging | noyo | 1h | s3://noyo-staging |
| septic-staging | septic | 1h | s3://septic-staging |
| site-staging | site | 3h | (no backup) |
| water-prod | water | 1h | s3://water-pond |
| noyo-prod | noyo | 1h | s3://noyo-pond |
| septic-prod | septic | 1h | s3://septic-pond |
| site-prod | site | 3h | (no backup) |

### Diagnostics

Check timer & service status

```
# Overview of all pond timers and services
 systemctl --user list-timers 'pond@*'
 systemctl --user status 'pond@water-staging' 'pond@noyo-staging' 'pond@septic-staging' 'pond@site-staging'
```

Check recent logs per pond:

```
 # All pond activity, most recent first
 journalctl --user -u 'pond@*' --no-pager -n 50

 # One specific pond (e.g. water-staging)
 journalctl --user -u 'pond@water-staging' --no-pager -n 30

 # Errors only across all ponds
 journalctl --user -u 'pond@*' -p err --no-pager -n 50
```

Check if timers are firing:

```
 # Shows last trigger time and next scheduled run
 systemctl --user list-timers 'pond@*-staging*'
```

Check containers ran successfully:

```
 # Recent podman container exits (they use --rm so only failures may linger)
 podman ps -a --filter 'status=exited' --format '{{.Names}} {{.Status}}'
```

Deeper dive:

```
 # Follow logs live for a specific pond
 journalctl --user -u 'pond@site-staging' -f

 # Logs since last reboot/reset
 journalctl --user -u 'pond@water-staging' --no-pager -b
```
