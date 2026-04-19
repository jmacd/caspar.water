# Caspar Water Operations

## Pond Configs

Four canonical configs in `config/`, one per pond type:

| File | Pond | What it does |
|------|------|-------------|
| `config/water.yaml` | Water | Dirs, logfile-ingest, backup, temporal-reduce, analysis |
| `config/noyo.yaml` | Noyo | Dirs, git-ingest (site content), laketech archive copy, backup, hydrovu, column-rename, combine/single/reduce, sitegen |
| `config/septic.yaml` | Septic | Dirs, logfile-ingest, backup, temporal-reduce |
| `config/site.yaml` | Site | Dirs, git-ingest (site content), cross-pond imports (water/noyo/septic), sitegen |

Each uses `${env:VAR}` for deployment-specific values (S3 credentials, data paths, git ref).

## Site Content

Site content (markdown pages, templates, images) lives in the git repo and is
pulled into ponds via the `git-ingest` factory.  No host-copy or file-push needed.

| Repo path | Used by | Pond destination |
|-----------|---------|-----------------|
| `site/content/` | Site pond | `/content/` |
| `site/templates/` | Site pond | `/templates/` |
| `site/img/` | Site pond | `/img/` |
| `config/noyo/site/` | Noyo pond | `/system/site/` |

The git-ingest `prefix` field filters the repo tree so each pond only syncs its
relevant subtree.  Staging ponds track the configured `GIT_REF` branch;
production ponds always track `main`.

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
| `NOYO_ARCHIVE_DIR` | Noyo laketech copy | Host path to archive |
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

### Instances

| Instance | Type | Timer interval | S3 bucket |
|----------|------|---------------|-----------|
| water-staging | water | 10min | s3://water-staging |
| noyo-staging | noyo | 30min | s3://noyo-staging |
| septic-staging | septic | 10min | s3://septic-staging |
| site-staging | site | 15min | (no backup) |
| water-prod | water | 10min | s3://water-pond |
| noyo-prod | noyo | 30min | s3://noyo-pond |
| septic-prod | septic | 10min | s3://septic-pond |
