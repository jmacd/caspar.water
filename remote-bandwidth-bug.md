# Remote import never advances watermark — burns 23 GB / 2 h

**Status:** active bug. Cloud `pond@site-prod.timer` stopped on
2026-04-29 22:51 PT to halt the bandwidth bleed; will stay stopped
until the underlying duckpond bug is fixed.

**Symptom (first observed):** Linode bandwidth alarm on
`debian12-us-west` averaging **25.74 Mb/s inbound for 2 h** (≈ 23 GB
into the cloud node). Cloud's only network workload is `pond run
/system/site-prod` calling three `remote` import factories; nothing
else fetches from R2.

## Repro / observed behaviour

The cloud sitegen pond `/system/site-prod` is configured with three
foreign-pond imports (see `config/site.yaml`):

```yaml
- path: /system/etc/10-water    # remote.import { source_path: "/**", local: /sources/water }
- path: /system/etc/11-noyo     # remote.import { source_path: "/**", local: /sources/noyo }
- path: /system/etc/12-septic   # remote.import { source_path: "/**", local: /sources/septic }
- path: /system/etc/90-sitegen
```

On every tick of `pond@site-prod.timer`, each import factory logs:

```
[DOWN] IMPORT: Importing from foreign pond backup (source=/**, local=/sources/water)
   Discovering child partitions from foreign backup...
   Importing 5 partition(s): [...]
   Watermark: 0 (will import transactions > 0)
   679 new transaction(s) to import: "1..=679"
   ...
   [OK] Import complete: 5 partition(s), 0 file(s) downloaded
```

**Note `Watermark: 0` and `0 file(s) downloaded` together.** The
import correctly identifies that there is nothing new to write
locally — but it only discovers that fact after walking transactions
`1..=679` of water-prod's foreign backup, `1..=675` of septic-prod's,
`1..=58` of noyo's, every tick.

Across ~5 ticks in 2 h, that's:

| Foreign pond | txns walked / tick | Approx parquet metadata read | × 5 ticks |
|---|---|---|---|
| water-prod  | 1..=679 (×5 partitions) | ~10 GB | ~50 GB capable, throttled to ~10 GB/h actual |
| septic-prod | 1..=675 (×5 partitions) | similar | similar |
| noyo-prod   | 1..=58  (×14 partitions) | smaller | |

End result: **0 bytes of new pond data, ~23 GB of OpLog parquet
metadata read**. Not a bandwidth-budget problem, a correctness
problem.

## Where in the code

All references in
`duckpond/crates/remote/src/factory.rs`.

### Watermark is read here (`execute_import`, ~line 1586)

```rust
// Step 2: Determine watermark — highest foreign txn_seq already imported.
let watermark: i64 = context
    .import_partitions
    .iter()
    .map(|(_, _, wm)| *wm)
    .max()
    .unwrap_or(0);

log::info!(
    "   Watermark: {} (will import transactions > {})",
    watermark,
    watermark
);
```

If `context.import_partitions` is empty, **or all entries' `wm` field
is 0**, the watermark resolves to 0 and the entire foreign txn range
is reconsidered.

### Watermark IS written here (`execute_import`, ~line 1707)

```rust
// Record updated watermark via import metadata (flows through Delta commit)
let new_watermark = new_txn_seqs.last().copied().unwrap_or(watermark);
let factory_key = part_ids_to_import.first().cloned().unwrap_or_default();
for pid in &part_ids_to_import {
    state
        .add_import_metadata(tlogfs::ImportPartitionRecord {
            factory_node_id: factory_key.clone(),
            foreign_part_id: pid.clone(),
            foreign_pond_id: String::new(),
            watermark_txn_seq: new_watermark,
        })
        .await;
}
log::info!("   Updated watermark to {}", new_watermark);
```

So the writer path exists. Each import does call
`add_import_metadata` with the new watermark per partition. The bug
must be on either the **persistence** side (the metadata never
actually lands in a Delta commit that survives) or the **read** side
(the next tick's `context.import_partitions` is built from a place
that doesn't see what was written).

### Cached-partitions fast path (~line 1496)

```rust
let part_ids_to_import: Vec<String> = if !context.import_partitions.is_empty() {
    // Use cached partitions from control table (fastest path)
    let ids: Vec<String> = context
        .import_partitions
        .iter()
        .map(|(pid, _, _)| pid.clone())
        .collect();
    ...
} else if !import_config.partitions.is_empty() {
    ... // pre-discovered partitions from mknod time
} else {
    // Discover from local directory entry (top-level) and foreign OpLog (children)
    ...
    log::info!("   Discovering child partitions from foreign backup...");
    ...
};
```

The log shows the **fallback discovery path** (`Discovering child
partitions from foreign backup...`) every tick, never the cached
path. So `context.import_partitions` is **always empty** at the start
of `execute_import`. That is the same data structure the watermark
is read from. Empty `import_partitions` ⇒ no cached partition list
**and** watermark = 0 ⇒ full re-walk.

So the actual question is: **why does
`context.import_partitions` come back empty on every tick despite
`add_import_metadata` having been called on the previous tick?**

Possible failure modes (in rough likelihood order, untested):

1. **`add_import_metadata` is fire-and-forget** (note the unawaited-style call
   in the writer block — though `.await` is there, the function's
   return value is discarded with no `?`). If the underlying write
   fails, the error is silently swallowed and nothing lands in the
   control table.
2. The control table the writer targets and the control table the
   factory-context builder reads from are not the same physical
   location (e.g. one is per-factory, one is global; or one is keyed
   by `factory_node_id`, the other by something else).
3. The metadata is written at txn-end but the per-tick `pond run`
   process is running multiple imports inside one transaction, and
   only the last factory's metadata survives a transaction merge.
4. The `factory_node_id` value used at write time
   (`part_ids_to_import.first().cloned().unwrap_or_default()` — the
   FIRST partition id, not the factory's own node id) is not what
   the read path queries by.

Item (4) is the most concrete-looking smell: writing metadata keyed
by `factory_node_id = <first foreign partition id>` is unlikely to
match a read keyed by the local `/system/etc/12-septic` factory's
node id. If true, the write succeeds but is invisible to subsequent
reads.

## What we know vs. don't know

**Known:**

- The bug is reproducible on every tick (every cloud import logs
  `Watermark: 0`). It is not a one-time corruption.
- The bug is not config-side: `source_path: /**` is the documented
  way to import a whole foreign pond, and the log shows that
  partition discovery itself works (5 partitions for water, 14 for
  septic across years, 5 for noyo) — only the per-partition
  watermark is wrong.
- The cloud's local pond DOES have the imported data: `0 file(s)
  downloaded` per tick proves the local store agrees with the foreign
  txn range. So the data import did succeed at some point — just the
  **watermark of where it stopped** is being lost between ticks.
- The bug is silent: nothing in the journal warns that
  `add_import_metadata` failed. The `[OK] Updated watermark to N`
  line is logged unconditionally even if the underlying state mutation
  was a no-op.

**Not yet verified (next steps):**

1. Inspect the cloud's local site-prod pond's control table directly
   (`pond log /system/etc/12-septic` or similar) and see whether any
   `ImportPartitionRecord` rows exist at all, and what their
   `factory_node_id` values look like.
2. Read the `tlogfs::add_import_metadata` implementation: is it
   committing at all, or is it staging into the next user-driven
   transaction (which never comes for a `remote` factory in
   pull-only mode)?
3. Read whatever populates `FactoryContext.import_partitions` and
   compare its query key to the writer's `factory_node_id` value.

## Operational impact

- **Cloud Linode bandwidth alarm** at 25.74 Mb/s sustained inbound.
- **R2 egress cost** on the source side, charged for object reads
  and ListObjectsV2 calls from the cloud, with zero useful work.
- **Wall-clock cost on the cloud:** site-prod ticks are running
  ~30–40 minutes each just on imports, then sitegen on top, then
  push back to R2 — exceeds the timer interval and stacks /
  serializes runs.
- **Watershop side effects:** unrelated to this bug, but worth
  noting for any future bandwidth investigation:
  - All four production timers (`{water,septic,noyo,site}-prod`)
    on watershop have been **inactive (dead)** since 2026-04-27 12:00
    PT, ~3 days. The cloud was importing from R2 (R2 still has
    everything), not from watershop directly, so the cloud bandwidth
    alarm is independent. But this is also reason to treat the cloud
    site-prod's stale data view as expected, not pathological.

## Mitigations

### Already applied

- `pond@site-prod.timer` and `pond@site-prod.service` stopped on
  cloud at 2026-04-29 22:51 PT.
- The in-flight `pond run /system/etc/12-septic pull` (PID 76463)
  killed with `SIGKILL` after ignoring SIGTERM. R2 connection count
  on the cloud went from 568 ESTABLISHED to 0.

### Should not be applied

- **Do not** "fix" by narrowing `source_path` from `/**` to specific
  partitions. That would mask the watermark bug while leaving every
  next-larger pond exposed; and the existing config matches the
  documented usage pattern.
- **Do not** raise the timer interval to compensate. Each tick
  re-walks the same foreign history regardless of cadence, so the
  same fixed-cost bug just runs less often.

### Real fix

A duckpond change that makes the watermark survive across ticks.
Concretely, in priority order:

1. Verify the silent-failure hypothesis: change
   `state.add_import_metadata(...).await;` (line 1718) to capture
   and propagate the error. If we have been swallowing a write
   failure, that single change will surface it immediately.
2. Audit the (`factory_node_id`, `foreign_part_id`) keying used by
   the writer vs. the read-side query that fills
   `FactoryContext.import_partitions`. Make them match.
3. Add a unit-test or testsuite case that runs two consecutive
   imports against an unchanging foreign backup and asserts the
   second tick reports `Already up to date` rather than re-walking
   transactions.

## Restart preconditions

Before re-enabling `pond@site-prod.timer` on the cloud:

1. The watermark bug is fixed in duckpond and the new image is
   deployed to the cloud (`terraform apply` against
   `terraform/station/cloud`).
2. A trial run is observed: the second tick's import logs
   `Already up to date (no transactions after N)` for at least one
   of the three sources, with N > 0.

## Files

- `/Volumes/sourcecode/src/caspar.water/duckpond/crates/remote/src/factory.rs:1469`
  `execute_import`, the entire function. Watermark read at 1586,
  written at 1707.
- `/Volumes/sourcecode/src/caspar.water/duckpond/crates/remote/src/factory.rs:1496`
  cached-partitions fast path that the bug skips on every tick.
- `/Volumes/sourcecode/src/caspar.water/config/site.yaml:81-93`
  the three import factory mknods.
- Cloud journal (live, not in repo):
  `journalctl --user -u pond@site-prod.service --since '3 hours ago'`.

## Resolution (2026-04-30)

Reproduced in `duckpond/testsuite/tests/540-import-watermark-incremental.sh`
against MinIO and root-caused.  The actual bug was simpler than the
hypotheses above: `crates/cmd/src/commands/run.rs` parsed the factory
config blob with `serde_json::from_slice` to fish out the
`RemoteConfig.import` field, but the config is always **YAML**.
Deserialization failed silently every tick, the
`if let Ok(remote_config) = ...` arm was skipped, and
`FactoryContext.import_partitions` was never populated.  `execute_import`
therefore always took the discovery branch with `watermark = 0` and
re-walked the entire foreign txn history.

Fix: duckpond `f4bb7b8b` switches to `serde_yaml::from_slice` and surfaces
query errors at WARN instead of swallowing them.  Both `add_import_metadata`
write paths (mknod and execute_import) were already keying records
correctly by `foreign_part_id`; the only lossy step was the read.

Restart preconditions for the cloud Linode:

1. Bump the cloud's duckpond image to one that includes `f4bb7b8b`
   (`terraform apply` against `terraform/station/cloud` after the new
   container is published).
2. `systemctl --user start pond@site-prod.timer`.
3. Confirm the second tick of each import logs
   `Watermark: N (will import transactions > N)` with N > 0 (or
   `Already up to date`), not `Watermark: 0`.
