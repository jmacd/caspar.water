# Selfmon: design and current limitations

`watershop-selfmon` is a self-monitoring watertown instance running on
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
| `measure-pond.sh` | Per-pond probe. Forks a subshell, re-sources `<pond>.env` so `POND` / `S3_*` point at the **probed** pond, asks that pond for `committed.txn_ids`, `parquet.files`, `delta_log.files`, `size.bytes`, `list.seconds`, journalctl-greps the unit for `peak_rss.bytes` and `run.seconds`, queries `systemctl --user` for `timer.active` and `last_run.seconds_ago`, appends one JSON row to `<pond>.jsonl`. Kept separate from the orchestrator because of the env-scoping requirement. |

#### Containerized vs. host-native ponds

On watershop, every pond except `watershop-selfmon` runs in a podman
named volume rather than at the host path that its env file's `POND`
variable names. Rootless podman keeps each volume's `_data` directory
in `~/.local/share/containers/storage/volumes/<vol>/_data`, owned by
the user, so the host-fs probes work directly against that path
without paying podman startup latency. `measure-pond.sh` resolves
`POND_VOLUME` (set in the container env files) via
`podman volume inspect --format '{{.Mountpoint}}'` and overrides
`POND` for the duration of the probe. Falls back to the env file's
`POND` value when `POND_VOLUME` is unset (e.g. `watershop-selfmon`
itself, which is host-native).

Before this fix every containerized pond reported all-zeros forever,
because the env file's `POND=/home/jmacd/pond-<name>` is just the
**container's** bind-mount label and does not exist on the host.

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

### Liveness signals

`measure-pond.sh` also emits three columns derived from `systemctl
--user` rather than from the pond itself, so a pond with **no recent
activity** still produces a meaningful row:

| Column | Source | Semantics |
|---|---|---|
| `timer.active` | `systemctl --user is-active <unit>.timer` | `1` when active, `0` otherwise (inactive, failed, disabled). |
| `last_run.seconds_ago` | `systemctl --user show <unit>.service -p ExecMainExitTimestamp --value` | Wall-clock seconds since the service's last clean exit. `-1` when the service has no parseable exit timestamp -- this includes "never run", "currently running", and parse failures, so the value is **not** a reliable in-flight signal on its own. |
| `timer.interval_s` | `systemctl --user show <unit>.timer -p OnUnitActiveSecUSec --value` | Configured `OnUnitActiveSec` in integer seconds (rounded up). Drives the dashboard's "stale = 2x interval" health threshold so the page knows what "overdue" means without per-unit YAML configuration. `0` when the timer is missing or the value is unset/infinity. |

These exist because every other signal in the per-pond JSONL row goes
silent the moment a pond stops being scheduled: the perf series stops
advancing, the status grid drops the unit because there are no fresh
journal entries, and the dashboard ends up showing a frozen-but-not-
obviously-frozen view. A flat-zero `timer.active` line and a
monotonically growing `last_run.seconds_ago` line make the failure
mode visible. As of the deploy that introduced these columns, all
four prod timers on watershop (`water-prod`, `septic-prod`,
`noyo-prod`, `site-prod`) have been inactive for at least three days
without selfmon noticing -- exactly the case these columns now
catch.

### Status page health classification

The status grid (front page of the dashboard) colours each card based
on the journal-derived terminal events plus the perf-derived timer
fields above. The `Health::classify` helper in
`crates/sitegen/src/shortcodes.rs` is a pure function over those
inputs:

| Color | Rule |
|---|---|
| **Red** | `timer.active == 0` (timer disabled), OR the most recent terminal event is a failure (`last_err_us > last_ok_us`, or `last_err_us` is set with no `last_ok_us` at all). |
| **Yellow** | `last_ok_us` is older than `2 * timer.interval_s` -- the run is overdue. |
| **Green** | Recent successful run within the staleness threshold and no fresher failure. |
| **Grey (Unknown)** | `timer.interval_s` is missing or zero, or no `last_ok_us` has been seen yet. The card still renders but lacks a colour. |

`StatusGridConfig.perf_pattern` (in the site config) controls where
the perf data comes from; for selfmon it is
`"series:///derived/p-{pond}"`. The `{pond}` placeholder is the unit
basename with `user-pond@` / `user-pond-selfmon@` prefix and
`.service` suffix stripped. A missing perf series for a unit logs a
`warn!` and leaves the card grey rather than failing the build.

### Local-time display

All timestamps in the status cards are emitted as
`<time datetime="..." data-utc-us="...">UTC string</time>` markers.
The companion `crates/sitegen/assets/relative-time.js` script (loaded
from the `data` layout alongside `chart.js`) walks every such element
on `DOMContentLoaded`, replaces its text with the visitor's local
time via `Intl.DateTimeFormat`, and appends a sibling
`<span class="rel-time">` with a "X minutes ago" relative label. A
`setInterval(30s)` keeps the relative label fresh as the page sits
open. There is no server-side timezone configuration; the page works
the same in every browser regardless of where sitegen ran.

### Routes and sidebar

The dashboard has two pages, linked by a sidebar driven by sitegen's
`partials.sidebar` + `sidebar:` config (see `config/watershop-selfmon.yaml`):

```
+-- Status (/selfmon/index.html)         pond_status_grid -- the front page
+-- Metrics (/selfmon/metrics.html)      chart wall -- per-metric line graphs
```

Both pages use `layout: data` so they share the chart.js / overlay.js /
relative-time.js loader and the same theme/sidebar shell. The
`partials.sidebar` partial (`config/selfmon/site/sidebar.md`) is a
single `{{ content_nav /}}` shortcode; the `sidebar:` config carries
explicit `DirectLink` entries so `content_nav` renders pills directly
rather than trying to match against content pages.

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

`config/semconv/watertown-pond.yaml` is the authoritative metric-name
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

`run_status_grid_queries` (`watertown/crates/sitegen/src/factory.rs`,
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
running on the host. We accept this -- exercising watertown under
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

### 8. Selfmon tick duration on slow ponds

`measure-pond.sh` shells out to `find` and `du` once per pond per
tick. On `site-staging` (5 K parquet files, 950 MB) one probe takes
several seconds; the full per-tick loop currently runs ~10-11 minutes
end to end. With the selfmon timer's `OnUnitActiveSec=1min` the
effective measurement cadence is ~12 minutes, not 1 minute. Acceptable
for current goals (long-term growth visibility) but worth replacing
the host-fs traversal with a dedicated `pond` subcommand if we ever
want sub-minute cadence.

### 9. `last_run.seconds_ago == -1` while a service is running

A `pond run` invocation has no `ExecMainExitTimestamp` until it exits,
so `last_run.seconds_ago` reports `-1` for any pond whose service is
currently mid-tick. For long-running services like `pond@site-staging`
this means the column toggles between a real "seconds since last exit"
value and `-1` rather than reporting a continuously growing number.
Could be improved by falling back to `ExecMainStartTimestamp` (or
`ActiveEnterTimestamp`) when the service is in `activating`/`active`
state; not done today.

## What is NOT a limitation (clarifications worth keeping)

- **Selfmon does not write to R2.** Selfmon's `S3_ENDPOINT` points at
  the local MinIO on the same box. The string `push` that appears in
  many `pond run` invocations is a positional argument forwarded to
  the destination factory, not a network operation.
- **Three scripts collapsed to two.** `measure-self.sh` was inlined
  into `run-selfmon.sh` because it had no env-scoping reason to be
  separate. `measure-pond.sh` stays standalone because it re-sources
  each pond's env.
- **`journalctl --merge` is correct.** All watertown units on watershop
  run as user units; system units are unrelated. The merge flag was
  the fix for the empty-status-grid bug we hit during deployment.

## Where things live

| Concern | Path |
|---|---|
| Selfmon config | `config/watershop-selfmon.yaml` |
| Tick orchestrator | `config/scripts/run-selfmon.sh` |
| Per-pond probe | `config/scripts/measure-pond.sh` |
| Semantic conventions | `config/semconv/watertown-pond.yaml` |
| Sitegen templates (status / metrics / sidebar) | `config/selfmon/site/*.md` |
| `pond_status_grid` shortcode | `watertown/crates/sitegen/src/shortcodes.rs` |
| `Health::classify` | `watertown/crates/sitegen/src/shortcodes.rs` |
| `run_status_grid_queries` (journal + perf enrichment) | `watertown/crates/sitegen/src/factory.rs` |
| `extract_pond_name` (unit -> `{pond}` placeholder) | `watertown/crates/sitegen/src/factory.rs` |
| Local-time hydration script | `watertown/crates/sitegen/assets/relative-time.js` |
| `journal-ingest` factory | `watertown/crates/provider/src/factory/journal_ingest.rs` |
| `Run summary` log line | `watertown/crates/cmd/src/main.rs`, `commands/run_summary.rs` |
| Format cache (the source of limitation #1) | `watertown/crates/provider/src/format_cache.rs` |
| Deploy | `tools/deploy-watershop.sh` |
