# Selfmon: design and current limitations

`watershop-selfmon` is a self-monitoring duckpond instance running on
`watershop.casparwater.us`. Every ~60 s it probes each local pond and
the host journal, joins the resulting time-series, and renders a tiny
static dashboard served by Caddy at `/selfmon/`.

This document captures the design as it actually runs today and the
inefficiencies we have observed but not yet fixed.

## Goals

- See how each pond on this host scales with sustained load and growth
  (RSS, txn rate, parquet/delta-log file counts, on-disk size, list
  cost, per-tick `pond run` duration).
- Show systemctl-status-style cards per pond unit so we can spot
  stuck/failing services without `ssh + journalctl`.
- Be cheap enough to run continuously next to the real workload.
- Be the testbed we use to evaluate efficiency problems elsewhere
  (e.g. the multi-hour `pond@site-staging` sitegen we are watching
  right now).

## Component layout

```
systemd-journald (host)
        │
        ▼
  journal-ingest factory      ── reads via cursor, appends per-unit
                                  JSONL (FilePhysicalSeries, one new
                                  version per tick)
        │
        ▼
  /logs/journal/<unit>.jsonl     /logs/web/watershop/access.*.jsonl
        │                                  │
        └──────────┬───────────────────────┘
                   │
                   ▼  every tick (run-selfmon.sh)
   ┌──────────────────────────────────────────────────────┐
   │  1. measure  per-pond + _self  → /measure/*.jsonl    │
   │  2. ingest   journal, caddy access, per-pond perf    │
   │  3. sync     site templates host → pond              │
   │  4. maintain delta-log checkpoint / cleanup          │
   │  5. sitegen  /derived/perf → /var/www/selfmon/<inst> │
   └──────────────────────────────────────────────────────┘
                   │
                   ▼
            Caddy /selfmon/
```

### Scripts (under `config/scripts/`)

| Script | Role |
|---|---|
| `run-selfmon.sh` | Tick orchestrator. Runs as the systemd unit's main job (`pond-selfmon@<inst>.service`). Sources the selfmon env, performs the five steps above, and inlines the `_self` probe as a brace block. |
| `measure-pond.sh` | Per-pond probe. Forks a subshell, re-sources `<pond>.env` so `POND` / `S3_*` point at the **probed** pond, asks that pond for `committed.txn_ids`, `parquet.files`, `delta_log.files`, `size.bytes`, `list.seconds`, journalctl-greps the unit for `peak_rss.bytes` and `run.seconds`, appends one JSON row to `<pond>.jsonl`. Kept separate from the orchestrator because of the env-scoping requirement. |

### Pond contents (`config/watershop-selfmon.yaml`)

- `journal-ingest` mknods for the host journal (with `extra_args:
  [--merge]` so user-scope pond units are captured) and for caddy
  access logs.
- `logfile-ingest` mknods, one per pond + `_self`, that mirror each
  measure script's per-tick JSONL into the selfmon pond.
- One `sql-derived-series` per pond + `_self` that CASTs the JSONL's
  Utf8 columns into typed columns. All pond entries share a YAML
  anchor `&pondseries`, so adding/removing a column happens in one
  place.
- One `timeseries-join` (`/derived/perf`) that FULL OUTER JOINs every
  pond's typed series on `timestamp`, scope-prefixing each input's
  columns to `<scope>.<param>.<unit>`.
- One `sitegen` factory at `/system/etc/sitegen` that consumes
  `/derived/perf`, references templates under `/system/site`,
  registers a metric-instrument-kind table (`metric_registry`) for
  per-kind chart transforms (counter → rate, etc.), provides
  human-readable per-chart captions, and renders a status-grid via
  the `pond_status_grid` shortcode driven by per-unit-glob queries
  against `jsonlogs:///logs/journal/<unit>.jsonl`.

### Pond CLI exit log

Every `pond run` emits two log lines at process exit:

```
Peak memory usage: NN.NN MB
Run summary  path=<path>  factory=<kind>  args=<args>
             elapsed_s=<f64>  peak_mem_mb=<f64>  outcome=ok|err
```

The `Peak memory` line predates this work; the `Run summary` line was
added so `measure-pond.sh` can report `run.seconds` per pond and so
future per-factory analysis can attribute time/memory cost to a
specific factory kind.

### Semantic conventions

`config/semconv/duckpond-pond.yaml` is the authoritative metric-name
registry. Gauges are the default and intentionally omitted; only
non-gauge metrics (counter, updowncounter) need entries. Sitegen's
`metric_registry` mirrors the non-gauge entries -- keep them in sync.

### Per-tick ordering

Measurements run **before** ingest each tick so the JSONL row written
this tick is ingested this tick (one less round-trip to seeing a
fresh number on the dashboard). Probes only read pond state; they
never write, so running them before maintain/checkpoint is safe.

## Known limitations and observed inefficiencies

### 1. Format-cache version explosion (the big one)

`journal-ingest` writes one new FileSeries version per tick. The
`jsonlogs://` table provider goes through the format cache, which
materializes **one Parquet file per source version**. After ~100 ticks
(~100 minutes) a single 32 KB JSONL becomes 101 tiny Parquet files
(avg ~325 bytes of payload each).

Measured on watershop, `user-pond@water-staging.service.jsonl` (32 KB,
v101):

| Query | Time | Peak RSS | Rows |
|---|---|---|---|
| Cold, unbounded `COUNT(*)` | 34 s | 125 MB | 3819 |
| Warm, unbounded `COUNT(*)` | 7.2 s | 124 MB | 3819 |
| Warm, `WHERE __REALTIME_TIMESTAMP >= now()-30min` | 10.5 s | 125 MB | 85 |

Adding `WHERE` is correctness-correct but does **not** help: every
query opens all 101 versioned Parquets. DataFusion's `ListingTable`
does not prune by `__REALTIME_TIMESTAMP` (Utf8) file-level stats, and
even if it did, opening 101 files just to read footers is itself the
dominant cost. `journal-ingest` already calls
`writer.set_temporal_metadata(min_ts, max_ts, ts_col)` per version
(`journal_ingest.rs:417`), but those bounds are not yet threaded into
the `jsonlogs://` listing path.

Consequence: every status_grid render re-pays the full cost. As the
journal grows, per-render cost grows unboundedly.

This is the same shape of cost we suspect is hurting `pond@site-staging`'s
multi-hour sitegen runs (many small parquets in `temporal-reduce`
output), so a fix here likely benefits both.

### 2. Many tiny JSONL files

The same per-tick versioning produces a long tail of small files for
every monitored unit (~30 system + user units on watershop). Ingest is
incremental and cheap, but the cumulative on-disk inode pressure and
per-version Parquet cache footprint scales linearly with retained
ticks.

### 3. Status-grid re-runs full SQL each render

`run_status_grid_queries` (`duckpond/crates/sitegen/src/factory.rs`,
~lines 681 and 738) issues two queries per (unit, glob): a
`MAX(...)`-style status query and a `LIMIT N ORDER BY DESC` tail
query. With currently ~9 ponds × 2 globs = ~18 queries per sitegen
build. Each query pays the version-explosion cost from (1).

### 4. Selfmon loads the system intentionally

The selfmon tick performs a `pond maintain` and a `remote` push to the
local MinIO each tick. That is a real S3 write per pond per tick (the
post-commit `remote` factory in `push` mode writing to
`http://watershop.casparwater.us:9000/watershop-selfmon`, **not** R2).
Under sustained load these pushes contend with whatever else is
running on the host. We accept this -- exercising duckpond under
resource constraints and continuous growth is part of the experiment.

### 5. `Run summary` is not yet observable per-factory

The new `Run summary` log line carries `factory=<kind>` but
`measure-pond.sh` only extracts `elapsed_s`, dropping the factory
attribute. Surfacing per-factory duration would require a non-numeric
column path through `sql-derived-series` → `timeseries-join` → chart,
which today only handles numeric columns.

### 6. Histograms are not in the chart pipeline

Chart transforms today are: counter → first-difference rate;
updowncounter → identity; gauge or unknown → identity. Histogram
support (HDR-style or just bucketed gauges) would need both a
semconv extension and a renderer change. We have agreed to start with
gauge/updowncounter and add histograms later.

### 7. Overlapping long-running services

`pond@site-staging` is currently running for 70+ minutes per
invocation at ~2.7 GB RSS. If its systemd timer fires at a shorter
interval than its actual duration, runs will either stack
(`Type=simple`) or get serialized by the unit lock (`Type=oneshot`).
Worth checking the timer unit's `OnUnitActiveSec` vs. observed run
duration, but unrelated to selfmon's own loop.

## What is NOT a limitation (clarifications worth keeping)

- **Selfmon does not write to R2.** Selfmon's `S3_ENDPOINT` points at
  the local MinIO on the same box. The string `push` that appears in
  many `pond run` invocations is a positional argument forwarded to
  the destination factory, not a network operation.
- **Three scripts collapsed to two.** `measure-self.sh` was inlined
  into `run-selfmon.sh` because it had no env-scoping reason to be
  separate. `measure-pond.sh` stays standalone because it re-sources
  each pond's env.
- **`journalctl --merge` is correct.** All duckpond units on watershop
  run as user units; system units are unrelated. The merge flag was
  the fix for the empty-status-grid bug we hit during deployment.

## Where things live

| Concern | Path |
|---|---|
| Selfmon config | `config/watershop-selfmon.yaml` |
| Tick orchestrator | `config/scripts/run-selfmon.sh` |
| Per-pond probe | `config/scripts/measure-pond.sh` |
| Semantic conventions | `config/semconv/duckpond-pond.yaml` |
| Sitegen templates | `config/selfmon/site/*.md` |
| `pond_status_grid` shortcode | `duckpond/crates/sitegen/src/shortcodes.rs` |
| `run_status_grid_queries` | `duckpond/crates/sitegen/src/factory.rs` |
| `journal-ingest` factory | `duckpond/crates/provider/src/factory/journal_ingest.rs` |
| `Run summary` log line | `duckpond/crates/cmd/src/main.rs`, `commands/run_summary.rs` |
| Format cache (the source of limitation #1) | `duckpond/crates/provider/src/format_cache.rs` |
| Deploy | `tools/deploy-watershop.sh` |
