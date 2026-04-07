# Watershop Staging — Session Context

## What We're Doing

Transitioning the Caspar Water infrastructure so that:

1. **Gateway** (linux.local) runs two duckpond instances: water logfile ingest + noyo HydroVu collection, both pushing to R2
2. **Cloud** (Linode) runs cross-pond sitegen only: imports both datasets from R2, generates a combined site with water at `/` and noyo at `/noyo-harbor/`
3. **Staging** (watershop) validates the full pipeline end-to-end before production, using MinIO instead of R2

## What's Been Done (on Mac)

All code is committed on branch `jmacd/seventeen`:

- `./site/` — water site content, templates, images, configs (moved from `duckpond/water/`)
- `./terraform/station/staging/` — staging configs + terraform for watershop
- `./terraform/station/gateway/` — gateway terraform with duckpond water+noyo
- `./terraform/station/cloud/` — cloud terraform with cross-pond sitegen
- `./README.md` — updated architecture docs

## What Needs to Happen Now (on Watershop)

The staging environment runs three duckpond ponds on watershop using podman containers and MinIO for backup/import.

### Prerequisites on Watershop

- podman installed and working
- MinIO running at localhost:9000 (alias "local" in mc)
- Two MinIO buckets pre-created: `water-staging` and `noyo-staging`
- HydroVu credentials in `~/.bashrc.private` (HYDRO_KEY_ID, HYDRO_KEY_VALUE)
- Water logfiles available at `/home/data/casparwater*.json`
- Container image: `ghcr.io/jmacd/duckpond/duckpond:latest-arm64`

### Option A: Deploy from Mac via Terraform

```bash
# On Mac:
cd terraform/station/staging
tofu init
tofu apply
# This pushes configs to watershop and runs setup via SSH
```

Then SSH in to run the pipeline:
```bash
ssh jmacd@watershop.casparwater.us
cd staging
./run-all.sh
cd dist && python3 -m http.server 4180
```

### Option B: Run Directly on Watershop

If terraform isn't working or you want to iterate directly:

```bash
# On watershop, in the repo checkout:
cd terraform/station/staging

# Teardown any previous state
./teardown-all.sh

# Set up all three ponds
./setup-all.sh

# Run the full pipeline
./run-all.sh

# Preview
cd dist && python3 -m http.server 4180
```

Note: if running from a repo checkout on watershop, the pond.sh scripts reference
`../site-content/` for the site files (pushed by terraform). If running from the
repo directly, you may need to create a symlink:
```bash
ln -s ../../../site staging/site-content
```

### What Each Step Does

1. **setup-all.sh** — runs setup.sh for water, noyo, and site ponds:
   - Creates podman named volumes (`pond-water-staging`, `pond-noyo-staging`, `pond-site-staging`)
   - Initializes each pond
   - Installs factory nodes (ingest, reduce, analysis, hydrovu, backup, sitegen, etc.)

2. **run-all.sh** — runs the full pipeline sequentially:
   - **Water**: ingest logfiles from `/home/data/`, push backup to MinIO `water-staging` bucket
   - **Noyo**: collect from HydroVu API, push backup to MinIO `noyo-staging` bucket
   - **Site**: import water+noyo from MinIO, generate combined site to `dist/`

3. **teardown-all.sh** — removes all podman volumes, clears `dist/`

### Debugging

- Check podman is working: `podman run --rm hello-world`
- Check MinIO buckets: `mc ls local/water-staging` and `mc ls local/noyo-staging`
- Check container image pulls: `podman pull ghcr.io/jmacd/duckpond/duckpond:latest-arm64`
- Run individual pond commands: `./water/pond.sh list /` (should show pond contents)
- If "Cannot connect to Podman" — podman needs to be installed/running on this machine
- The staging image tag is `latest-arm64` (watershop is ARM); gateway/cloud use `latest-amd64`

### Architecture Diagram

```
Staging on watershop.casparwater.us
┌─────────────────────────────────────────────────┐
│                                                 │
│ water pond ──push──▶ MinIO (water-staging)      │
│   logfile-ingest from /home/data/               │
│   temporal-reduce (5 metrics × 5 resolutions)   │
│   pump cycle analysis                           │
│                                                 │
│ noyo pond  ──push──▶ MinIO (noyo-staging)       │
│   HydroVu API collection                        │
│   combine → single → reduce                    │
│                                                 │
│ site pond  ◀──pull from both MinIO buckets      │
│   cross-pond import water + noyo                │
│   combined sitegen → dist/                      │
│   water site at /                               │
│   noyo subsite at /noyo-harbor/                 │
│                                                 │
└─────────────────────────────────────────────────┘
```

### Key Files

| File | Purpose |
|------|---------|
| `staging/env.sh` | Shared env (image tag, MinIO creds) |
| `staging/water/pond.sh` | Podman wrapper for water pond |
| `staging/water/setup.sh` | Init water pond + install factories |
| `staging/water/run.sh` | Ingest + backup push |
| `staging/noyo/pond.sh` | Podman wrapper for noyo pond |
| `staging/noyo/setup.sh` | Init noyo pond + install factories |
| `staging/noyo/run.sh` | Collect + backup push |
| `staging/site/pond.sh` | Podman wrapper for site pond |
| `staging/site/setup.sh` | Init site pond + install imports + sitegen |
| `staging/site/import.sh` | Pull from MinIO |
| `staging/site/generate.sh` | Build combined site |
| `staging/site/site.yaml` | Combined sitegen config with subsites |

### After Staging Validates

Once the site looks good at `http://watershop.casparwater.us:4180/`:

1. Deploy gateway: `cd terraform/station/gateway && tofu apply`
2. Deploy cloud: `cd terraform/station/cloud && tofu apply`
