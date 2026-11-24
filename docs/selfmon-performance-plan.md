# Watershop selfmon: next performance fixes

**Status: milestones 1-4 implemented locally, not deployed.** Selfmon now runs the count without a preflight listing and logs cursor-check/seed time separately as `selfmon_timing step=journal-cursor elapsed_s=...`. On a disposable `0.309.228` pond, a first run seeded the cursor, a warm run counted all three rows across two files without reseeding, and no-match and query-error runs each recorded one `read-benchmark` failure.

Template sync reads the current pond templates in one snapshot, compares their bytes with the host files, and copies just the differences in one transaction. An unchanged tick makes no template commit, including after a restart; a restored pond is compared on its own contents. Use `selfmon_templates copied=... elapsed_s=...` to compare with the 1m18s baseline after rollout. Neither milestone's time savings have been measured on Watershop.

Watertown commit `e028d7c0` makes the transparency-log exporter check the authoritative leaf count and tip using version metadata. A caught-up checkpoint needs no payloads; a normal append fetches one; a lagging export fetches its ordered suffix. Losing the export still triggers full replay. The metadata query is not necessarily constant-time. With `steward::content_tree=debug`, the transparency-log messages include `elapsed_ms` for comparison after rollout.

Watertown commit `75a63cd5` validates older JSON-log chunks against the microsecond column named by a labeled chunk of the same source. The journal writer allowed a configurable field, so its default alone is not proof. Successful checks are cached by immutable version, content, column and bounds. Expect one full target verification when adopting a previously unproven checkpoint. After that, strictly newer appends can avoid historical verification; ambiguous or conflicting time metadata still requires the conservative path. No history rewrite or pond reset is needed.

**Milestone 5: instrumentation implemented locally; session reuse not decided.**
Watertown commit `e352f202` adds opt-in phase timings to `pond run`; selfmon
enables them for all 12 measurement feeds without changing their transactions
or retries. After an approved rollout, use the [per-feed journal records](../README.operations.md#selfmon-io-diagnostics)
for at least five warm ticks. Sum each phase across the feeds per tick and
compare with the 4m45s ingest baseline. Reusing an open pond can save repeated
open work; it will not remove per-feed commit or publication costs.

Also check whether `/system/etc/materialize-azure-access` stops reporting
`lower_us=None` on warm append-only ticks. Item 6's maintenance timings can
be added independently.

The goal is less work for the **same tick**: keep the full-history benchmark, all measurement feeds, current timer, maintenance safeguards, and visible failures. Do not trade away history or correctness for a faster dashboard.

## Baseline: 2026-11-23, `pond 0.309.228`

The [November 13 investigation](../watertown/docs/selfmon-incremental-work-report.md) recorded roughly an hour and 93 GB of logical reads per tick. Recent successful ticks take about 30 minutes and read about 49 GB. The timer is nominally one minute; a 30-minute tick is not a 30-minute idle interval. Compare elapsed time separately from systemd CPU time, which can exceed elapsed time.

The tick from **15:23:25 to 15:53:55 UTC** took 30m30s, used 37m02s of CPU, and had zero failed steps. Its `rchar` was 48.94 GB; `read_bytes` was only 1.63 MB and `write_bytes` was 532 MB. Most reads were not disk reads.

| Work in that tick | Wall time | What we know |
|---|---:|---|
| Maintenance | 8m07s | 11.72 GB logical reads; 40 candidate series, 25 repacked, 15 already bounded. Recent runs took 6-9 min. No timings yet separate pruning, pack work and Delta compaction. |
| Cursor check, journal listing and benchmark | 6m29s combined | The timed `COUNT(*)` was 3m09s. The remaining 3m20s is **not** a measured listing time. `pond list` collects and sorts the directory before the shell tests for the first `.jsonl`. |
| Azure-access materialization | 5m48s | 6.25 GB logical reads. Repeated `verifying ... lower_us=None` on warm ticks: the engine cannot prove the time bounds of old journal versions. |
| 12 measurement ingests | 4m45s | 6.78 GB logical reads. A 267-byte append finished ingestion about 4s after start, but the command took 21s and peaked at about 1.2 GB. This is ingestion into selfmon, not the cost of probing the 12 files' sources. |
| Four template copies | 1m18s | Four overwrites and four commits, even when no template was edited. |
| Perf, limiters, Sitegen | 55s, 30s, 59s | Earlier fixes are holding. Keep these as regression checks, not first targets. |

These times are not additive savings. In particular, removing a template commit also removes its commit-log work.

## Implementation order and completion criteria

1. **Remove the duplicate journal scan (low complexity, outer repo).** In [run-selfmon.sh](../config/scripts/run-selfmon.sh), time cursor bootstrap separately, retain first-run seeding, and run the real count without first listing the directory. Verify the query's no-match behavior: an absent source or failed query must still set `read.ok=0` and count a failure, not publish a fast successful read. If the query cannot distinguish no files from valid empty input, use a stop-at-first-match existence check rather than another full listing. **Done when:** a normal tick does one full-history count, no full preflight listing, and returns the same row count on a fixed fixture.

2. **Copy only changed templates (low complexity, outer repo).** Compare the four small host files with the actual pond destinations before writing; prefer one cheap lookup of destination digests if available. A local “last copied” marker alone would fail after a pond restore. **Done when:** no edits means zero template writes/commits; editing one, deleting its destination, or starting with a fresh pond installs exactly what is missing or different. Read errors must not be treated as equality.

3. **Read only missing transparency-log leaves (moderate, Watertown).** [`materialize_tlog` and `read_log_leaves`](../watertown/crates/steward/src/content_tree.rs) currently select, order and load the entire reserved commit log *before* checking the exported checkpoint size. Watershop had 43,111 exported leaves. The tile writer already appends incrementally. Use the committed tip and a validated export boundary to fetch only the missing suffix in order; do not assume that a transaction sequence or Delta version is a leaf position. Keep the authoritative full-replay path for recovery. **Done when:** a caught-up export reads no historical payloads, one new commit reads one leaf, and a missing export or interrupted multi-leaf catch-up yields the same root and tile contents as full replay. Count payloads read in the regression test; measure post-commit time separately from ingestion. The full scan is proven; its share of the 21s append is not.

4. **Restore bounded Azure-access verification (moderate if legacy metadata can be proven, Watertown).** The [Azure-access query](../config/watershop-selfmon.yaml) uses journal `__REALTIME_TIMESTAMP` in microseconds. Some pre-upgrade versions have bounds but no recorded time column. Do not teach all JSON logs to assume that axis. First look for an unambiguous journal-writer guarantee; otherwise validate the legacy bounds against those immutable versions once. Tie any reusable proof to the exact source version, content and timestamp interpretation. **Done when:** after one verification, warm append-only ticks no longer verify all history; mixed/unknown axes, lost proof and retroactive changes still trigger the existing verification or rejection. Compare exact output, including duplicates, with full verification. If the only route requires a pond reset or rewriting history, retain the safe fallback and stop for design review.

5. **Reduce repeated measurement-ingest overhead (start small, after item 3).** Time pond open, factory work, commit and post-commit separately for all 12 feeds. If fixed per-command work is still material, start with the [session-reuse proposal](../watertown/docs/session-reuse-design.md): reuse the open pond for a sequence of existing factories, preserving one transaction and outcome per feed. A single filename glob is not equivalent: `logfile-ingest` selects one active file per node, and `limits.jsonl` has a different timestamp field. **Done when:** the same inputs yield the same rows, cursors and rotation behavior, including after one feed fails and retries; total elapsed time and logical reads drop. Defer a new daemon or cross-feed atomic transaction model unless measurements justify that complexity.

6. **Make maintenance cheaper without weakening it (instrument now; optimize the measured phase).** Add elapsed time and work counts around prune, pack discovery/spooling/repacking, reclaim, and Delta compaction/checkpoint/vacuum in [maintain](../watertown/crates/cmd/src/commands/maintain.rs) and [pack maintenance](../watertown/crates/steward/src/pack_maintenance.rs). The example already reports **one shared inline-content scan**, not one scan per candidate. If repacking dominates, test whether a small append causes an unchanged packed prefix to be reread; if compaction dominates, inspect which unchanged files are revisited instead. **Done when:** the measured dominant phase does less work on repeat or small-append ticks at the same thresholds, with identical logical content, recoverable packs, and bounded file counts. A new pack format, retention policy, or GC protocol needs separate review. Keep pre-tick maintenance in place.

## Qualification

Land the small script changes independently. Test engine changes on disposable ponds, including empty, warm and recovery cases. After an approved selfmon rollout, compare at least five completed warm ticks at similar input volumes with the baseline above. Record per-step wall time, whole-tick CPU, `rchar`, physical I/O, peak memory, failures and the work counts specific to the fix. Report the first catch-up tick separately; do not clear caches, reset the live pond, or slow the timer to make the comparison look better. Use the [operations guide](../README.operations.md#selfmon-io-diagnostics) for the journal counters.
