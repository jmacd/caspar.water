# QuickBooks 2021 reconstruction work list

## Purpose

Reconstruct the 2021 books from the available QuickBooks exports and other
records, producing an editable candidate hledger journal that can replace the
old books after manual review.

This is a recovery workflow, not an ordinary import. The final replacement
journal does not need QuickBooks provenance tags. Temporary source identifiers
and reconciliation reports must nevertheless be retained during reconstruction
so that omissions, duplicates, and parser defects can be found before the
candidate journal is accepted.

The raw source files remain read-only and outside Git. Generated manifests,
normalized rows, model responses, and financial reports must also remain in a
private, git-ignored workspace.

## Current baseline

The first complete run read one bank-register export and ten Account QuickReport
exports.

| Observation | Initial result | Meaning |
|---|---:|---|
| Bank-register rows dated 2021 | 5 | Almost certainly incomplete; investigate extraction before trusting any reconstruction |
| Account QuickReport rows dated 2021 | 56 | Includes appearances of the same transaction in different account reports |
| Total extracted source rows dated 2021 | 61 | Not a count of unique accounting transactions |
| Later Wat journal examples | 0 | No later classification evidence reached the model |
| Candidates in one run | 27 | Many source rows were omitted or deduplicated |
| Candidates in an earlier run | 31 | Model interpretation is not reproducible yet |

The initial run proved that the file formats can be opened, but it did not prove
that all spreadsheet rows were extracted or that the generated entries are
complete. Missing configured accounts also caused both omissions and unsafe
account guesses.

## Safety rules

- [ ] Do not run AI reconstruction again until the register-extraction gate is
  complete.
- [ ] Never modify the source exports.
- [ ] Never silently skip a nonempty spreadsheet row.
- [ ] Never map an unknown QuickBooks account to a merely plausible Wat account.
- [ ] Never treat a guessed cash account, owner-equity account, or expense
  account as established evidence.
- [ ] Every normalized source row must receive an explicit disposition:
  transaction evidence, duplicate appearance, structural/report row, unresolved
  row, or rejected row with an error.
- [ ] Keep unresolved transactions visible; do not turn them into prose warnings
  that are easy to overlook.
- [ ] Keep temporary source identifiers in private working artifacts until all
  coverage checks pass, even though they will not appear in the final journal.
- [ ] Do not incorporate a candidate journal into Wat until the final validation
  gate is complete.

## Gate 1: prove the bank-register extraction

The five-row result is the first issue to resolve. Determine whether the source
export is incomplete, Pond selected the wrong worksheet/range, or Wat discarded
otherwise valid rows.

### Inspect the workbook and Pond output

- [ ] Record every worksheet name and its used range without modifying the
  workbook.
- [ ] Identify which worksheet and range Pond currently selects.
- [ ] Record the number of nonempty rows in the selected worksheet before header
  interpretation.
- [ ] Capture Pond's normalized CSV in the private workspace.
- [ ] Count rows at each stage:
  - nonempty workbook rows;
  - rows before the selected header;
  - header rows;
  - rows with a date;
  - rows without a date but with other values;
  - rows parsed into transactions;
  - rows outside 2021;
  - subtotal, balance-forward, and total rows;
  - rejected rows.
- [ ] Check for multiple worksheets, repeated headers, hidden rows, merged cells,
  filtered ranges, blank date cells, and multi-line transaction/split rows.
- [ ] Check the raw cell types and number formats used by every apparent date
  column.
- [ ] Confirm whether blank dates mean continuation/split rows rather than
  nontransaction rows. The current parser skips any blank-date row unless it is
  a one-cell section heading.
- [ ] Confirm whether payments and deposits use separate columns, a signed amount
  column, or both.
- [ ] Check whether the export contains opening and ending balances.

### Add deterministic diagnostics

- [ ] Add an extraction/inspection mode that stops before any model request.
- [ ] Report the workbook, worksheet, selected header, date range, and row counts
  by classification.
- [ ] Report the minimum and maximum parsed dates and counts by calendar year.
- [ ] Report nonempty skipped rows individually with row number and reason.
- [ ] Fail if a nonempty row cannot be classified; do not silently continue.
- [ ] Add a private normalized-row export suitable for manual comparison.
- [ ] Add a regression fixture reproducing the actual register structure without
  including private financial data.

### Reconcile the extracted register

- [ ] Compare the extracted transaction count with the count visible in the
  original QuickBooks export.
- [ ] Compare the first and last transaction dates.
- [ ] Compare total payments and total deposits.
- [ ] Where balances are available, verify the running-balance equation using
  QuickBooks' actual sign convention.
- [ ] Explain every difference between the workbook rows and normalized rows.

**Exit condition:** the normalized register contains every transaction row in
the exported register, with matching dates, amounts, payment/deposit direction,
and balances. If the export itself contains only five 2021 transactions, obtain
a corrected full-year export before continuing.

## Gate 2: inventory every input file

Create a deterministic source manifest before interpreting transactions.

- [ ] For every workbook, record:
  - private file identifier and content hash;
  - file type;
  - worksheet names;
  - selected worksheet and range;
  - report title;
  - QuickBooks account/report subject;
  - stated report date range;
  - detected header row;
  - transaction row count by year;
  - earliest and latest transaction date;
  - counts by QuickBooks transaction type;
  - amount-column totals;
  - nonempty skipped-row count.
- [ ] Identify the QuickBooks account represented by each generically named
  Account QuickReport.
- [ ] Determine whether each zero-row 2021 report genuinely has no 2021 activity
  or failed extraction.
- [ ] Detect identical files by content hash.
- [ ] Detect repeated report coverage and overlapping date ranges.
- [ ] Distinguish duplicated transaction appearances across reports from
  genuinely duplicated QuickBooks transactions.
- [ ] Record the source files expected but not yet present.

**Exit condition:** every supplied file has a known purpose, selected data range,
and explained row count.

## Gate 3: collect the minimum completeness evidence

Request exports from QuickBooks where they are still available. Prefer reports
that expose complete split transactions rather than more presentation-oriented
Account QuickReports.

### Required

- [ ] Complete bank register for `2021-01-01` through `2021-12-31`.
- [ ] General Ledger or Transaction Detail by Account for all accounts over the
  same date range, including split detail.
- [ ] QuickBooks Chart of Accounts with account type and hierarchy.
- [ ] Trial Balance as of `2020-12-31`.
- [ ] Trial Balance as of `2021-12-31`.
- [ ] 2021 Balance Sheet.
- [ ] 2021 Profit and Loss report.

### Required when applicable

- [ ] Determine whether the old books used cash or accrual accounting.
- [ ] If accrual-based, obtain year-end accounts-receivable and
  accounts-payable detail, open invoices, and unpaid bills.
- [ ] Obtain statements or other independent cash records for any part of 2021
  available outside QuickBooks.
- [ ] Obtain support for noncash journal entries, owner contributions/draws,
  reimbursements, and inter-account transfers that cannot be inferred from the
  bank register.

**Exit condition:** opening balances, 2021 activity, and ending balances can each
be checked against an independent QuickBooks report or source record. Any
unavailable evidence is listed explicitly as a known reconstruction limitation.

## Gate 4: normalize rows without accounting guesses

The deterministic parser, not the model, must recover the spreadsheet
structure.

- [ ] Preserve these fields when present:
  - source report account;
  - distribution account;
  - transaction date and type;
  - reference/check number;
  - payee/name;
  - description/memo;
  - amount, debit, credit, payment, deposit, and balance;
  - section and split relationship;
  - temporary source file, worksheet, and row identifier.
- [ ] Classify title, metadata, header, repeated-header, section, transaction,
  split, subtotal, total, and blank rows explicitly.
- [ ] Preserve continuation and split rows that have no date of their own.
- [ ] Normalize dates and monetary values deterministically.
- [ ] Reject malformed dates and amounts loudly.
- [ ] Do not flatten a multi-line journal entry into unrelated single-line
  transactions.
- [ ] Retain QuickBooks transaction IDs if a future export supplies them.
- [ ] Produce a normalized private CSV or JSON artifact before grouping or AI.

**Exit condition:** the same inputs always produce the same normalized rows, and
every nonempty source row has a recorded classification.

## Gate 5: reconstruct transaction groups and duplicates

Account reports can show different sides of the same transaction. Build
transaction groups before asking the model for interpretation.

- [ ] Use the bank register as the primary cash chronology once its completeness
  is proven.
- [ ] Group report appearances using available transaction IDs first.
- [ ] When no ID exists, use a conservative combination of date, amount,
  transaction type, reference number, payee, report account, distribution
  account, and split structure.
- [ ] Never merge records solely because date and amount match.
- [ ] Represent one-to-many split transactions as one candidate group.
- [ ] Record every contributing source row in the private group artifact.
- [ ] Mark ambiguous possible matches unresolved rather than selecting one.
- [ ] Identify noncash journal entries separately from bank transactions.
- [ ] Check that grouped cash transactions account for the register's payment
  and deposit totals.

**Exit condition:** every normalized transaction row belongs to exactly one
transaction group or one explicit unresolved set, and duplicate appearances do
not create duplicate journal entries.

## Gate 6: define the QuickBooks-to-Wat account map

Account translation is an accounting decision and must not vary by model run.

- [ ] List every distinct QuickBooks source and distribution account found in
  2021.
- [ ] Map each QuickBooks account to:
  - one configured Wat account;
  - a documented split rule; or
  - unresolved.
- [ ] Verify every Wat account against the CPUC chart before adding it to
  `wat.yaml`.
- [ ] Distinguish expenses, capital assets, liabilities, owner equity,
  reimbursements, transfers, income, and noncash contributions.
- [ ] Decide the treatment of historical categories that currently have no Wat
  account, including professional services, insurance, maintenance, operations,
  taxes, legal/project costs, and invested labor.
- [ ] Decide whether mappings apply to an entire QuickBooks account or require
  transaction-level review.
- [ ] Store approved mappings in private, reviewable configuration rather than
  only in a prompt.
- [ ] Prohibit fallback mappings such as using a known vendor expense account
  merely because no better configured account exists.
- [ ] Prohibit treating ambiguous transfers or deposits as owner equity without
  independent evidence.

**Exit condition:** all deterministic mappings are approved and every remaining
account or transaction is explicitly unresolved.

## Gate 7: establish later-journal examples

The initial run supplied zero post-2021 examples.

- [ ] Determine whether the active Wat workspace actually contains accepted
  transactions after 2021.
- [ ] If it does, determine why `quickBooksJournalExamples` returned zero.
- [ ] Confirm that `books.books.journal` includes the expected later journals.
- [ ] Report example count, date range, and account coverage before model use.
- [ ] Do not describe later examples as available when the count is zero.
- [ ] Treat later examples as classification hints only; never copy dates,
  amounts, or unreviewed historical mistakes.

**Exit condition:** later examples are either correctly loaded and summarized or
explicitly declared unavailable.

## Gate 8: narrow and validate the AI role

The model should interpret ambiguous descriptions, not repair parser gaps or
invent the chart of accounts.

- [ ] Supply complete global context: input-file inventory, whether a register
  exists, whether that register is complete, and whether the current month has a
  register row.
- [ ] Supply pre-grouped transactions instead of unrelated raw rows.
- [ ] Supply only approved account mappings and configured accounts.
- [ ] Require one structured disposition for every transaction group:
  candidate, unresolved, or excluded with an allowed deterministic reason.
- [ ] Require candidates to cite temporary group identifiers in the private
  response.
- [ ] Validate that every candidate amount and posting structure is supported by
  its cited group, not merely that the same amount appears somewhere in the
  month.
- [ ] Keep unresolved items in a structured artifact with reason and required
  decision.
- [ ] Decide an acceptance policy for confidence. Recommended: low-confidence
  items remain unresolved rather than entering the candidate journal.
- [ ] Prevent the model from choosing a cash account where no source evidence or
  approved deterministic rule identifies one.
- [ ] Cache the exact structured response by normalized-input hash so a reviewed
  run can be reproduced.
- [ ] Use deterministic model settings where the provider supports them.
- [ ] Compare repeated uncached runs. Accounting content and dispositions must
  not change, ignoring newly generated journal IDs.

**Exit condition:** changing model wording cannot change deterministic
extraction, grouping, or account mappings, and the validator can prove complete
disposition coverage.

## Gate 9: produce review artifacts

One stdout journal and free-form stderr warnings are insufficient.

- [ ] Produce a candidate journal containing only accepted balanced candidates.
- [ ] Produce a structured unresolved-transactions report.
- [ ] Produce a source coverage report showing:
  - workbook rows;
  - normalized transaction rows;
  - structural rows;
  - grouped transaction count;
  - duplicate appearances;
  - candidate count;
  - unresolved count;
  - rejected count.
- [ ] Produce an account-mapping report listing used and unresolved mappings.
- [ ] Produce bank totals by month and for the year.
- [ ] Produce noncash journal totals separately.
- [ ] Make the counts add up exactly; warnings are not a substitute for coverage.
- [ ] Keep financial artifacts private and git-ignored.
- [ ] Strip temporary source identifiers only from the final hand-edited journal,
  after validation is complete.

**Exit condition:** a reviewer can account for every input row and transaction
group without reading model prose.

## Gate 10: accounting and completeness validation

- [ ] Run hledger strict validation on the candidate journal.
- [ ] Verify that every entry balances and uses an approved configured account.
- [ ] Reconcile reconstructed bank activity to the complete register by month.
- [ ] Reconcile opening and ending bank balances.
- [ ] Reconcile total 2021 activity by QuickBooks account before translation.
- [ ] Reconcile the reconstructed opening balances to the 2020 year-end Trial
  Balance.
- [ ] Reconcile the reconstructed ending balances to the 2021 year-end Trial
  Balance.
- [ ] Compare reconstructed Balance Sheet and Profit and Loss reports with the
  QuickBooks reports.
- [ ] Review every unresolved item manually and either resolve it or document the
  accepted limitation.
- [ ] Review owner-equity, liability, transfer, capital-asset, and noncash entries
  individually.
- [ ] Run the complete reconstruction twice and compare canonical accounting
  content, ignoring generated entry IDs.
- [ ] Confirm that no source row disappeared between the manifest, normalized
  data, grouping, and final disposition reports.

**Exit condition:** the journal is balanced, coverage is complete, register and
control-report differences are zero or explicitly explained, and every
low-confidence accounting decision has been reviewed.

## Order of work

1. Prove the bank-register extraction and add parser diagnostics.
2. Inventory all supplied reports and explain every zero-row result.
3. Obtain the complete register and accounting control reports.
4. Correct deterministic normalization, including blank-date split rows.
5. Group duplicate appearances and split transactions.
6. Build and approve the QuickBooks-to-Wat account map.
7. Fix or explicitly waive the missing later-journal examples.
8. Replace free-form omission warnings with complete structured dispositions.
9. Generate the private review artifacts and candidate journal.
10. Reconcile the result to the bank register and QuickBooks control reports.

Do not skip directly to another AI run. The next executable task is Gate 1:
explain why the current bank-register export produces only five 2021
transactions.
