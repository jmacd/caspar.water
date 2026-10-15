# Water Accounting Terminal

`wat` is a local terminal workbench for ordinary hledger journals. The first
increment supports transaction entry, journal review, strict validation, and
reports. It does not calculate bills, reconcile bank statements, or close years.
There is no database, server, browser, or Watertown dependency.

## Start a workspace

For a persistent test workspace, run:

```sh
./cmd/wat/run.sh
./cmd/wat/run.sh check
./cmd/wat/run.sh report journal
```

The script copies the example into git-ignored `local/wat-test` on first use,
including its test configuration, and creates a source-document directory.
Later runs preserve your test entries and configuration. It works regardless of
the current working directory and forwards arguments to `wat`.

Install `hledger` on PATH. The adapter requires `--no-conf`, `files`, strict
checking, JSON transaction output, and the standard report commands; development
uses hledger 1.52.4.
It ignores hledger configuration files so personal report options cannot change
validation or saved entries.

Copy the example into a private workspace; do not enter real books in the example:

```sh
mkdir -p local
cp -R cmd/wat/example local/water-books
mkdir -p local/water-books/sources
go run ./cmd/wat --config local/water-books/wat.yaml
```

The starter chart is deliberately incomplete. It is not an approved acquisition
entry or a determination of entity structure. Review and extend the account
declarations against CPUC U-39-W and the acquisition evidence before entry.
No opening balances, purchase price, or financial activity are fabricated.

```yaml
version: 1
name: Caspar Water
year: 2021
journal: books/books.journal
write_journal: books/2021.journal
sources: sources
```

Paths are relative to the configuration file. All included journals must be
regular `.journal` or `.ledger` files within that directory, without symlinks.
Use relative includes. `write_journal` must already exist and be included by
`journal`. Unknown configuration keys and additional YAML documents are errors.
`sources` names the supporting-document directory; references are currently
free-form text, not imported or automatically verified document links.

## Use the terminal

Select a menu item with arrows and Enter, or its numbered shortcut. In transaction
entry, Up/Down always move vertically between inputs, staying in the account or
amount column across posting rows, even with suggestions open. Tab/Shift-Tab
step through fields and buttons in order: account, amount, next account, next
amount. Moving away from a field never accepts a suggestion.

Payee and description suggest case-insensitive text matches from existing
transactions across all included journals, including earlier years. Suggestions
refresh when you start a new transaction; no separate history database is kept.
Account suggestions come from the declared chart: start with digits to match
the leading CPUC account-code prefix (`13` matches `131.2`, but not a `13` later
in an account name); start with other text to search anywhere in full account
names, ignoring case. The code is the first number-prefixed account component;
numbers in descendant names do not change its classification.

Use **Ctrl-N/Ctrl-P** to cycle suggestions and **Enter** to accept the highlighted
suggestion. Enter without suggestions advances to the next field. You can also
type complete values directly. New posting accounts start blank rather than
silently selecting the first chart account.

Transactions accept any number of posting rows with account and amount columns,
each with a declared account and a signed USD amount:
positive is a debit, negative is a credit. Use `125.00` or `-125.00`, without a
currency symbol or grouping separators. The signed sum must be zero.
The live balance stays below the posting area and shows the signed amount still
needed: entering `-100` shows `+100.00 USD (debit)` to balance; adding `+40`
changes that to `+60.00`. Blank amounts are ignored while drafting, but invalid
amounts or overflow show an explicit unavailable-balance message rather than an
incorrect total. A balanced amount total alone does not mean the entry is valid.

Dates must be within the configured year. Enter the actual transaction date,
payee, optional description, reference, and source reference. Every saved entry
also receives a stable random `id` tag. Reference/source tags cannot contain
commas; fields cannot contain control characters. Payee/description cannot
contain journal delimiters `;` or `|`.

The action bar stays visible even when posting rows scroll. **Esc** or **Back
(Esc)** returns to the menu after confirming that unsaved work should be
discarded. Choose Keep editing in that confirmation to retain the draft.

**F2** or **Save (F2)** saves directly from the form, without a separate preview
step. Save validates the complete proposed books
with `hledger check --strict`, then appends the entry while preserving existing
bytes. Going back does not change the journals. A repeated Save of the same draft
cannot post it twice.

Press **F4** from the entry form or preview to edit the pending transaction's
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

Keep the generated `id` tag unchanged. Only one transaction, dated in the
configured year, with regular `$` postings is accepted; journal directives are
not allowed. hledger parses the edited text and validates it against the complete
books. Closing the editor returns to an exact-text preview, not an automatic
save. Invalid edits can be reopened without losing their text.

After editing text, use F4 again to revise it or Save to post it. Returning to
the structured form requires explicit confirmation to discard the text edits;
the tool does not round-trip or silently simplify manually edited journal text.
The temporary file is removed after the editor exits.

```sh
go run ./cmd/wat --config local/water-books/wat.yaml check
go run ./cmd/wat --config local/water-books/wat.yaml report journal
go run ./cmd/wat --config local/water-books/wat.yaml report trial-balance
go run ./cmd/wat --config local/water-books/wat.yaml report balance-sheet
go run ./cmd/wat --config local/water-books/wat.yaml report income-statement
```

Journal and income reports show the selected year's activity. Trial balance and
balance sheet include earlier balances through the selected year-end and exclude
later activity. The journals remain usable directly:

```sh
hledger -f local/water-books/books/books.journal
hledger-ui -f local/water-books/books/books.journal
```

Use your editor for arbitrary existing-entry changes, then run `wat check`.
Reports are hledger's presentation, not a second Go accounting interpretation.
No entire journal is round-tripped through a parser or reformatted.

## File safety and recovery

Save validates private temporary copies, retains a synced `.wat-backup-*` recovery
copy beside the writable journal, and atomically replaces the journal. Recovery
copies are not included by the example's explicit includes; retain them according
to your backup policy. Local recovery copies are not off-device backups.

A draft snapshots every included journal. Changes to their contents, permissions,
or include set cause Save to fail: cancel and reopen the transaction. Cooperative
`wat` writers use an exclusive `<write_journal>.wat-lock` file during save.
After a crash, inspect the journal and ensure no other writer is active before
removing a stale lock.

External editors do not honor that lock. Changes are rechecked immediately before
replacement, but there is no cross-program filesystem compare-and-swap: do not
edit the books concurrently with a save.

Passing strict checks establishes syntax, declared accounts/commodities, balance
assertions, and balanced entries, not correct classification, completeness,
regulatory compliance, or bank reconciliation. There is no implicit balancing
adjustment, cleared status, or year-close operation.
