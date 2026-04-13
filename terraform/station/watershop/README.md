# Staging Environment — watershop.casparwater.us

Pre-release validation of the full Caspar Water pipeline.

## Overview

Runs all three duckpond instances on watershop, using MinIO as the
backup/import target (simulating R2). This validates the entire pipeline
end-to-end before deploying to production gateway and cloud machines.

```
┌─────────────────────────────────────────────────┐
│ watershop.casparwater.us                        │
│                                                 │
│ water pond ──push──▶ MinIO (water-staging)      │
│ noyo pond  ──push──▶ MinIO (noyo-staging)       │
│                         │                       │
│ site pond  ◀──pull──────┘                       │
│    └── sitegen ──▶ dist/                        │
└─────────────────────────────────────────────────┘
```

## Quick Start

```bash
cd terraform/station/staging
tofu init    # or terraform init
tofu apply   # pushes configs to watershop, runs teardown + setup

# Then SSH in to run the pipeline and preview
ssh jmacd@watershop.casparwater.us
cd staging
./run-all.sh

# Preview the site
cd dist && python3 -m http.server 4180
# Browse: http://watershop.casparwater.us:4180/
```

## Individual Pond Operations

```bash
# Reset just the water pond
./water/teardown.sh && ./water/setup.sh && ./water/run.sh

# Reset just the noyo pond
./noyo/teardown.sh && ./noyo/setup.sh && ./noyo/run.sh

# Reset just the site pond
./site/teardown.sh && ./site/setup.sh
./site/import.sh && ./site/generate.sh
```

## Prerequisites

- podman installed
- MinIO running at localhost:9000 (mc alias "local" configured)
- HydroVu credentials in `~/.bashrc.private`
- Water logfiles at `/home/data/casparwater*.json`
- Container image: `ghcr.io/jmacd/duckpond/duckpond:latest-amd64`

## What This Validates

- Water logfile ingest → temporal-reduce → pump cycle analysis
- Noyo HydroVu collection → combine → reduce pipeline
- S3-compatible backup push/pull cycle (via MinIO)
- Cross-pond import of both datasets
- Combined sitegen with water site at `/` and noyo subsite at `/noyo-harbor/`
- Blog, content pages, and data charts render correctly
