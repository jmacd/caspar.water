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
| SBMC bank-register rows dated 2021 | 5 | Confirmed complete for that account; earlier company costs were paid through a personal bank account |
| Personal-account transaction history | Unavailable by design | Do not request or ingest it; preserve cash-side and owner-treatment questions explicitly |
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
- [ ] Never request, ingest, or reconstruct the operator's personal bank-account
  history. Use only the available business records.
- [x] Every normalized dated transaction row must receive exactly one
  model-response disposition: balanced candidate or unresolved skeleton.
- [x] Classify structural/report rows deterministically before the model as
  section, total, export footer, blank, or rejected row with an error.
- [x] Keep unresolved transactions visible; do not turn them into prose warnings
  that are easy to overlook.
- [ ] Keep temporary source identifiers in private working artifacts until all
  coverage checks pass, even though they will not appear in the final journal.
- [ ] Do not incorporate a candidate journal into Wat until the final validation
  gate is complete.

## Gate 1: prove the SBMC bank-register extraction

The five-row result initially needed to be separated into a source-data question
and a parser question.

### Current evidence

The model-free `quickbooks --inspect` path reports 202 normalized data rows in
the bank-register export. All 202 rows have valid dates; none are blank,
undated, or silently skipped after header detection. The date span is
`2021-11-10` through `2026-09-30`, and exactly five rows are dated 2021.

This rules out Wat's year filter and blank-date handling as the cause of the
five-row result. The operator confirmed that five is the correct count for the
SBMC account: the other side of the earlier 2021 company transactions is in a
personal bank account.

The SBMC count question is therefore resolved. Amount and balance checks remain
part of final accounting validation, but the reconstruction must not expect this
register to provide the cash side of pre-SBMC transactions.

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
- [x] Confirm whether blank dates mean continuation/split rows rather than
  nontransaction rows. The current parser skips any blank-date row unless it is
  a one-cell section heading. This register's normalized data contains no
  blank-date rows.
- [ ] Confirm whether payments and deposits use separate columns, a signed amount
  column, or both.
- [ ] Check whether the export contains opening and ending balances.

### Add deterministic diagnostics

- [x] Add an extraction/inspection mode that stops before any model request.
- [ ] Report the workbook, worksheet, selected header, date range, and row counts
  by classification. Header, date range, and normalized row counts are
  implemented; worksheet metadata remains.
- [x] Report the minimum and maximum parsed dates and counts by calendar year.
- [x] Report nonempty undated rows individually with row number and populated
  fields.
- [x] Fail on malformed dates or nonempty values beyond the detected header
  columns; do not silently continue.
- [ ] Add a private normalized-row export suitable for manual comparison.
- [ ] Add a regression fixture reproducing the actual register structure without
  including private financial data.

### Reconcile the extracted register

- [x] Compare the extracted transaction count with the count expected in the
  original QuickBooks export.
- [ ] Compare the first and last transaction dates.
- [ ] Compare total payments and total deposits.
- [ ] Where balances are available, verify the running-balance equation using
  QuickBooks' actual sign convention.
- [ ] Explain every difference between the workbook rows and normalized rows.

**Exit condition:** the normalized register contains every transaction row in
the exported register, with matching dates, amounts, payment/deposit direction,
and balances. The five-row 2021 count is confirmed; the remaining amount and
balance checks are retained for final reconciliation.

## Gate 2: inventory every input file

Create a deterministic source manifest before interpreting transactions.

### Current inventory

The private model-free inventory records a content hash, file type, normalized
header, row classes, date coverage, annual counts, report sections, totals, and
selected-year transaction types for every supplied file. All eleven content
hashes are distinct. Every normalized row is classified as a dated transaction,
section, total, export footer, or blank row; there are no unexplained undated
rows.

| File key | QuickBooks report subject | 2021 rows | Date coverage or explanation |
|---|---|---:|---|
| SBMC register | SBMC bank account | 5 | Starts `2021-11-10`; count confirmed |
| unnumbered | Income / company labor invested | 2 | `2021-05-09` through `2021-07-30` |
| `(1)` | Landlord expenses / professional services | 6 | Starts `2021-01-10` |
| `(2)` | Minna Costs | 13 | Starts `2021-02-16` |
| `(3)` | Needs Research | 0 | Only one dated row, in 2024 |
| `(4)` | Property tax, South Pasture | 1 | Starts `2021-11-19` |
| `(5)` | Sales of Product Income | 0 | Earliest dated row is in 2022 |
| `(6)` | Solar Lease Revenue | 0 | Earliest dated row is in 2024 |
| `(7)` | Water Company Expenses: maintenance, operations, and utilities | 33 | Starts `2021-01-15` |
| `(8)` | Water Company Non-Service / professional services | 0 | Earliest dated row is in 2022 |
| `(9)` | Water Company Yearly: insurance and Upper Well property tax | 1 | Starts `2021-02-19` |

- [ ] For every workbook, record:
  - [x] private file identifier and content hash;
  - [x] file type;
  - worksheet names;
  - selected worksheet and range;
  - [x] report title/section labels;
  - [x] QuickBooks account/report subject;
  - stated report date range;
  - [x] detected header row;
  - [x] transaction row count by year;
  - [x] earliest and latest transaction date;
  - [x] counts by QuickBooks transaction type for the selected year;
  - amount-column totals;
  - [x] nonempty skipped-row count and classification.
- [x] Identify the QuickBooks account represented by each generically named
  Account QuickReport.
- [x] Determine whether each zero-row 2021 report genuinely has no 2021 activity
  or failed extraction.
- [x] Detect identical files by content hash.
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

- [x] Complete SBMC register for its 2021 active period. The five-row count is
  confirmed; amounts and balances still require final reconciliation.
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

### Known unavailable evidence

- [x] The personal bank-account transaction history will not be shared with or
  ingested by this workflow.
- [ ] Treat a QuickBooks business transaction described as personally paid as
  evidence only for the business-side posting. Do not attempt to reconstruct
  the personal account or infer its other transactions.
- [ ] When the available business records do not establish whether a payment was
  reimbursable, contributed capital, or subsequently reimbursed, emit one
  structured open question rather than a balanced candidate.
- [ ] Allow manual answers to those questions to supply the missing
  due-to-owner, owner-contribution, reimbursement, or other approved posting.

**Exit condition:** opening balances, 2021 activity, and ending balances can each
be checked against an independent QuickBooks report or source record. Any
unavailable evidence is listed explicitly as a known reconstruction limitation,
and every transaction affected by it remains visible as an open question.

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

- [ ] Use the SBMC register as the primary cash chronology only for transactions
  actually present in that account.
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
- [ ] Decide whether each company cost paid from the personal account created an
  amount due to the owner, was an owner contribution, or was later reimbursed;
  otherwise retain that decision as an open question. Do not assume one
  treatment for every personal-account payment.
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
- [x] Require one structured disposition for every supplied transaction row:
  candidate, unresolved, or excluded with an allowed deterministic reason.
- [x] Require candidates and unresolved items to cite temporary source
  identifiers in the private response. Reject missing, unknown, and reused
  identifiers.
- [ ] Validate that every candidate posting structure is supported by its cited
  transaction group. Amounts are now checked against cited rows rather than any
  same-valued row in the month; deterministic grouping remains.
- [x] Keep unresolved items in a structured artifact with reason and required
  decision.
- [x] For missing personal cash-side evidence, require the unresolved item to
  state the supported business-side posting, missing counterposting fact,
  allowed accounting decisions, and the question a reviewer must answer.
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

- [x] Produce a candidate draft containing accepted balanced candidates while
  leaving unresolved skeletons fully commented and therefore ignored by
  hledger.
- [x] Produce structured unresolved transactions in the model response.
- [x] Produce open questions that can be answered incrementally
  without exposing personal-account records.
- [x] Render unresolved transactions as editable, fully commented half-written
  entry skeletons in the generated draft. Include the supported date, payee,
  description, amount, and business-side posting, then mark the missing
  counterposting and question explicitly. Because they remain comments, they
  cannot be imported accidentally as postings.
- [x] Expect manual work to complete those skeletons. An incomplete entry is not
  a reconstruction failure, but it must never appear balanced or resolved until
  a reviewer supplies the missing fact and account.
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
2. Inventory all supplied business reports and explain every zero-row result.
3. Obtain the available business accounting control reports.
4. Correct deterministic normalization, including blank-date split rows.
5. Group duplicate appearances and split transactions.
6. Build and approve the QuickBooks-to-Wat account map.
7. Fix or explicitly waive the missing later-journal examples.
8. Replace free-form omission warnings with complete structured dispositions and
   open questions.
9. Generate the private review artifacts and candidate journal.
10. Reconcile the result to SBMC and the available QuickBooks control reports.

Do not skip directly to another AI run. The next task is to inventory the
available business reports and design the structured open-question output for
facts the intentionally unavailable personal-account history cannot establish.
