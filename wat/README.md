# Operating the Caspar Water books

`wat/` will be the private operational accounting workspace, not an example or
demo. The first production milestone is a reviewed and reconciled reconstruction
of 2021.

## Repository boundary

The local `.gitignore` denies everything by default. Commit only:

- `wat/README.md`;
- `wat/.gitignore`; and
- generic, non-instance `wat/reference/*.yaml` definitions selected by
  `chart_defs`.

Keep `wat.yaml`, generated account/include files, journals, mappings, source
documents, AI responses, review drafts, Pond state, and backups private and
untracked. Before any commit involving this directory, run:

```sh
git status --short --ignored wat
```

The original QuickBooks exports and other source systems remain authoritative.
The Pond must not become the only recoverable copy of accounting data.

## Operational sequence

Authority should move in three deliberate stages.

### 1. Operate Wat on ordinary files

First make the Wat program useful with `wat/` as its direct filesystem
workspace. In this stage, `wat/books/*.journal` are the live books. Copy the
existing master chart to `wat/reference/cpuc-u39w.yaml`, review it, and
commit it. Add that workspace-relative path to `chart_defs` in the private
`wat/wat.yaml`, alongside the real posting and bank accounts.

The direct launcher should:

- accept `wat/` as an explicit workspace rather than hardcoding `local/`;
- mount the workspace read-write and the source inbox read-only;
- provide Wat, hledger 1.52, the pinned Python statement parser, and the `pond`
  executable in one reproducible image;
- use `pond cat` only as a stateless `.xls`/`.xlsx` decoder;
- pass short-lived Azure credentials only through the environment; and
- preserve Wat's existing locks, validation, atomic replacement, and local
  recovery copies.

Using the `pond` executable for spreadsheet conversion does not make a Pond the
accounting store. Establish an off-repository backup of `wat/` before posting
accepted transactions.

Bootstrap the direct workspace once from the current configuration and master
chart:

```sh
mkdir -p wat/books wat/reference wat/sources wat/review
cp local/wat-test/wat.yaml wat/wat.yaml
cp cmd/wat/example/reference/cpuc-u39w.yaml \
  wat/reference/cpuc-u39w.yaml
```

Before continuing, edit `wat/wat.yaml` and set:

```yaml
chart_defs: [reference/cpuc-u39w.yaml]
```

Then initialize and inspect the empty books:

```sh
go run ./cmd/wat --config wat/wat.yaml check
go run ./cmd/wat --config wat/wat.yaml accounts
```

This creates new, initially empty file-authoritative books and leaves the
existing test Pond untouched. Review `wat/wat.yaml` before continuing. If the
test Pond contains accepted transactions that must be retained, inventory and
export them instead of silently starting from empty journals.

Open the direct terminal with:

```sh
go run ./cmd/wat --config wat/wat.yaml
```

### 2. Run the same Wat program under Watertown

Once the direct workflow is routine, run the unchanged Wat CLI as a Watertown
exec program. During this stage, the filesystem workspace remains authoritative:
import a complete snapshot, run Wat transactionally, and export the complete
validated result back to `wat/` atomically. Record and compare manifests so a
concurrent filesystem change cannot be overwritten.

The current `cmd/wat/run.sh` is not this boundary: it hardcodes
`local/wat-test`, keeps a persistent `local/wat-pond`, and leaves journals
Pond-owned after the initial import. Treat it as an integration prototype; do
not make it the production path by merely changing its directory names.

### 3. Decide whether Pond becomes primary

Promote Pond to the authoritative accounting store only after all of these are
proven:

- complete deterministic export of the accounting workspace;
- restore from original sources plus private backups;
- scheduled off-Pond backups and a tested disaster-recovery procedure;
- manifest and content-hash verification across import/export;
- safe migration from the file-authoritative workspace with an explicit
  cutover point; and
- equivalent Wat validation, reporting, locking, and recovery behavior.

Even after cutover, original QuickBooks exports, statements, and other source
systems remain authoritative evidence and must stay recoverable independently.

## Resolve 2021 in the file-authoritative stage

1. **Inventory deterministically.** Run `quickbooks --inspect --year 2021`
   against the read-only source inbox. Retain the private report and explain
   every source file, zero-row workbook, duplicate appearance, and ambiguous
   transaction group.
2. **Approve accounts.** Run `quickbooks --review --year 2021`. Map only
   categories supported by the source evidence and CPUC chart; `F2` saves the
   private `quickbooks-mappings.yaml`. Leave transaction-dependent categories
   unresolved.
3. **Generate once for review.** Run `quickbooks --year 2021`, saving stdout and
   stderr to uniquely named private files. Never overwrite a reviewed run.
   Generation does not post anything to the books.
4. **Resolve every skeleton.** Work through the commented entries using the
   business records. Supply the missing bank, liability, owner-equity, capital,
   or expense treatment only when supported. Record the evidence and keep
   genuinely unknowable facts explicitly unresolved; do not ingest personal
   bank history or guess a balancing account.
5. **Accept safely.** Preserve the generated draft, make a separate working
   copy under `wat/review/`, and edit it with Wat stopped. Keep unresolved
   skeletons in that private working file. Copy only completed, reviewed entries
   into `wat/books/manual.journal`, make a private backup first, and run
   `go run ./cmd/wat --config wat/wat.yaml check` after each small batch. Never
   edit generated `accounts.journal` or `books.journal`. A future bulk importer
   should reject unresolved markers and duplicate IDs, validate the exact
   combined books, preview the change, and atomically save with a backup; it
   must never call the model or silently alter entries.
6. **Reconcile before declaring 2021 complete.** Run `wat check`, reconcile the
   business-bank activity by month, reconcile opening and ending balances, and
   compare account totals, Balance Sheet, and Profit and Loss to the available
   QuickBooks control reports. Every difference must be zero or documented.

## Definition of done

2021 is complete only when every normalized source row has exactly one traced
disposition, every posted entry uses an approved account and balances, all
unresolved limitations are documented, control-report differences are
explained, the final books pass strict validation, and a private recoverable
backup exists.
