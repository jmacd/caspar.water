# Water Accounting Terminal

`wat` is a local terminal workbench for ordinary hledger journals. It supports
transaction entry, journal review, strict validation, bank-statement
reconciliation checkpoints, and reports. It does not calculate bills or close years.
The application uses ordinary local files and has no database, server, or
browser. Customer and service-account workflows do not require a Watertown Pond.
Keep the private filesystem workspace authoritative.

## Start a direct-file workspace

Run from the application checkout with Go and hledger 1.52 available on `PATH`.
Use an existing, operator-reviewed `wat.yaml` and workspace-relative chart
definitions. For the private top-level `wat/` workspace:

```sh
go run ./cmd/wat --config wat/wat.yaml check
go run ./cmd/wat --config wat/wat.yaml report journal
go run ./cmd/wat --config wat/wat.yaml
```

Use `--config /absolute/path/to/wat.yaml` for a workspace elsewhere. Startup
regenerates account/include declarations and creates missing writable journals
as empty files; it does not reconstruct missing history. Before opening a
restored workspace, verify that all authoritative files were restored.
Edit configuration or reference charts only while Wat is stopped.

Saved journal and registry steps take effect immediately in those ordinary
files; exiting the TUI is not an additional commit or rollback boundary. There
is no startup import/export cycle or copy-back step. The former
`cmd/wat/run.sh` Pond prototype is not shipped in this checkout and is not the
operational path. Watertown-backed execution remains a separate integration.

To import statement PDFs from an existing read-only source inbox:

```sh
go run ./cmd/wat --config wat/wat.yaml --statement-inbox ../caspar_water_bank
```

Wat treats the inbox as read-only; it does not require a container mount.
It hashes regular PDFs, copies unseen content into the workspace's
`sources/statements/<sha256>/original.pdf`, and lists the local copy as pending.
Identical content is imported once regardless of filename. Symlinked and
nonregular `.pdf` entries are rejected. No extraction
runs during startup. Choose **Bank statements** and select one
**READY TO EXTRACT** statement. `wat` extracts only that statement, applies
the complete original PDF through `bankstatementparser`, and opens the result
for review. A failed extraction returns to the statement list and leaves the
PDF pending; the inbox remains untouched.

Customer setup, manual journal entry and reports do not need model
credentials or a statement parser. New PDF extraction uses the configured
`statement.python` and the dependencies in `requirements-statement.txt`, plus
the configured model service. `WAT_STATEMENT_PYTHON` explicitly overrides the
configured Python executable. For Azure, supply a short-lived `AZURE_AD_TOKEN`
through the environment using the operator's credential setup; never put tokens
in workspace files or Git. `--azure-cli` and `--inbox` were wrapper options,
not Wat CLI flags.

Keep `wat/wat.yaml`, journals, source documents, and review artifacts out of this
public repository; use independent private version control and backups.
The workspace's `wat/reference/cpuc-u39w.yaml` is the CPUC U-39-W master account
chart selected by the workspace-relative `chart_defs` list. The structured YAML
is reference data, not an hledger journal. Do not add vendors, banks, owners, or
other instance names to the master chart. No opening balances, purchase price,
or financial activity are fabricated.

```yaml
version: 3
name: Caspar Water
chart_defs: [reference/cpuc-u39w.yaml]
manual_journal: books/manual-2022.journal
historical_journals:
  - books/manual-2021.journal
accounts:
  - Asset:151.000:Admin-Supplies
  - Expense:615.000:Electricity
  - Equity:218.000:Owner-Equity
bank_accounts:
  - Bank:131.200:SBMC:0000
```

The live books layout is fixed; the reference chart path is configured:

- `reference/cpuc-u39w.yaml` is the manually maintained CPUC reference chart.
- additional reference YAML files may be added to `chart_defs`; account numbers
  must remain unique across the merged definitions.
- `books/accounts.journal` is generated from `accounts`, `bank_accounts`, and
  enabled registry-owned service accounts.
- `books/payees.journal` is an ordinary hledger journal containing native
  `payee NAME` directives for manually maintained counterparties. It may include
  additional `.journal` files using hledger's `include` directive.
- `books/customers.journal`, when the customer registry is enabled, is generated
  from customer names in `registry.yaml`. Do not edit it or duplicate those
  names in `books/payees.journal`.
- `books/books.journal` is the generated hledger include file.
- `manual_journal` names the active writable journal that receives ordinary
  entries; it defaults to `books/manual.journal`.
- `historical_journals` names fixed, read-only journals included in every check
  and report. At year rollover, move the previous `manual_journal` path into
  this list and configure a new active yearly file.
- `wat check` runs hledger's native `payees` check, so every transaction payee
  must be declared with a `payee` directive. Descriptions use native hledger
  syntax: a pipe and memo are optional, and without a pipe the entire
  description is the payee. Existing entries do not need to be rewritten as
  `PAYEE | MEMO`. `Journal` and `Deposit` are ordinary declared payees; neither
  requires special metadata. Wat additionally requires `Deposit` to have a
  `Bank:` posting and no `Expense:` posting.
- each bank account receives a derived all-years journal, such as
  `books/bank-sbmc-0000.journal`.
- `sources/statements` holds Wat-managed statement sources and caches.
- `registry.yaml`, when enabled, holds customer/connection/service metadata.
- `sources/customer-statements` holds immutable issued closing statements.

Generated files carry a do-not-edit header and are refreshed when Wat opens the
workspace. The generated account declarations include `commodity $` and declare
the `Bank` hierarchy as hledger Asset type. Writable journals are created when
missing and remain ordinary plain-text hledger files. All workspace files and
directories must be nonsymlink paths. Unknown configuration keys and additional
YAML documents are errors.

An account must use `Category:xxx.yyy:LocalName[:MoreNames...]`. `xxx` is the
three-digit CPUC family and `yyy` combines one formal branch digit with a
two-digit local number. For a CPUC leaf such as `615`, local codes `615.000`
through `615.099` map to that leaf. When the chart defines formal branches such
as `131.1` and `131.2`, local codes must use the matching ranges `131.100`-
`131.199` or `131.200`-`131.299`; the `.0yy` range is not allowed for that
family. An unused branch digit from `.4xx` through `.9xx` is a local extension
of the CPUC parent when that family has formal subaccounts. Exact formal
branches always win: if `265.4` is formal, `265.4yy` maps to it, while an unused
`265.5yy` maps to parent `265`. Undefined `.1xx`-`.3xx` branches remain errors,
and a CPUC leaf without formal subaccounts continues to use only `.0yy`. The
resolved CPUC category must agree with the posting category.
Every definition path must be relative to the workspace, and CPUC numbers must
be unique across all files. `Bank` accounts must resolve to an Asset account.
Their final component is the unique statement identifier suffix: an extracted
identifier ending in `0000` binds to the configured account with that suffix.
A missing or ambiguous match is an error; Wat never guesses a journal. The
bound account is stored in the private extraction cache.

Views and reports cover all accounting dates by default. Report-period selection
belongs in the report workflow rather than persistent workspace configuration.
A statement may cross a calendar-year boundary.

The first time you open a statement, Wat uses the configured statement model to
suggest accounts for all unmatched rows in one request, before transaction
review begins. The request includes those bank rows, their suggested payees,
declared payees, configured nonbank accounts, and accepted same-bank,
same-direction journal description/account examples. This recognizes vendor
aliases and descriptions that include the payment amount without changing the
original bank evidence. Listing statements does not make these requests, and
workspaces without a configured statement model remain manual.

Validated suggestions and abstentions are saved in the statement's private
`account-suggestions.json` sidecar, together with the evidence used for inference.
Reopening the statement reuses this snapshot, and entering or advancing through
transactions never calls the model. Existing exact-match defaults and check
image splits take precedence over cached account suggestions.

Use **Re-run AI suggestions** (`a`) on an unfinished statement to read its check
images again, then refresh account suggestions. This retries previously
unrecognized payees and empty suggestions without re-extracting transactions
or modifying saved entries. Bare numeric withdrawal descriptions such as
`1013`, as well as labeled checks, are candidates for image inspection; the
image must establish the payee. Differences in a caption's formatting or date
do not prevent inspection.

Statement references are retained in the entry's Reference field. When the
extraction has no separate reference, recognizable check numbers are taken
from the bank description (bare numbers, labeled checks, or check captions).
Revising an older statement entry with a blank Reference also offers that
default; an existing saved reference is preserved. Changes are written only
when you Save.

Check numbers never establish vendor identity or an account assignment, even
when they are equal or sequential. Rows and historical payees containing only
check references are excluded from account learning; an identified payee from
a check image can still support a suggestion. Older account-suggestion caches
are regenerated when the statement opens under this rule.

An AI account suggestion must cite a single-account journal example and use an
exact configured account. Ambiguous or split-account history should produce no
default. The entry form labels the suggestion and its explanation; all fields
remain editable, and only Save records an entry. Model failures are displayed
explicitly and allow manual entry. Esc cancels an in-progress request and opens
the statement for manual review.

## Customers and service accounts (direct files)

Customer workflows do not require a Pond. Their authoritative metadata lives in
one private `registry.yaml`, alongside `wat.yaml`. It contains versioned YAML
lists named `customers`, `connections`, and `service_accounts`; monetary balances
remain exclusively in the hledger journals.

Initialize the file, then separately enable it in your workspace configuration:

```sh
go run ./cmd/wat --config wat/wat.yaml customer init
```

```yaml
customer_registry: true
```

Initialization refuses to overwrite a registry and does not change configuration
or journals. Once enabled, a missing, malformed, or symlinked registry is an
error, not an empty customer list. Keep the registry in the private books'
version-control and backup boundary, never the public application repository.

Run `customer init` only once. If `registry.yaml` already exists and
`customer_registry: true` is enabled, open the existing customer menu instead:

```sh
go run ./cmd/wat customer --config wat/wat.yaml
```

A **customer** is the responsible party. Its name and billing address are
maintained in `registry.yaml`; Wat generates native hledger payee declarations
in `books/customers.journal` and includes that file automatically. Names are
generated for all retained customers, not just those with open service.
A **connection** is the physical service point. A **service account** links them
for an explicit date range. IDs are stable (`C-0001`, `CN-0001`, `SA-0001`);
changing a mailing address does not rename ledger accounts. One customer may
have several connections. Responsibility periods on one connection cannot
overlap; `service_through` is inclusive. Historical records remain in the
registry. Closing ends responsibility, not the associated debt or refund
obligation. The next occupant always receives a new service account.

Create the records directly; no separate customer payee declaration is needed:

```sh
wat customer add --payee "Example Customer" \
  --address "100 Example Street" --address "Exampletown, CA 00000"
wat connection add --name "Example house" --address "100 Example Street"
wat service open --customer C-0001 --connection CN-0001 --from 2026-07-01
```

These examples assume `wat` is installed and the workspace is the current
directory; otherwise use `go run ./cmd/wat --config wat/wat.yaml` as above.
The `--payee` flag sets the customer's transaction name in the registry.
Use `customer list`, `connection list`, and `service list` to look up IDs.
`customer edit ID --address ...` updates mailing information; `connection edit`
updates the name or service address. Names must match exactly and unambiguously,
or select the record by ID.
Changing a customer name updates its generated declaration, not historical
transaction descriptions. Old payee names still used by transactions must
remain declared.

Opening service normally creates `Asset:141.000:Customers:SA-0001`. Use
`--receivable ACCOUNT` only to explicitly adopt an appropriate existing `141`
account after reviewing its history. Optional `--credit ACCOUNT` and
`--deposit ACCOUNT` select approved liability accounts; the latter must map to
CPUC `233`. These options do not choose a prepayment policy or post money.
Distinct service accounts cannot own overlapping ledger account subtrees.
Declarations are generated from configured accounts and the registry together;
do not edit `books/accounts.journal`.

In the TUI, choose **Customers / service accounts (c)** for setup, edits, and
reports. Address inputs use semicolons to separate address lines. Each F2 saves
one authoritative file immediately. Cancelling a later step does not undo an
earlier registry save. Registry saves refresh the derived account and customer
declarations without editing `books/payees.journal`. Missing or outdated
generated declarations are also rebuilt when Wat opens the workspace;
if that refresh fails, Wat reports that metadata was saved and blocks further
bookkeeping until restart successfully regenerates the declarations. There is
no multi-file commit or rollback mechanism. Use one Wat session at a time and
do not edit the workspace concurrently.

Registry and native payee-declaration saves validate their metadata and journal
structure without requiring cleanup of unrelated historical descriptions or
undeclared transaction payees. Service-account changes still validate affected
service activity and billing evidence. `wat check` remains the explicit full-book
audit and continues to report historical issues; setup never rewrites those
transactions.

### Allocate an existing deposited payment

Open the original bank-statement row or working journal entry. Focus its
**nonbank** posting row and press **F3** to choose a service account and accounting
role. This changes only the account; review the amount and classification before
F2. Account completion also searches customer names, connection addresses, and
service IDs. Existing split-posting controls support combined receipts.

For a simple recorded receipt, the CLI provides a preview-first equivalent:

```sh
wat service allocate TRANSACTION-ID --to SA-0001=100.00
wat service allocate TRANSACTION-ID --to SA-0001=100.00 --save
# A combined deposit:
wat service allocate TRANSACTION-ID --to SA-0001=60.00 --to SA-0002=40.00
```

The default role is receivable; explicitly select an approved liability with
`--to SA-0001:credit=100.00` or `--to SA-0001:deposit=100.00`. Every allocation
must be positive and the sum must equal the original bank receipt. This command
replaces the receipt's credit counterpostings, not its bank posting, and previews
the entire revised transaction. Receipts containing debit-side fees or other
complex splits must use the transaction editor. The original transaction ID,
statement evidence, and journal location are retained; cash is not posted twice.
Finished reconciliations stay locked and require the existing explicit reopen
workflow before revision.

Do not treat a bank deposit as automatically earned revenue or a security
deposit. Review existing charges before crediting receivables; use an explicitly
approved liability treatment for advances. No opening balance or missing charge
is invented by customer setup or allocation.

### Customer statements

```sh
wat service statement SA-0001 --from 2026-07-01 --as-of 2026-11-16
wat customer show C-0001 --as-of 2026-11-16
wat connection show CN-0001 --as-of 2026-11-16
```

Reports are read-only text, using hledger's opening balances, dated registers,
and closing balances. The end date is inclusive; the default start is the
service start (or the as-of date for service not yet begun). Earlier postings
remain in the opening balance. Customer/connection reports keep each service
account and liability separate, rather than silently transferring or netting
balances. A credit balance needs review; neither a zero balance nor this report
certifies complete billing or final settlement. Use the explicit lifecycle steps
below for closing statements; PDF output remains deferred.

### Close service and review settlement

```sh
wat service close SA-0001 --through 2026-11-16
wat service settle SA-0001 --as-of 2026-11-16
```

`close` saves only the inclusive end date in `registry.yaml`. It neither zeros
balances nor transfers them to a new occupant. A new service account at that
connection may start the following day. The application does not reopen closed
service, change a saved end date, or reassign its customer/ledger accounts.
`settle` is a read-only review, not a financial posting or a stored status flag.

Save any reviewed final charge or credit as a separate journal step:

```sh
wat service adjust SA-0001 --date 2026-11-17 --through 2026-11-16 \
  --amount 25.00 --account Income:460.100:Service \
  --reference "Approved final-charge calculation and service coverage"
# Inspect the preview, then repeat with --save.
```

Amounts and counteraccounts are explicit operator decisions: positive amounts
increase receivables, negative amounts credit them. Wat does not calculate
proration, interest, vacancy responsibility, or write-offs. The example income
account must already be declared and appropriate to the approved treatment.
The adjustment helper cannot create bank movements or transfer customer debt.

Later-posted noncash charges/adjustments require a transaction-level
`; service-through: YYYY-MM-DD` tag within the closed service period. The helper
adds it; the ordinary ledger editor can also supply it. Unsupported postclosure
charges are rejected, including when attempting to close an account with such
entries already present. Later receipts, actual refunds, and applications
between that service account's own roles remain possible. Customer postings
must use exact USD regular postings without commodity costs or separate posting
dates.

After reviewing all service coverage and saved financial activity, separately
confirm billing completion:

```sh
wat service confirm-billing SA-0001 --through 2026-11-16 --date 2026-11-17 \
  --review "Reviewed full service coverage, final charge and approved policy evidence"
wat service settle SA-0001 --as-of 2026-11-17
```

This registry save records the covered end date, review date and explanation,
plus IDs and SHA-256 digests of the normalized, already-saved transactions
touching the account. Every related transaction must have a unique ID.
Confirmation cannot predate closure or recorded account activity. Zero billing
also requires explicit review; zero balances alone never prove completeness.

The initial implementation deliberately invalidates confirmation after **any**
added, removed, or revised related transaction, including payments and refunds.
Reconfirm after separately saving and reviewing those steps. This conservative
rule avoids inferring which changes affect final billing. It fingerprints ledger
transactions, not the contents of externally referenced source documents.
Settlement is derived only when service has ended, confirmation is valid as of
the requested date, and **each** configured receivable/credit/deposit balance is
zero. Opposite balances in different roles are not silently netted.

### Apply liabilities and allocate an actual refund

An application needs an explicitly configured liability role, an approved
reference, sufficient liability, and sufficient outstanding receivable:

```sh
wat service apply SA-0001 --role deposit --amount 20.00 --date 2026-11-18 \
  --reference "Approved deposit application; policy/evidence reference"
# Review, then repeat with --save. Use --role credit for an approved credit liability.
```

This posts a debit to that liability and a credit to the same service account's
receivable. It creates no cash entry. Each application is a new transaction;
do not repeat `--save` unless another application is intended.

A refund due is not a disbursement. First record the actual bank transaction
through the existing bank-statement/manual workflow, then allocate that recorded
payment:

```sh
wat service refund SA-0001 --transaction EXISTING-DISBURSEMENT-ID --role deposit
# Review, then repeat with --save.
```

The refund helper revises a simple disbursement with one negative bank posting;
it preserves the transaction ID, amount/date, journal location and statement
evidence. It does not append another bank movement, and refuses amounts exceeding
the selected refundable balance. Select `credit` for a credit liability or
`receivable` for an explicitly reviewed receivable credit. Use the existing
transaction editor for complex splits. Finished reconciliations remain locked
until explicitly reopened. Reconfirm final billing after saved activity.

### Issue a closing statement

```sh
wat service statement SA-0001 --as-of 2026-11-18 --issue
```

Issuance requires closed service and valid billing confirmation; debt or a refund
may still be outstanding and is shown explicitly. It writes a complete,
read-only text artifact to
`sources/customer-statements/SA-0001/statement-0001.txt`. Subsequent issues use
new numbered files and never overwrite an earlier issue. Include this private
directory in backups and version control.

Each issue snapshots identities and addresses, service/report/issue dates,
opening and closing balances, recorded activity, derived settlement status,
billing review, and transaction/source references. Publication happens only
after the complete file is prepared and journal/registry inputs are rechecked.
It changes neither journals nor registry. Later address corrections and ledger
activity cannot alter issued files. The application will not open a viewer.

The TUI service-account detail menu exposes closure, billing confirmation,
settlement review, reviewed adjustments, liability applications and actual-refund
allocation. Financial steps require a journal preview before Save. The
**Statement / history** form has **Issue closing statement**; leave From blank
to include the complete service history. Registry saves, journal saves and
document issuance remain separate actions, with no all-or-nothing transaction
claim. These tools enforce bookkeeping constraints, not tariff approval.

### Private rollout, checkpoints and restore

Customer workflows and failure cases are covered by automated unit and
integration tests; a manual rehearsal or private-copy pilot is not required.
Use the existing direct-file installation with operator-managed private Git
checkpoints and review saved changes there. Initializing a registry does not import old
customer identities or reconstruct balances. Review the customer, connection,
service dates and any adopted `141`/liability accounts yourself. Do not run
another `customer init` over an existing registry or fabricate opening entries
to make an adoption balance. Workspace copies are an optional recovery aid, not
a prerequisite for using the customer commands.

Stop Wat and other workspace writers before making a backup or Git checkpoint.
A closed account with debt, an unapplied deposit, or a stale billing
confirmation is a valid intermediate state; preserve it rather than changing
money to make a checkpoint look complete. Back up the **whole workspace**:
configuration, every configured chart, native payee declarations and includes,
all manual/historical/bank journals, the registry, mappings/review artifacts,
and the complete `sources/` tree. Bank PDFs, extractions and reconciliation
checkpoints are needed together. Issued statements are historical artifacts;
do not discard them as regenerable reports.

Keep original source systems/inboxes backed up independently. A private Git
checkpoint is useful history but does not include ignored/untracked files or
preserve all file permissions. It is not a substitute for a complete private
backup. Keep backups outside the live workspace and outside this public repo.

For example, with existing private parent directories, replace these paths and
choose fresh, nonsymlink destinations. These copies preserve file modes and
include hidden files; they do not merge a backup into an existing workspace:

```sh
umask 077
test ! -e /private/backups/wat-0001 &&
  test ! -L /private/backups/wat-0001 &&
  cp -pR /private/books/wat /private/backups/wat-0001
test ! -e /private/restores/wat-0001 &&
  test ! -L /private/restores/wat-0001 &&
  cp -pR /private/backups/wat-0001 /private/restores/wat-0001
```

Verify the restored file inventory and contents against the stopped snapshot
**before** starting Wat. Missing writable journals would otherwise be recreated
empty. Use a new directory, retain the original, and never mix registry,
journals or checkpoints from different snapshots. Only `books/accounts.journal`
and `books/books.journal` are regenerated declarations; other files are not
reconstructed at startup.

From the application checkout, open the copy explicitly:

```sh
go run ./cmd/wat --config /private/restores/wat-0001/wat.yaml check
go run ./cmd/wat --config /private/restores/wat-0001/wat.yaml customer list
go run ./cmd/wat --config /private/restores/wat-0001/wat.yaml service settle SA-0001 --as-of 2026-11-18
go run ./cmd/wat --config /private/restores/wat-0001/wat.yaml service statement SA-0001 --as-of 2026-11-18
hledger --no-conf -f /private/restores/wat-0001/books/books.journal balance
```

Use the same account IDs and as-of dates as the pre-backup comparison. Confirm
that registry identities, separate obligation balances, billing confirmation,
bank reconciliation locks and issued statement bytes match. Do not use
`--issue` just to compare an existing statement: it intentionally creates
another numbered artifact. File-relative journal includes and evidence IDs
survive moving the workspace; review host-specific Python/model paths separately
before new extraction. A successful `check` is not proof of completeness,
reconciliation, or an approved tariff.

## QuickBooks reconstruction

`wat quickbooks` generates an editable candidate journal from a directory of
QuickBooks `.xls`, `.xlsx`, and `.csv` reports without changing the books. The
spreadsheet rows are normalized with Pond, filtered to the requested year, and
processed one month at a time by the configured Azure or Ollama model. Existing
transactions after that year are supplied as account-classification examples.
The model must use exact configured Wat accounts and produce balanced entries;
Wat rejects invented accounts, unbalanced postings, malformed dates, duplicate
candidates that post to a configured bank without a cited cash-source row,
incomplete group coverage, reused group IDs, omitted split amounts, and non-JSON
responses. Every dated source row belongs to one immutable deterministic
transaction group, and every group must become exactly one balanced candidate
or unresolved item. Ordinary unresolved items preserve signed account-side
postings and use balanced virtual placeholders for an editable hledger
template. When one transaction group spans multiple non-cash report files, Wat
marks that its source amounts may overlap as aggregate and component views,
renders only unbalanced `(TODO:Source-Magnitude:...)` evidence, and refuses to
manufacture a counterposting. A reviewer must replace all placeholders with
complete supported postings before moving an entry into the books.

The CLI scans its read-only inbox when no filenames are supplied. Supply any
required model credentials through the environment:

```sh
go run ./cmd/wat --config wat/wat.yaml --statement-inbox ../caspar_water_bank \
  quickbooks --year 2021 >2021-draft.journal
```

Inspect the same inputs deterministically without configuring or calling a
model:

```sh
go run ./cmd/wat --config wat/wat.yaml --statement-inbox ../caspar_water_bank \
  quickbooks --inspect --year 2021
```

Inspection reports content hashes, normalized headers and row classes, date
coverage, annual counts, report sections, totals, and selected-year transaction
types. It also groups selected-year rows conservatively before any model call,
reports group counts by month and match basis, counts groups whose cross-report
source amounts may overlap, and lists every multi-row group with its
classification and contributing source rows. Finally, it inventories report
sections and account, account-name, and distribution-account labels for
deterministic mapping review. It does not call a model or modify the books.

Review those source categories in Wat instead of deciding mappings in a prompt:

```sh
go run ./cmd/wat --config wat/wat.yaml --statement-inbox ../caspar_water_bank \
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
resolved inside the configured inbox:

```sh
go run ./cmd/wat --config wat/wat.yaml --statement-inbox ../caspar_water_bank \
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

Local posting accounts in `wat.yaml` use
`Category:xxx.yyy:LocalName[:MoreNames...]`. Wat materializes them as these
declarations in generated `books/accounts.journal`:

```journal
account Bank  ; type: A
account Bank:131.200:SBMC:0000
account Expense:650.100:Feiner-Fixings
account Equity:218.000:Owner
account Liability:241.000:Due-to-Owner
```

The ordinary categories are exactly `Asset`, `Equity`, `Liability`, `Expense`,
and `Income`. `Bank` is the special Asset category used for accounts that have
statements. It needs the hledger `type: A` declaration shown above so standard
balance-sheet reports classify it as an asset.

`wat` resolves the number in each local account through the configured chart.
`Expense:650.100:Feiner-Fixings`, for example, maps to formal CPUC designation
`Expense:650.1:Contract-Water-Quality-and-Testing-Work`. Unknown numbers,
unsupported low branch digits, `.0yy` codes on a family with formal subaccounts,
category mismatches, and `Bank` accounts that do not resolve to an Asset are
errors. A high branch already defined by CPUC remains formal; only unused
`.4xx`-`.9xx` branches are local extensions of a CPUC parent that has formal
subaccounts. The long formal name is therefore not repeated in every posting
while remaining available whenever needed:

```sh
go run ./cmd/wat --config wat/wat.yaml accounts
```

The command prints aligned `LOCAL ACCOUNT` and `CPUC TITLE` columns. Formal chart
names are presented as human titles, replacing YAML hyphens with spaces; for
example, `Water-Plant-Purchased-or-Sold` is displayed as
`Water Plant Purchased or Sold`. An optional chart comment appears on the next
line under the CPUC title, with the local-account column blank.

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

Run `go run ./cmd/wat --config wat/wat.yaml --statement-inbox PATH` to import
PDFs from a read-only inbox. Choose **Bank statements (5)** to see pending PDFs
and previously extracted statements. The title reports extracted and ready
counts. Cached entries labeled **EXTRACTED** appear first, followed by pending
entries labeled **READY TO EXTRACT**, so a successful extraction remains visible
at the top of the list. Selecting **READY TO EXTRACT** opens the statement
immediately; no extraction questions or command-line statement flags are
required. While extraction is running, the screen animates and shows the
configured model and elapsed time.

To reopen an imported statement later, run `go run ./cmd/wat --config wat/wat.yaml` and
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
go run ./cmd/wat --config wat/wat.yaml --statements 2022/*.pdf
go run ./cmd/wat --config wat/wat.yaml --statements 2022/01.pdf 2022/02.pdf
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
resolved.

To undo a premature Finish while the books still open normally, select the
finished statement and choose **Reopen reconciliation (u)**. Wat removes the
checkpoint while retaining every recorded journal entry and restoring saved
outstanding selections. The entries become editable and the statement can be
reconciled and finished again. Reopening an earlier statement also reopens all
later finished statements for the same bank account because their opening
balances depend on the earlier checkpoint.

If normal startup is blocked because a reconciled transaction or statement was
already edited, use the transaction ID printed in the error:

```sh
./wat/run.sh reopen-reconciliation 33a486a16f493c7cf76b7c6500da2924
```

The selector may also be a unique statement-hash prefix, period start/end date,
or `START..END`. This recovery command intentionally loads only workspace
configuration and checkpoint metadata, bypassing normal reconciliation
verification. It does not modify or remove journal entries.

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
  transaction_prompts:
    - payee: Example Vendor
      prompt: |
        Use the visible check memo to propose an editable transaction memo and
        split the total between the appropriate declared expense accounts.
```

Each `transaction_prompts` rule selects one exact hledger-declared payee. The
prompt is sent only as context for that matched payee and may ask the model to
propose a memo and multiple counterpostings. Suggested accounts must be declared
non-bank accounts, amounts must be positive exact USD values, and the split must
balance exactly to the payment. These are nondeterministic editable defaults:
nothing is written until the operator reviews the form and presses F2. Changing
a rule invalidates the corresponding cached AI enrichment and reruns it when
the statement is next opened.

Pass an optional read-only QuickBooks bank-register export only for the Wat run
that should use it:

```sh
go run ./cmd/wat --config wat/wat.yaml \
  --quickbooks-register ../caspar_water_bank/Bank_Register.xls
```

The CLI path is relative to the current directory. For a pending statement row,
Wat uses only a unique match with the same date and signed payment/deposit
amount. Check/reference or description evidence resolves otherwise-duplicate
matches; unresolved ambiguity is shown and leaves the form blank. An exact
match prefills its payee, memo, and reference. Its QuickBooks account prefills a
counteraccount only after that label has been approved with
`quickbooks --review`; unmapped labels remain explicit in the statement row
hint. The operator must still review the form and press F2. Spreadsheet
registers require the paired `pond` CLI through `WAT_POND`; CSV registers do
not.

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

PDF extraction uses `bankstatementparser` with the configured model.
Azure uses a short-lived `AZURE_AD_TOKEN`; Ollama uses its direct local HTTP
bridge. Text extraction has a 20-minute limit; check-image extraction has a
separate 12-minute limit. Extraction never posts automatically.
`wat` asks `bankstatementparser` to parse all embedded text available in the
complete PDF. The extracted opening balance, transactions, and closing balance
must balance-verify. Missing text or a discrepancy is reported as an extraction
failure.

When an extracted debit row contains the word `CHECK`, or has the exact
`NUMBER MM/DD/YYYY $AMOUNT` form with a date and amount matching the row, Wat
automatically sends the statement's rendered page images and deduplicated
embedded images to the same configured model and asks for the exact name written
on the check's payee line. Embedded images are upscaled before transmission so
check text remains legible. Wat also supplies the canonical names from
`hledger payees --declared` as spelling hints. The model must still use visible
check evidence and may return a new name when no declaration matches. This
happens during new
statement extraction and on the first open of an older cached statement, with
no separate action. The validated suggestions are stored separately in
`check-payees.json`, tied to the statement fingerprint and row numbers, and
prefill only editable fields. For an exact configured payee, the sidecar may
also hold a validated memo and balanced split proposed from its
`transaction_prompts` rule. They do not alter extracted statement rows,
reconciliation evidence, payee declarations, or journals. An exact QuickBooks
payee takes precedence and suppresses a split for a different payee; a nonempty
QuickBooks memo takes precedence, while a matched AI split remains editable.
The progress screen identifies that page images are being sent to the
configured endpoint.
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

The configured `manual_journal` and the derived bank journals are writable
sections. Configured `historical_journals` are fixed and read-only.
Transactions are revised in the same source journal: ordinary entries stay in
the active manual journal, and statement-derived entries stay in their bank journal.
Entries in other included journals remain read-only.
Existing books are not migrated or changed just by opening this screen.
Deletion and period closing are not implemented. A transaction becomes
reconciled only through the explicit Finish action described above; reopening
removes that checkpoint without reversing journal entries.

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
matches `131.200`, but not a `13` later in a local name); start with other text to
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
description/memo, reference, and source reference. The form renders new entries
as `PAYEE | MEMO`; `wat check` also accepts native descriptions without a memo.
The payee must be declared in the books, either manually in `books/payees.journal`
or automatically from the customer registry in `books/customers.journal`.
Every saved entry also receives a stable random `id` tag. Reference/source tags cannot contain
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

Passing `wat check` establishes syntax, declared accounts/commodities/payees,
posting account depth, balance
assertions, and balanced entries, not correct classification, completeness,
regulatory compliance, or a new bank reconciliation. Existing reconciliation
checkpoint integrity is verified when the books open. There is no implicit
balancing adjustment or year-close operation.
