# Caspar Water Operations

## Pond Configs

Four canonical configs in `config/`, one per pond type:

| File | Pond | What it does |
|------|------|-------------|
| `config/water.yaml` | Water | Dirs, site content copies, logfile-ingest, backup, temporal-reduce, analysis, sitegen |
| `config/noyo.yaml` | Noyo | Dirs, laketech archive copy, site copy, backup, hydrovu, column-rename, combine/single/reduce, sitegen |
| `config/septic.yaml` | Septic | Dirs, logfile-ingest, backup, temporal-reduce, sitegen |
| `config/site.yaml` | Site | Dirs, site content copies, cross-pond imports (water/noyo/septic), sitegen |

Each uses `${env:VAR}` for deployment-specific values (S3 credentials, data paths, site paths).

## Shared Scripts

| File | Purpose |
|------|---------|
| `config/scripts/pond.sh` | Podman wrapper — runs `pond` in a container with the right volumes, env, and image |
| `config/scripts/run.sh` | Systemd timer entrypoint — dispatches by pond type (ingest/collect/pull/sitegen) |
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
| `SITE_DIR` | Site content copies | `/site` (container) or repo path (local) |
| `SITE_BASE_URL` | Sitegen base URL | `/` or `/noyo-harbor/` |
| `DATA_DIR` | Water/septic ingest | Host path to NFS data, mounted at `/data` |
| `NOYO_ARCHIVE_DIR` | Noyo laketech copy | Host path to archive |
| `NOYO_SITE_DIR` | Noyo site pages copy | `/config/noyo/site` (container) |
| `HYDRO_KEY_ID` | HydroVu API | OAuth client ID |
| `HYDRO_KEY_VALUE` | HydroVu API | OAuth client secret |
| `WATER_S3_URL` | Site imports | `s3://water-staging` |
| `NOYO_S3_URL` | Site imports | `s3://noyo-staging` |
| `SEPTIC_S3_URL` | Site imports | `s3://septic-staging` |

## Local Development

```bash
cd local
./setup.sh          # pond init + pond apply -f config/site.yaml
./sync.sh           # pull data from staging MinIO
./generate.sh       # run sitegen
./serve.sh          # serve locally
```

Env vars come from `local/env.sh` (MinIO on watershop, staging buckets).

## Watershop Staging

### Deploy / Update configs

```bash
cd terraform/station/watershop
terraform apply                    # staging only (default)
terraform apply -var deploy_production=true   # + production
```

Terraform pushes `config/`, `site/`, env files, timer files to the machine.
For each instance: `pond init` (no-op if exists) + `pond apply -f /config/<type>.yaml`.

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
