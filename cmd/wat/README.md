# Water Accounting Terminal

`wat` is a local terminal workbench for ordinary hledger journals. It supports
transaction entry, journal review, strict validation, bank-statement
reconciliation checkpoints, and reports. It does not calculate bills or close years.
The application itself uses ordinary local files and has no database, server, or
browser. The development runner stores those files transactionally in a local
Watertown Pond.

## Start a workspace

For a persistent test workspace, run:

```sh
./cmd/wat/run.sh --azure-cli
./cmd/wat/run.sh check
./cmd/wat/run.sh report journal
```

The script requires an existing git-ignored `local/wat-test/wat.yaml`; it does
not install a demo workspace. It creates `local/wat-inbox` and initializes the
persistent Pond at `local/wat-pond`, then builds a Linux accounting image. The
first Pond run imports the complete existing `local/wat-test` workspace, so work
from the earlier direct runner is preserved. Before every later run, the runner
copies changed `local/wat-test/wat.yaml` and its configured `chart_defs` into Pond and
refreshes the exec factory definition. These are the host-editable inputs;
manual/bank journals, generated files, and statement caches remain Pond-owned.
Edit the YAML or chart while Wat is stopped, then restart it. Each run stages
`/accounting/`, opens `wat` interactively, and commits all saved changes together
only after a clean exit. Ctrl-C stops `wat` cleanly, allowing the runner to
commit changes already saved during the session.

Place statement PDFs in `local/wat-inbox`, set `WAT_STATEMENT_INBOX`, or select
another host directory explicitly:

```sh
./cmd/wat/run.sh --inbox ../caspar_water_bank
WAT_STATEMENT_INBOX=../caspar_water_bank ./cmd/wat/run.sh
```

The host inbox is mounted read-only. `wat` hashes regular PDFs, copies unseen
content into `/accounting/sources/statements/<sha256>/original.pdf`, and lists
the Pond-owned copy as pending. Identical content is imported once regardless
of filename. Symlinked and nonregular `.pdf` entries are rejected. No extraction
runs during startup. Choose **Bank statements** and select one
**READY TO EXTRACT** statement. `wat` extracts only that statement, applies
the complete original PDF through `bankstatementparser`, and opens the result
for review. A failed extraction returns to the statement list and leaves the
PDF pending; the inbox remains untouched.

The runner requires Docker (or set `WAT_CONTAINER_RUNTIME=podman`) and uses a
privileged development container because nested bubblewrap namespaces require
it. Its image supplies hledger, Go, Rust, Python 3.13, and the pinned statement
parser; these tools do not need to be installed on the host. The image build
checks that Debian's hledger package is from the required 1.52 series.
For Azure statement extraction, `--azure-cli` requires an authenticated `az`
CLI session with the Cognitive Services OpenAI User role. The runner obtains a
short-lived Entra token and passes it through the container and sandbox
environment. The token is not written to YAML, Pond, extraction results, or
command arguments. Restart the runner to obtain a new token if the session
remains open past token expiry. Non-Azure workflows omit `--azure-cli`.
The sandbox wrapper sets `WAT_STATEMENT_PYTHON=/usr/local/bin/python3.13`, so
a workspace migrated from the old direct runner may retain its host virtualenv
path in YAML without making that path meaningful inside the sandbox. Direct
invocations continue to use `statement.python` unless the same explicit
environment override is supplied.

For direct file-authoritative operation, use the private top-level `wat/`
workspace and follow its [operational guide](../../wat/README.md). Keep
`wat/wat.yaml`, journals, source documents, and review artifacts untracked.
The committed `wat/reference/cpuc-u39w.yaml` is the CPUC U-39-W master account
chart selected by the workspace-relative `chart_defs` list. The structured YAML
is reference data, not an hledger journal. Do not add vendors, banks, owners, or
other instance names to the master chart. No opening balances, purchase price,
or financial activity are fabricated.

```yaml
version: 3
name: Caspar Water
chart_defs: [reference/cpuc-u39w.yaml]
accounts:
  - Asset:151:Admin-Supplies
  - Expense:615:Electricity
  - Equity:218:Owner-Equity
bank_accounts:
  - Bank:131.2:SBMC:0000
```

The live books layout is fixed; the reference chart path is configured:

- `reference/cpuc-u39w.yaml` is the manually maintained CPUC reference chart.
- additional reference YAML files may be added to `chart_defs`; account numbers
  must remain unique across the merged definitions.
- `books/accounts.journal` is generated from `accounts` and `bank_accounts`.
- `books/books.journal` is the generated hledger include file.
- `books/manual.journal` receives ordinary entries.
- each bank account receives a derived all-years journal, such as
  `books/bank-sbmc-0000.journal`.
- `sources/statements` holds Wat-managed statement sources and caches.

Generated files carry a do-not-edit header and are refreshed when Wat opens the
workspace. The generated account declarations include `commodity $` and declare
the `Bank` hierarchy as hledger Asset type. Writable journals are created when
missing and remain ordinary plain-text hledger files. All workspace files and
directories must be nonsymlink paths. Unknown configuration keys and additional
YAML documents are errors.

An account must use `Category:CPUC-number:LocalName[:MoreNames...]`, and its CPUC
number and category must agree with the merged `chart_defs`. Every definition
path must be relative to the workspace, and CPUC numbers must be unique across
all files. `Bank` accounts must map to an Asset number. Their final component is
the unique statement identifier suffix: an extracted identifier ending in
`0000` binds to the configured account with that suffix. A missing or ambiguous match is an error;
Wat never guesses a journal. The bound account is stored in the private
extraction cache.

Views and reports cover all accounting dates by default. Report-period selection
belongs in the report workflow rather than persistent workspace configuration.
A statement may cross a calendar-year boundary.

## QuickBooks reconstruction

`wat quickbooks` generates an editable candidate journal from a directory of
QuickBooks `.xls`, `.xlsx`, and `.csv` reports without changing the books. The
spreadsheet rows are normalized with Pond, filtered to the requested year, and
processed one month at a time by the configured Azure or Ollama model. Existing
transactions after that year are supplied as account-classification examples.
The model must use exact configured Wat accounts and produce balanced entries;
Wat rejects invented accounts, unbalanced postings, malformed dates, duplicate
candidates, configured-bank postings without a cited cash-source row, incomplete
group coverage, reused group IDs, omitted split amounts, and non-JSON responses.
Every dated source row belongs to one immutable deterministic transaction
group, and every group must become exactly one balanced candidate or unresolved
item. Unresolved items preserve all known split postings and render as one
compact review comment, a `;` separator, and an uncommented pending transaction
template. Balanced virtual placeholder postings make the template valid hledger
syntax for text-editor highlighting without making it an accepted real entry.
A reviewer must replace the placeholders and remove the brackets before moving
the completed entry into the books.

The local runner scans its read-only inbox when no filenames are supplied:

```sh
./cmd/wat/run.sh --azure-cli --inbox ../caspar_water_bank \
  quickbooks --year 2021 >2021-draft.journal
```

Inspect the same inputs deterministically without configuring or calling a
model:

```sh
./cmd/wat/run.sh --inbox ../caspar_water_bank \
  quickbooks --inspect --year 2021
```

Inspection reports content hashes, normalized headers and row classes, date
coverage, annual counts, report sections, totals, and selected-year transaction
types. It also groups selected-year rows conservatively before any model call,
reports group counts by month and match basis, and lists every multi-row or
ambiguous group with its contributing source rows. Finally, it inventories
report sections and account, account-name, and distribution-account labels for
deterministic mapping review. It does not call a model or modify the books.

Review those source categories in Wat instead of deciding mappings in a prompt:

```sh
./cmd/wat/run.sh --inbox ../caspar_water_bank \
  quickbooks --review --year 2021
```

The review screen shows one canonical decision label per source row: the report
section for Account QuickReports, or the account label for register rows. Each
label starts unresolved and can map only to an existing configured posting
account. `F2` saves the policy to the private
`quickbooks-mappings.yaml` file in the Wat workspace. The editor never calls a
model and never changes source spreadsheets or journals.

Normal QuickBooks generation loads that mapping file. Human-approved mappings
are included as immutable model input, and Wat rejects a candidate or unresolved
skeleton that omits a mapped account. Remove a mapping in the review screen to
make that category unresolved again.

Pass filenames after the command to select a subset. Relative filenames are
resolved inside the configured inbox, which avoids exposing host paths inside
the accounting sandbox:

```sh
./cmd/wat/run.sh --azure-cli --inbox ../caspar_water_bank \
  quickbooks --year 2021 \
  'Caspar_Water_SMBC_Register.xls' \
  'Joshua MacDonald and Ariana Bayer_Account QuickReport.xlsx'
```

Progress and model warnings go to stderr; stdout contains only the candidate
draft, including unresolved virtual-posting templates. The command does not add
entries to `manual.journal` or a bank journal. Review and edit the generated file
before incorporating any transaction. Temporary source row identifiers appear
only in unresolved review comments, not in balanced candidate entries intended
for the replacement journal.

## CPUC chart and local posting accounts

The master chart stores structured CPUC accounts:

```yaml
version: 2
accounts:
  Asset:
    - name: Water-Plant-Purchased-or-Sold
      number: "104"
      comment: Optional operator-facing explainer text.

  Expense:
    - name: Electricity
      number: "615"
```

The formal account identity is displayed as
`Asset:104:Water-Plant-Purchased-or-Sold`. Category sections, number, name, and
optional single-line comment are validated strictly. Wat prints the comment with
account
listings and mapping choices.

Local posting accounts in `wat.yaml` use the shorter
`Category:CPUC-number:LocalName[:MoreNames...]` form. Wat materializes them as
these declarations in generated `books/accounts.journal`:

```journal
account Bank  ; type: A
account Bank:131.2:SBMC:0000
account Expense:650.1:Feiner-Fixings
account Equity:218:Owner
account Liability:241:Due-to-Owner
```

The ordinary categories are exactly `Asset`, `Equity`, `Liability`, `Expense`,
and `Income`. `Bank` is the special Asset category used for accounts that have
statements. It needs the hledger `type: A` declaration shown above so standard
balance-sheet reports classify it as an asset.

`wat` resolves the number in each local account through the configured chart.
`Expense:650.1:Feiner-Fixings`, for example, maps to formal CPUC designation
`Expense:650.1:Contract-Water-Quality-and-Testing-Work`. Unknown numbers,
category mismatches, and `Bank` accounts whose number is not an Asset are errors.
The long formal name is therefore not repeated in every posting while remaining
available whenever needed:

```sh
go run ./cmd/wat --config wat/wat.yaml accounts
```

Account completion and the F4 cheatsheet show only declared local accounts.
Completion can match either the local name or formal CPUC designation while
inserting only the short local account. The structured form, raw-text
validation, Save, and `wat check` enforce the same mapping. Existing invalid
postings do not prevent opening the app for migration, but every posting in a
revised entry must use a valid local account before it can be saved.

Before importing statements, a bank-account rename must update its declarations,
postings, and `bank_accounts[].account` setting together. Cached statements are
durably bound to the configured account; rebinding them after a rename is not
implemented. The `id`, `bank-row`, and `bank-description`
tags do not encode account names and should remain unchanged. The long values
identify a transaction, the source PDF/row, and the original bank description;
they are not account-name checksums.
Quit and restart `wat` after manual account/configuration renames. Open drafts
detect external journal changes and refuse to overwrite them.

## Work through a bank statement

For the Pond workflow, place PDFs in the configured inbox and run
`./cmd/wat/run.sh`. Choose **Bank statements (5)** to see both pending inbox PDFs
and previously extracted statements. The title reports extracted and ready
counts. Cached entries labeled **EXTRACTED** appear first, followed by pending
entries labeled **READY TO EXTRACT**, so a successful extraction remains visible
at the top of the list. Selecting **READY TO EXTRACT** opens the statement
immediately; no extraction questions or command-line statement flags are
required. While extraction is running, the screen animates and shows the
configured model and elapsed time.

To reopen an imported statement later, run `./cmd/wat/run.sh` without flags and
choose **Bank statements (5)**. Select a cached statement to review its pending
rows or revise its recorded entries. The list shows the bound bank account,
actual statement period,
journal-derived recorded/pending counts, and **RECONCILED** for finished statements.
Statement dates come from `wat_statement_period` metadata in the cached
extraction, never from the first/last transactions. The extraction model must
return the explicit statement-cycle start and end dates printed in the PDF;
`wat` validates their format, ordering, and containment of every transaction
before caching the result. Legacy caches without this metadata explicitly say
**Statement dates unavailable**. Known periods are ordered by start date;
legacy caches without dates use their earliest activity only for ordering, not
as a displayed statement period.
It uses the retained PDF and extraction:
the original input path, Python process, model service, and another `--statement`
run are not needed. Esc returns to the statement browser, then the main menu. Reopening does
not post entries or mark anything reconciled.

For a sequence, let the shell expand the glob, or supply explicit filenames:

```sh
./cmd/wat/run.sh --statements 2022/*.pdf
./cmd/wat/run.sh --statements 2022/01.pdf 2022/02.pdf
```

Files are processed in exactly the supplied argument order; `wat` neither
expands globs nor sorts the list. Relative paths use the invocation directory,
not the YAML directory. Quote individual filenames containing spaces, not
wildcards intended for shell expansion. At least one file is required, and
missing files or paths that are not regular PDFs are errors. Statements from
different years use the same workspace and account journal.

The terminal identifies the current file and its position in the batch. It
loads/extracts only one statement at a time. Record all rows, reconcile and
Finish the statement, then choose **Next statement (n)**, or
**Exit statement batch (n)** on the last file. Quit ends the whole session
without processing later files. Rerunning the same file list reuses cached
extractions, journal-derived row status, saved reconciliation work, and finished
checkpoints. The batch itself is not an atomic commit.

`--statements` and `--statement` are mutually exclusive. A reviewed extraction
seed requires the single-file `--statement` mode. `--statements 2022/*.pdf
--statement-check` checks every supplied file in order without opening the
terminal or posting.

The statement queue shows pending, recorded, and possible-match rows. Enter
opens a pending row in the existing entry form with its bank account, date,
description and opposite signed amounts filled in. Focus starts on the
counteraccount. Choose the account, split the counterposting when needed, and
press F2 to save. Acknowledge the save message to advance to the next pending
row. Esc / Back discards the current unsaved edits and returns to the queue;
it does not mark that row recorded. The main menu can resume the current statement
with **Continue bank statement (6)** or open any cached statement with
**Bank statements (5)**. Browsing another cache does not advance a command-line batch.
Enter on a recorded row reopens its existing entry for revision from that bank
account's journal; it never creates another posting.

Only Save writes journal postings. `bank-row` and `bank-description` metadata
identify the original source row and description. They survive payee edits and
must remain intact in F4. The original bank account, date and signed amount
are checked even for ledger-text edits; counterpostings remain editable.
Recorded status is read from the journals, not a second bookkeeping database.
The same PDF resumes its stable cached row order without extracting again.
Overlapping statements or manually recorded entries with the same bank date
and amount are flagged for explicit review, not automatically posted or skipped.

Suggestions reuse **unambiguous prior accepted counteraccounts** for the same
bank description (case/whitespace normalized), bank account and payment direction.
Conflicting assignments or multi-account splits do not become a one-account
rule. Unknown descriptions start with a blank counteraccount: this version does
not ask an LLM to invent account assignments. A deposit is not assumed to be income.

The source PDF and extracted JSON are kept privately under
`sources/statements/<PDF-SHA256>/`. Do not reorder or edit cached rows after
posting. Older caches can be given a `wat_statement_period` object with
`"start"` and `"end"` in `YYYY-MM-DD` format after checking the PDF's own
beginning/ending balance dates; adding this metadata does not change row
identities or journal postings. Every transaction must fall within the period.
Extraction notices and the statement period are available with **i** in the statement queue.
Matching opening/closing balances is an arithmetic check, not evidence that all
rows were extracted correctly or that the books are reconciled. Compare with
the original PDF before recording.

## Reconcile and finish a statement

After every row is recorded, choose **Reconcile statement (r)**. The reconciliation
uses every posting to the statement's bound bank account through the statement
closing date, including manual entries and not only transactions imported from
that PDF. It shows:

- statement and book opening balances;
- statement and book period credit/deposit and debit/payment totals;
- statement and book closing balances;
- outstanding deposits and payments shown separately; and
- the remaining unexplained difference.

Reconciliation cannot begin with an unexplained opening balance. For the first
statement, the book opening must equal the statement opening. For each later
statement, it must equal the prior finished checkpoint's book closing, while the
new statement opening must equal the prior statement closing. A mismatch appears
as **Reconciliation blocked** on the statement page and must be corrected in the
journal first; it cannot be classified as a current-period outstanding item.

Unmatched bank entries start as **UNEXPLAINED**. The reconciliation page is
diagnostic and does not edit or enter transactions. Return to
**Review / revise journal**, or quit and use a text editor, to correct the books;
restart `wat` after external edits. Use **o** on the reconciliation page only when
an existing entry is a genuine check or deposit still outstanding at the
statement date. An outstanding deposit increases the statement balance for
comparison; an outstanding payment decreases it. These are displayed separately
and net into **Adjusted statement**. The selection is saved in
`reconciliation-work.json`, so leaving the page or restarting `wat` does not lose
the work. Marking an item outstanding does not edit the journal, clear it, or
create a balancing transaction.

**Finish reconciliation** is allowed only when every statement row is recorded
and the unexplained difference is exactly zero. `wat` recalculates under its
journal and reconciliation locks, then writes a private
`reconciliation.json` checkpoint beside the cached PDF. The checkpoint records
the statement facts, cleared transaction IDs and semantic digests, outstanding
items, balances, and finish time. The statement then appears as
**RECONCILED — read-only**. Its cleared transactions cannot be revised from the
statement page, journal browser, structured form, or F4 path; the whole
transaction, including counterpostings, is protected.

Finished statements must be processed in chronological order independently for
each bank account, and each later opening balance must equal that account's prior
reconciled closing balance. An unfinished statement for one account does not
block another account. Outstanding
items remain unlocked because they have not cleared. A later statement may carry
them forward as outstanding; automatic matching of an old outstanding check to
a later bank row is not implemented yet.

On startup, `wat` verifies that each finished checkpoint still matches its
cached statement and that every locked journal transaction still exists
unchanged. Direct text edits cannot be physically prevented, but a deletion or
change is reported and the books refuse to open until the inconsistency is
resolved. There is intentionally no ordinary Edit or Reconcile action after
Finish. An explicit reopen/rollback workflow is not implemented.

For a new PDF, install the optional Python parser in a separate environment
(Python 3.13 is supported by the pinned hybrid dependencies):

```sh
python3.13 -m venv local/wat-python
local/wat-python/bin/python -m pip install -r cmd/wat/requirements-statement.txt
```

Configure the same workspace YAML; executable paths containing `/` are relative
to that YAML file, while bare executable names are looked up on PATH:

```yaml
statement:
  python: ../wat-python/bin/python
  api_base: https://example.openai.azure.com/
  api_version: 2025-04-01-preview
  model: azure/example-deployment
```

For a local Ollama model, use a provider-prefixed model and omit `api_version`:

```yaml
statement:
  python: ../wat-python/bin/python
  api_base: http://watershop:11434
  model: ollama/qwen3:30b-a3b
```

Existing Ollama workspaces may retain `ollama_url` and a bare model name as a
migration alias. New configurations use `api_base`; specifying both endpoint
fields is an error.

PDF extraction uses `bankstatementparser` with the configured text model.
Azure uses a short-lived `AZURE_AD_TOKEN`; Ollama uses its direct local HTTP
bridge. The overall extraction limit is 20 minutes, and extraction never posts
automatically.
`wat` asks `bankstatementparser` to parse all embedded text available in the
complete PDF. The extracted opening balance, transactions, and closing balance
must balance-verify. Missing text or a discrepancy is reported as an extraction
failure; `wat` does not select pages or switch to an image model.
Some statements present a dated zero-value beginning/ending balance as a row.
`wat` omits such rows from the posting queue and shows an extraction notice;
opening and closing balances remain part of mandatory arithmetic verification.

For scripted diagnostics, `--statement-check` extracts/validates and prints
queue counts without opening a terminal. `--statement-extraction reviewed.json`
can seed a new cache from a previously reviewed `bankstatementparser` JSON
export; the operator must ensure it belongs to the supplied PDF. Neither option
writes journal postings. Once cached, omit extraction-seed options so row
identities remain stable.

## Revise working entries

Choose **Review / revise journal (2)** and Enter on a working entry to reopen it
in the structured form. Change payee, description, reference, counteraccounts,
amounts, or split postings as appropriate, then **F2** saves the revision and
returns to the list. Esc / Back opens a discard confirmation for the unsaved revision.
Recorded statement rows can also be revised directly from their statement list.

The journal browser is a chronological one-row table. When bank accounts are
configured, **Bank change** and the running **Bank balance** are right-aligned
and computed independently for each account. With multiple accounts, the table
also identifies the affected bank account. All dated activity is shown and
contributes to the running balances. Up/Down selects a row, Enter revises it, and Esc or `q` returns
to the menu. These key instructions appear once in the footer rather than on
every transaction.

`books/manual.journal` and the derived bank journals are writable sections.
Transactions are revised in the same source journal: ordinary entries stay in
the manual journal, and statement-derived entries stay in their bank journal.
Entries in other included journals remain read-only.
Existing books are not migrated or changed just by opening this screen.
Deletion, period closing, and reopening or rolling back a finished reconciliation
are not implemented. A transaction becomes reconciled only through the explicit
Finish action described above.

A revision preserves the transaction's stable `id`, replaces only its original
source range, and leaves all bytes outside that range intact. It never appends
a duplicate transaction. Transaction-level notes and additional comment tags
are retained. IDs must be nonempty and unique across the included books.
Entries using features the structured form cannot preserve, such as posting
annotations, cleared/pending status, secondary dates, transaction codes,
prices, or balance assertions, remain read-only rather than losing information.
F4 is still available once an entry has been opened for revision.

For statement-linked entries, bank account, bank date, signed bank amount,
`bank-row`, `bank-description`, and `source` must stay unchanged. Revisions are
checked against the cached source row. Counteraccounts and splits remain
editable, and future suggestions read the revised assignments from the journals.
All revisions use the same strict whole-books validation, concurrent-change
checks, atomic replacement, and recovery-copy mechanism as new entries.

## Use the terminal

Select a menu item with arrows and Enter, or its numbered shortcut. In transaction
entry, Up/Down always move vertically between inputs, staying in the account or
amount column across posting rows, even with suggestions open. Tab/Shift-Tab
step through fields and buttons in order: account, amount, next account, next
amount. Moving away from a field never accepts a suggestion.

Payee and description suggest case-insensitive text matches from existing
transactions across all included journals, including earlier years. Suggestions
refresh when you start a new transaction; no separate history database is kept.
Account suggestions come from declared local accounts that resolve through the
configured CPUC chart. Start with digits to match the CPUC-number prefix (`13`
matches `131.2`, but not a `13` later in a local name); start with other text to
search the local account and its formal CPUC designation, ignoring case.

Use **Ctrl-N/Ctrl-P** to cycle suggestions and **Enter** to accept the highlighted
suggestion. On an account field with no filtered suggestions—including an
existing full account in the revision form—Enter opens the complete declared
account list without clearing the current value. Up/Down moves within that
explicit chooser and Enter accepts; Tab leaves the field. On other fields,
Enter without suggestions advances. You can also type complete values directly.
New posting accounts start blank rather than silently selecting the first chart
account.

Transactions accept any number of posting rows with account and amount columns,
each with a declared account and a signed USD amount:
positive is a debit, negative is a credit. Use `125.00` or `-125.00`, without a
currency symbol or grouping separators. The signed sum must be zero.
The live balance stays below the posting area and shows the signed amount still
needed: entering `-100` shows `+100.00 USD (debit)` to balance; adding `+40`
changes that to `+60.00`. Blank amounts are ignored while drafting, but invalid
amounts or overflow show an explicit unavailable-balance message rather than an
incorrect total. A balanced amount total alone does not mean the entry is valid.

Enter the actual transaction date in `YYYY-MM-DD` form. Enter the payee,
optional description, reference, and source
reference. Every saved entry
also receives a stable random `id` tag. Reference/source tags cannot contain
commas; fields cannot contain control characters. Payee/description cannot
contain journal delimiters `;` or `|`.

The action bar stays visible even when posting rows scroll. Its button order is
**Save (F2)**, **Add posting**, **Remove last posting**, then **Back (Esc)**, so
Tab from the last posting amount focuses Save in both new and revised entries.
**Esc** or **Back (Esc)** returns to the calling list or menu after confirming that unsaved work should be
discarded. Press **Esc again** to discard changes and leave the transaction page.
Choose **Keep editing** (the default button) to retain the draft.

**F2** or **Save (F2)** saves directly from the form, without a separate preview
step. Save validates the complete proposed books
with `hledger check --strict`, then appends a new entry or replaces the selected
working entry while preserving bytes outside it. Going back does not change the
journals. A repeated Save of the same draft cannot post it twice.

Press **F4** from the entry form or preview to edit the unsaved transaction's
Ledger text in an external editor. `wat` uses `$VISUAL`, then `$EDITOR`, then
`vi`. Quoted executable paths and arguments are supported, for example
`EDITOR='vim -f'`; shell expansions, pipelines, and redirections are not run.
Editors must wait until editing finishes (use `--wait` for editors that require
it).

The editor receives a private temporary file, never the live journal. F4 works
even before the form is complete: fill in the date, payee, and postings in the
editor. A commented cheatsheet below the draft shows posting syntax and the
current declared account names for copy/paste. Lines beginning `; wat-help: `
are temporary and removed before preview or save, even if the editor exits
unsuccessfully. Write your own comments without that reserved prefix to retain
them. Edit the transaction above the cheatsheet, not within it.

Keep the generated `id` tag unchanged. Only one transaction with a valid date
and regular `$` postings is accepted; journal directives are not allowed.
hledger parses the edited text and validates it against the complete books.
Closing the editor returns to an exact-text preview, not an automatic save.
Invalid edits can be reopened without losing their text.

After editing text, use F4 again to revise it or Save to post it. Returning to
the structured form requires explicit confirmation to discard the text edits;
the tool does not round-trip or silently simplify manually edited journal text.
The temporary file is removed after the editor exits.

```sh
go run ./cmd/wat --config wat/wat.yaml check
go run ./cmd/wat --config wat/wat.yaml report journal
go run ./cmd/wat --config wat/wat.yaml report trial-balance
go run ./cmd/wat --config wat/wat.yaml report balance-sheet
go run ./cmd/wat --config wat/wat.yaml report income-statement
```

Reports cover all dates currently in the books. The balance sheet uses hledger's
`balancesheetequity`, showing assets, liabilities, and equity. Its Net line
reflects unclosed income/expense balances; displaying the report does not post a
closing entry to equity.
The journals remain usable directly:

```sh
hledger -f wat/books/books.journal
hledger-ui -f wat/books/books.journal
```

The structured revision workflow handles supported working entries. For
arbitrary manual journal changes, quit `wat`, use your editor, and run `wat check`
before reopening the workspace.
Reports are hledger's presentation, not a second Go accounting interpretation.
No entire journal is round-tripped through a parser or reformatted.

## File safety and recovery

Save validates private temporary copies, retains a synced `.wat-backup-*` recovery
copy beside the writable journal, and atomically replaces the journal. Recovery
copies are not included by the books' explicit includes; retain them according
to your backup policy. Local recovery copies are not off-device backups.

A draft snapshots every included journal. Changes to their contents, permissions,
or include set cause Save to fail: cancel and reopen the transaction. Cooperative
`wat` writers use an exclusive `<target-journal>.wat-lock` file during save.
After a crash, inspect the journal and ensure no other writer is active before
removing a stale lock.

External editors do not honor that lock. Changes are rechecked immediately before
replacement, but there is no cross-program filesystem compare-and-swap: do not
edit the books concurrently with a save.

Passing `wat check` establishes syntax, declared accounts/commodities, posting
account depth, balance
assertions, and balanced entries, not correct classification, completeness,
regulatory compliance, or a new bank reconciliation. Existing reconciliation
checkpoint integrity is verified when the books open. There is no implicit
balancing adjustment or year-close operation.
