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
| `config/scripts/reset.sh` | Erases a staging S3 bucket for an instance — reads credentials from an env file |
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
| `AZURE_STORAGE_ACCOUNT` | Production storage | Azure storage account name |
| `AZURE_TENANT_ID` | Production storage | Service-principal tenant |
| `AZURE_CLIENT_ID` | Production storage | Per-instance writer or reader identity |
| `AZURE_CLIENT_SECRET` | Production storage | Per-instance credential |
| `AZURE_URL` | Production producer backup | `az://water-prod-0003` |
| `WATER_AZURE_URL` | Production site import | `az://water-prod-0003` |
| `NOYO_AZURE_URL` | Production site import | `az://noyo-prod-0003` |
| `SEPTIC_AZURE_URL` | Production site import | `az://septic-prod-0003` |

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

## Watershop deployment

Staging uses MinIO on watershop. Production uses Azure native-v2 publication
with a separate service principal for each producer and a read-only identity
for `site-prod`.

### Production state (2026-09-12)

Production is running Watertown `0.165.228` from
`ghcr.io/jmacd/watertown/watertown:prod-arm64`, digest
`sha256:f13fe24accc0b9a5cb900f4ae45fcd43944351aee9a92f5c52844d83912e3df9`.

| Instance | Local generation | Azure container | Timer |
|---|---|---|---|
| `water-prod` | `pond-water-prod-0003` | `water-prod-0003` | 1h, enabled |
| `noyo-prod` | `pond-noyo-prod-0003` | `noyo-prod-0003` | 1h, enabled |
| `septic-prod` | `pond-septic-prod-0003` | `septic-prod-0003` | 1h, enabled |
| `site-prod` | `pond-site-prod-0003` | read-only imports of all three containers | 3h, enabled |

The native-v2 cutover, normal-limit producer/site canaries, and the first
timer-triggered cycle all completed successfully. The active site build after
timer activation was `build-20260912-234140`; it was atomically installed on
watershop and the cloud host. The public root and `/noyo-harbor/` both returned
HTTP 200.

Legacy `-0002` Azure containers and the prior local volumes remain rollback
state. Do not delete or overwrite them as part of routine deployment, and do
not infer that changing a Terraform generation name authorizes their removal.

### Weekly email report

`sitegen` defines the transport-independent `weekly` report in
`config/site.yaml`. The separate `/system/etc/95-email-report` factory sends
that report through Azure Communication Services Email; its endpoint, access
key, and recipient exist only in each deployed instance's mode-0600 env file.

Provision the verified `casparwater.us` sender domain, its `reports` sender
username, and its Linode DNS records:

```bash
cd terraform/station/email
cp terraform.tfvars.example terraform.tfvars
# Set the private Linode token, then:
terraform apply
./verify-domain.sh
```

After Azure reports Domain, SPF, DKIM, and DKIM2 as verified, copy
`email_endpoint` and the sensitive `email_access_key` output into the ignored
Watershop tfvars. Enable staging first:

```hcl
weekly_report_email_instances = ["site-staging"]
weekly_report_email_credentials = {
  endpoint   = "https://..."
  access_key = "..."
  recipient  = "..."
}
```

Apply with production excluded, then send one staging smoke test:

```bash
cd terraform/station/watershop
terraform apply -var deploy_production=false
ssh watershop.casparwater.us \
  '~/watertown/config/scripts/run-email-report.sh site-staging'
```

The enabled instance sends every Monday at 09:00 America/Los_Angeles. Add
`site-prod` only after the staging message is received and the corresponding
Watertown image has been promoted.

### Deploy / Update configs

```bash
cd terraform/station/watershop
terraform apply                    # staging only (default)
terraform apply -var deploy_production=true   # + production
terraform apply -var git_ref=my-branch        # Caspar staging with custom branch
terraform apply -var noyo_git_ref=my-branch   # Noyo staging with custom branch
```

Terraform pushes `config/` and env files to the machine.
For each instance: `pond init` (no-op if exists) + `pond apply -f /config/<type>.yaml`.
Site content is pulled from git at runtime by `run.sh` — no file push needed.

`activate_production_timers` defaults to `false`. A production apply therefore
leaves all `-prod` timers disabled until an operator completes the seed and
normal-limit canary below. Set it to `true` only after those checks pass.

### Selfmon I/O diagnostics

`run-selfmon.sh` emits a structured `selfmon_io` line for each wrapped step and
one aggregate line when the tick exits:

```text
selfmon_io scope=tick instance=watershop-selfmon exit_rc=0 failures=0 rchar=... wchar=... syscr=... syscw=... read_bytes=... write_bytes=... cancelled_write_bytes=...
```

These are per-step or per-tick deltas from `/proc/$$/io`; completed child
processes are included in the parent shell's counters. `rchar` and `wchar`
measure bytes passed through read/write syscalls, including cache hits and
pipes. `read_bytes` and `write_bytes` measure bytes that reached block storage.
`syscr` and `syscw` count read/write syscalls. This requires no systemd
`IOAccounting` or journald configuration.

To compare aggregate ticks:

```bash
journalctl --user -u "pond-selfmon@watershop-selfmon.service" --no-pager \
  | grep "selfmon_io scope=tick"
```

Efficiency is evaluated by comparing these counters at similar input volumes
after retention has reached steady state. Tick bytes, syscall counts, and
runtime should plateau rather than rise with transaction history.

### Reset an instance

```bash
# 1. Erase S3 bucket
config/scripts/reset.sh terraform/station/watershop/env/water-staging.env

# 2. Wipe volume + re-init (terraform)
cd terraform/station/watershop
terraform apply -var 'reset_instances=["water-staging"]'
```

### When a reset is the only repair

A pond has no `rm`: `pond` exposes no delete/truncate for a file it has already
committed (`emergency` offers only `erase-bucket`). `logfile-ingest` likewise has
no re-baseline subcommand — only `b3sum`, `push`, and `pull`. So for the failures
below there is no in-place fix, and `reset_instances` above is the only lever.
Reach for it directly instead of trying to repair pond state by hand.

**`logfile-ingest` prefix verification failure.** The tick logs, per source file:

```
Active file <name>.jsonl grew from <N> to <M> bytes but prefix changed
  (host root <hash>) - checking for rotation
ERROR Factory 'logfile-ingest' execution error: ... Prefix verification failed
  for <name>.jsonl: expected blake3=<a>, got blake3=<b>.
  File may have been rotated during ingestion.
Error: Transaction aborted: Execution failed for factory 'logfile-ingest'
```

This means the host file was rotated/replaced without matching the mknod's
`archived_pattern`, so the pond holds a stale pre-rotation segment that is not a
prefix of the current host file. The guard is correct — it refuses to splice
unrelated data onto the old segment — but the abort is permanent until the state
is reset. Confirm before resetting, by comparing the two directly:

```bash
pond cat /measure/<name>.jsonl > /tmp/pondcopy
cmp /tmp/pondcopy /var/log/watertown-selfmon/<instance>/<name>.jsonl
# diverging at byte ~1 (not at the recorded size) == disjoint, needs a reset
```

Note the selfmon reset also wipes `/var/log/watertown-selfmon/<instance>`, so the
host-side JSONL history is discarded too and ingest genuinely restarts empty.
That is deliberate: replaying old rows can reintroduce a stale schema.

**A stalled ingest can hide for weeks.** This failure only surfaces once the
relevant `pond run .../measure/<name> push` step actually runs. If a config change
enables ingest steps that were previously dormant, the first tick can surface a
divergence dating back months. Check the *first* occurrence before assuming a
recent deploy caused it:

```bash
journalctl --user -u "pond-selfmon@<instance>.service" --no-pager \
  | grep "Prefix verification failed" | head -2
```

### Production generation cutover

Do not use `reset_instances` for production. Terraform rejects production
reset targets because an in-place reset destroys rollback state. A production
repair that requires a new pond identity must instead use a new numeric
generation for all four local volumes and all three Azure containers.

1. Provision the new Azure containers and identities, configure the new
   `-NNNN` volume/container names, and apply with production timers disabled.
2. Pull the promoted immutable build:

   ```bash
   ~/watertown/config/scripts/pond.sh water-prod --pull-image
   ```

3. Seed each fresh producer explicitly. The initial snapshot is expected to
   exceed the ordinary 64 MiB burst, so the override is scoped to this command
   and is never placed in an env file:

   ```bash
   POND_IGNORE_LIMITS=1 ~/watertown/config/scripts/run.sh water-prod
   POND_IGNORE_LIMITS=1 ~/watertown/config/scripts/run.sh septic-prod
   POND_IGNORE_LIMITS=1 ~/watertown/config/scripts/run.sh noyo-prod
   ```

   If source ingestion already committed locally but publication failed,
   retry only the pending publication:

   ```bash
   POND_IGNORE_LIMITS=1 ~/watertown/config/scripts/pond.sh water-prod push azure
   ```

4. Seed the fresh site imports and atomically deploy the result:

   ```bash
   POND_IGNORE_LIMITS=1 ~/watertown/config/scripts/run.sh site-prod
   ```

5. Run every producer and the site once without an override. Confirm changed
   work remains below its normal limiter, unchanged pushes are acknowledged
   no-ops, site pulls are incremental, and the cloud symlink/public endpoints
   are current.
6. Enable timers through Terraform with
   `-var activate_production_timers=true`. Verify the immediately triggered
   cycle finishes successfully and each timer has its next scheduled
   activation.

The initial override authorizes only full seed transfer. It is not evidence
for normal efficiency; the subsequent no-override producer and consumer cycle
is the release gate.

### Instances

| Instance | Type | Timer interval | Remote |
|----------|------|---------------|--------|
| water-staging | water | 1h | `s3://water-staging-0002` |
| noyo-staging | noyo | 1h | `s3://noyo-staging-0002` |
| septic-staging | septic | 1h | `s3://septic-staging-0002` |
| site-staging | site | 3h | pulls the three staging MinIO buckets |
| water-prod | water | 1h | `az://water-prod-0003` |
| noyo-prod | noyo | 1h | `az://noyo-prod-0003` |
| septic-prod | septic | 1h | `az://septic-prod-0003` |
| site-prod | site | 3h | read-only pulls of the three production Azure containers |

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
