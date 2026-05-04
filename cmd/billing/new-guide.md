# Caspar Water Books — Operator's Guide

> A reference manual for the day-to-day operator. The system is already
> set up for you. This guide covers the routine tasks: cycling tenants,
> recording expenses and payments, running a billing cycle, and asking
> the books "how are we doing?"

---

## Table of contents

1. [About this guide](#1-about-this-guide)
2. [Three things to keep straight](#2-three-things-to-keep-straight)
3. [How money moves through the books](#3-how-money-moves-through-the-books)
4. [Routine tasks](#4-routine-tasks)
5. [Run a billing cycle](#5-run-a-billing-cycle)
6. [Reports](#6-reports)
7. [Fixing mistakes](#7-fixing-mistakes)
8. [Backups](#8-backups)
9. [Quick command cheat sheet](#9-quick-command-cheat-sheet)
10. [Troubleshooting](#10-troubleshooting)
11. [Glossary](#11-glossary)

---

## 1. About this guide

You operate the books for Caspar Water. The plumbing — the pond storage,
the chart of accounts, the billing policy, the dynamic-factory views —
is already wired up by your tech contact. From your seat, every command
looks like:

```bash
pond run /accounts <subcommand> [options]
```

The handful of subcommands in this guide is everything you need for
ordinary day-to-day work.

If a command refuses to run, or a number doesn't look right, jump to
[§7 Fixing mistakes](#7-fixing-mistakes) or [§10 Troubleshooting](#10-troubleshooting).

---

## 2. Three things to keep straight

The system tracks three different kinds of records that often get
confused. They are kept separate on purpose.

### Connections — the houses

A **connection** is a physical water hookup at a fixed address (the
community center, "100 Example St", etc.). Connections rarely change —
only when a new house joins the system or one is permanently removed.
They have:

- A short **numeric id** (e.g. `5`) assigned by the system.
- A short **name** (e.g. "Community Center", "100 Example St").
- The **service address**.
- A **commercial** flag (the community center is the classic example).

### Customers — the paying parties

A **customer** is a person, family, or organization that pays bills.
Customers come and go as people move in and out. They have:

- A short **numeric id** (e.g. `12`).
- The **full name** as it appears on the bill.
- A **billing address** (where to mail the statement).
- **Contact info** (phone / email).

A customer can be responsible for several connections at once. A
landlord who owns four houses is **one** customer with four bills.
When someone moves out and someone else moves in, the **connection
stays the same** but the **customer changes**.

### Tenancies — who is responsible right now

A **tenancy** says "this customer was responsible for that connection
between these two dates." Every active connection has at most one
*current* tenancy. Tenancies are how the system knows who to send each
cycle's bill to.

Three pictures to lock this in:

```
CONNECTION (the house)        connection #5  --- one row, basically forever
CUSTOMER   (the person)       customer #12 (Alex Carver) --- one row, basically forever
TENANCY    (the relationship) (connection 5, customer 12, 2021-10-01, NULL)
                                                                       ^
                                                                  NULL = still here
```

When Alex moves out and Jordan moves in:

```
TENANCY (connection 5, customer 12 (Alex),   2021-10-01, 2026-06-30)   <- closed
TENANCY (connection 5, customer 23 (Jordan), 2026-07-01, NULL)         <- new, open
```

The connection row is unchanged. Alex's customer row is unchanged
(and **if Alex still owes money, that balance stays with Alex**, not
with the house).

### How to look things up

Every connection, customer, tenancy, payment, expense, and journal
entry has an **integer id** (1, 2, 3, …). You can pass either the id
or the full name to most commands:

```bash
pond run /accounts payment add --customer=12 ...
pond run /accounts payment add --customer='Alex Carver' ...
```

If a name is ambiguous, the command refuses and lists candidates.

To find an id you've forgotten:

```bash
pond run /accounts customer list                    # everyone, alphabetical
pond run /accounts customer find henderson          # fuzzy by partial name
pond run /accounts customer show 4                  # full picture for one customer
pond run /accounts connection list                  # all houses
pond run /accounts connection show 5                # one house: status, history, current customer
```

---

## 3. How money moves through the books

You don't need accounting training. Two rules cover it:

1. **Every transaction has two sides that have to match.** Money
   doesn't appear or disappear; it moves *from* one bucket *to* another.
   When a customer pays $300:
   - Your **bank account** goes up by $300, and
   - That customer's **balance owing** goes down by $300.

2. **Buckets are named with numbers.** The codes you'll actually type
   are all expense categories:

   | Code | What it's for |
   |---|---|
   | 5100 | Operations (chemicals, lab tests, water treatment) |
   | 5200 | Utilities (electricity, etc.) |
   | 5300 | Insurance (liability premiums) |
   | 5400 | Taxes (property tax, business licensing, certifications) |
   | 5900 | Other expenses (anything that doesn't fit above) |

   The other codes (cash, accounts receivable, service revenue) are
   handled automatically — you'll see them in journal listings but you
   never have to type them.

When you record a payment or an expense, the system writes both sides
for you. You only see one amount and one category.

---

## 4. Routine tasks

The work that comes up week-to-week.

### 4.1 Record an incoming payment

A check arrived from Robin Doyle (customer 7) for $371.87:

```bash
pond run /accounts payment add \
  --date=2026-05-15 \
  --customer=7 \
  --amount='$371.87' \
  --method=check \
  --memo='check #1234'
```

- `--date=` is the day you received the payment.
- `--customer=` is the customer id (or full name).
- `--amount=` accepts `$371.87`, `371.87`, or `$1,371.87`.
- `--method=` is optional (`check`, `ach`, `cash`, `card`).
- `--memo=` is optional but useful for notes.

The system prints back the new payment id and the customer's new balance.

### 4.2 Record an outgoing expense

You paid Acme Labs $130 for water testing:

```bash
pond run /accounts expense add \
  --date=2026-05-02 \
  --account=5100 \
  --vendor='Acme Labs' \
  --amount='$130.00' \
  --memo='lab analysis Apr 2026'
```

- `--account=` uses the codes from §3 (operations is `5100`, utilities
  `5200`, insurance `5300`, taxes `5400`, miscellaneous `5900`).
- The expense is automatically attached to the cycle that contains
  `--date=`. Override with `--cycle=2026H1` if needed.

#### Bulk import from a spreadsheet

If you keep your line-item expenses in a spreadsheet, export to CSV
and import in one go:

```bash
pond run /accounts expense import host:///tmp/expenses-2026H1.csv
```

The CSV needs columns named `date`, `account`, `vendor`, `amount`, and
optionally `memo` and `cycle`. Re-importing is safe — duplicates are
detected by date + amount + vendor.

### 4.3 A tenant moves out

End the tenancy on the day they vacated. The connection itself stays
in service (waiting for the next tenant); the customer record stays
too (carrying any balance they still owe).

```bash
pond run /accounts tenancy end \
  --connection=5 \
  --end=2026-06-30
```

If the customer truly owes nothing more and you don't expect to hear
from them again, that's it. If they owe a balance, see [§6.2](#62-one-customers-complete-picture)
for how to keep an eye on it.

### 4.4 A new tenant moves in

Two steps: create the customer (if they're new to the system) and
start the tenancy.

```bash
# Step 1 - create the customer (skip if they already exist)
pond run /accounts customer add \
  --name='Jordan Hayes' \
  --billing='100 Example Street; Anytown, CA 99999' \
  --contact='jordan@example.com'
# -> Created customer 23: Jordan Hayes

# Step 2 - start the tenancy
pond run /accounts tenancy start \
  --connection=5 \
  --customer=23 \
  --start=2026-07-01
```

`tenancy start` automatically closes any prior open tenancy on that
connection at the same date — you don't have to remember to call
`tenancy end` first.

If an existing customer is taking over a different connection (e.g. a
landlord adds another house to their portfolio), skip Step 1 and go
straight to Step 2 with their existing customer id.

#### What if the move falls in the middle of a billing cycle?

The customer holding the connection on the cycle's bill date pays the
**whole** share for that cycle. The system does not pro-rate. If you
want to split the bill between the outgoing and incoming tenant,
either arrange a side payment between them, or [call your tech
contact](#74-when-to-call-the-tech-contact) for a manual adjustment.

### 4.5 Update a customer's billing address or contact info

These are reference data — edit in place:

```bash
pond run /accounts customer edit 23 \
  --billing='New mailing address; Anytown, CA 99999' \
  --contact='jordan-new@example.com'
```

Use `customer edit ID --name=...` to fix a typo in the name, or
`--active=false` to mark a customer inactive (their balance is
preserved; they just stop appearing in default lists).

Same idea for connections (rare):

```bash
pond run /accounts connection edit 5 \
  --notes='Now operating as a small B&B'
```

### 4.6 A new connection joins the system (rare)

This happens once in a blue moon. You'll need the new connection's
service address and its first-active date. If it's a commercial
location (uses more water, counts as 2× in the share split), add
`--commercial`.

```bash
pond run /accounts connection add \
  --name='200 Example Drive' \
  --service='200 Example Drive; Anytown, CA 99999' \
  --first-active=2027-04-01
# -> Created connection 17: 200 Example Drive
```

Then add the first customer and tenancy as in §4.4.

> ⚠️ Adding a new connection may affect how the share split is
> computed for upcoming cycles. Talk to your tech contact before doing
> this around a billing date — they'll confirm whether the policy
> needs an update.

---

## 5. Run a billing cycle

This is the big workflow. Cycles run **April 1 – September 30** (call
that `H1` for "first half") and **October 1 – March 31** (`H2`). Six
steps, six months apart.

### Step 1: Make sure all expenses for the cycle are entered

Pull the spreadsheet from your outside accounting program. Either type
each expense in with `expense add` (§4.2) or `expense import` from a
CSV.

Sanity-check that the totals match what you expect:

```bash
pond run /accounts cycle totals --cycle=2026H1
```

You should see four numbers (operations / utilities / insurance /
taxes) that match the spreadsheet.

### Step 2: Make sure all payments received during the cycle are entered

```bash
pond run /accounts payment list --cycle=2026H1
```

Cross-check against your bank statements.

### Step 3: Open the cycle

If this cycle isn't already in the system, add it:

```bash
pond run /accounts cycle add \
  --name=2026H1 \
  --start=2026-04-01 \
  --bill-date=2026-11-01 \
  --policy=share-by-weight-2024 \
  --inactive='4,12,15' \
  --notes='Forgot Acme Labs pre-bill, see expenses 2026-05-02'
```

What these mean:
- `--name=` operator-friendly label (`2026H1` = Apr–Sep 2026,
  `2026H2` = Oct 2026–Mar 2027).
- `--start=` first day of the cycle (April 1 or October 1).
- `--bill-date=` the day you'll mail the statements.
- `--policy=` the billing policy in effect for this cycle. This is
  almost always the most recently registered policy — run
  `pond run /accounts policy list` to see what's available. If you're
  unsure, ask your tech contact; **never invent a policy name**.
- `--inactive=` comma-separated connection ids that are temporarily
  not paying this cycle (vacant, broken meter, etc.).
- `--notes=` free text reminders.

### Step 4: Preview the bills

```bash
pond run /accounts bills preview --cycle=2026H1
```

This prints, for every connection, what the bill *would* be. **It does
not change anything.** Look for:

- The total at the bottom matches what you expected.
- Every connection has a customer named (no blanks).
- The "estimated" column says `false` (or `true` if you're previewing
  before the cycle has actually closed — that's an early estimate).

If a connection shows up with no responsible customer, fix the
tenancy ([§4.4](#44-a-new-tenant-moves-in)) before going to the next
step.

### Step 5: Issue the bills

When you're happy, commit them:

```bash
pond run /accounts bills issue --cycle=2026H1
```

This is the one-way step. The system records every customer's bill in
the books. After this, customer balances reflect the new charges, and
the cycle is marked "issued."

If you spot a problem after this point, see
[§7.3 Reverse an issued cycle](#73-reverse-an-issued-cycle).

### Step 6: Pull each statement

For each connection, pull the data the PDF generator consumes:

```bash
pond run /accounts statement --connection=1 --cycle=2026H1
```

Repeat for each active connection (or hand off the cycle id and let
the PDF tool iterate). Then mail the bills and wait for payments to
arrive (record each one with §4.1).

---

## 6. Reports

### 6.1 Who owes me money?

The single most useful question:

```bash
pond run /accounts who-owes
```

Prints every customer with a positive balance, sorted by amount, with
contact info and the date of their oldest unpaid bill. Filter:

```bash
pond run /accounts who-owes --min='$500'           # only big debtors
pond run /accounts who-owes --as-of=2025-12-31     # what was owed at year-end
```

For one specific customer:

```bash
pond run /accounts balance --customer=7
```

For the full balance roster (including zeros):

```bash
pond run /accounts balance
```

### 6.2 One customer's complete picture

```bash
pond run /accounts customer show 7
```

You'll get:
- Their contact info.
- Every connection they've held (past and present).
- Every bill they've received.
- Every payment they've made.
- Their current balance.

Useful before reaching out to a customer with a question or reminder.

### 6.3 Aging of unpaid balances

How old is each unpaid debt? Bucketed 0–30 / 31–60 / 61–90 / 90+ days
from bill date, sorted with the biggest debts first:

```bash
pond run /accounts aging
```

For one specific customer:

```bash
pond run /accounts aging --customer=7
```

As of an earlier date (e.g. for a year-end report):

```bash
pond run /accounts aging --as-of=2025-12-31
```

### 6.4 Profit and loss for a cycle or period

```bash
pond run /accounts pnl --cycle=2026H1
pond run /accounts pnl --from=2026-01-01 --to=2026-12-31
```

Shows revenue (what you billed), expenses (what you spent), and the
margin contribution to reserves.

### 6.5 Trial balance

Every account with its current total:

```bash
pond run /accounts trial-balance
pond run /accounts trial-balance --as-of=2025-12-31
```

Useful when sharing books with an accountant. The total at the bottom
should always be zero (debits equal credits); if it isn't, run
[`verify`](#10-troubleshooting).

### 6.6 Recent activity

What changed in the journal recently?

```bash
pond run /accounts journal show --limit=20
pond run /accounts journal show --customer=7 --limit=20
pond run /accounts journal show --cycle=2026H1
pond run /accounts journal show --from=2026-05-01
```

---

## 7. Fixing mistakes

The golden rule: **the books never lie about the past.** If you
recorded something wrong, you don't erase it — you record a
correction. This keeps the audit trail clean.

### 7.1 Void a payment

To find the payment id, list first:

```bash
pond run /accounts payment list --customer=7
```

Then void:

```bash
pond run /accounts payment void --id=137 --memo='duplicate of payment 132'
```

Voiding writes a *reversing* entry that cancels out the original. Both
the original and the reversal stay in the books. After voiding, you
can re-add the corrected version (right amount, right date) with
`payment add`.

### 7.2 Void an expense

Same pattern:

```bash
pond run /accounts expense list --cycle=2026H1
pond run /accounts expense void --id=84 --memo='wrong category, should be 5200'
```

Then re-add with the right `--account=`.

### 7.3 Reverse an issued cycle

If you discover an error after `bills issue`:

```bash
pond run /accounts bills reverse --cycle=2026H1 --reason='wrong inactive list'
```

This reverses every bill in that cycle. The cycle is marked "not
issued" again. You can then `cycle edit` to fix the cycle metadata
(bill date, inactive list, notes) and `bills issue` again.

> The cycle's `--policy=` cannot be changed by editing. If you used the
> wrong policy, [call your tech contact](#74-when-to-call-the-tech-contact).

### 7.4 When to call the tech contact

Don't try these alone the first time:

- **A manual adjustment that moves money between accounts** —
  e.g. writing off an uncollectible balance, refunding an overpayment,
  or splitting a mid-cycle bill between two tenants. These need a
  hand-built journal entry.
- **A new billing policy** — when the company decides to change the
  margin, the denominator, or the share weights for upcoming cycles.
- **A new policy *kind*** — e.g. switching to usage-based metered
  billing once meters are read regularly. That requires code from
  the tech contact before you can register it.
- **`verify` reports a problem you don't immediately understand.**
- **Anything involving the chart of accounts** (codes 1000, 1100,
  3000, 4000) directly.

Save the exact command you ran and the message the screen showed; that
plus a "what I was trying to do" sentence is everything they need.

---

## 8. Backups

The books live in a folder pointed to by `$BOOKS`. Backing it up is
something your tech contact set up — either an automatic sync to S3
(`pond sync`) or a periodic copy to an external drive. You shouldn't
have to think about it day-to-day.

If you need a snapshot of the books *outside* the system to share
with an accountant, the same report commands accept `--format=csv`:

```bash
pond run /accounts trial-balance --format=csv > trial-balance.csv
pond run /accounts who-owes      --format=csv > who-owes.csv
pond run /accounts cycle totals  --format=csv > cycle-totals.csv
pond run /accounts aging         --format=csv > ar-aging.csv
pond run /accounts pnl --cycle=2026H1 --format=csv > pnl-2026H1.csv
```

Read-only commands (anything not in the
[mutating list](#cant-mess-it-up-by-reading)) are always safe to run.

#### Can't mess it up by reading

The commands that *change* the books are: `add`, `edit`, `void`,
`issue`, `reverse`, `import`, and `tenancy start` / `tenancy end`.
Everything else (`list`, `show`, `find`, `balance`, `who-owes`,
`aging`, `cycle totals`, `pnl`, `trial-balance`, `statement`,
`journal show`) just looks. You can run them all day with no risk.

---

## 9. Quick command cheat sheet

### Recording transactions

| Task | Command |
|---|---|
| Record an incoming payment | `pond run /accounts payment add --date=… --customer=ID --amount=$…` |
| Record an outgoing expense | `pond run /accounts expense add --date=… --account=5100 --amount=$…` |
| Bulk-import expenses from CSV | `pond run /accounts expense import host:///path/file.csv` |
| Void a payment | `pond run /accounts payment void --id=N --memo=…` |
| Void an expense | `pond run /accounts expense void --id=N --memo=…` |

### People and houses

| Task | Command |
|---|---|
| Add a new customer | `pond run /accounts customer add --name=… --billing=…` |
| Edit a customer | `pond run /accounts customer edit ID --billing=…` |
| Find a customer by name | `pond run /accounts customer find QUERY` |
| Tenant moves in (start tenancy) | `pond run /accounts tenancy start --connection=ID --customer=ID --start=…` |
| Tenant moves out (end tenancy) | `pond run /accounts tenancy end --connection=ID --end=…` |
| List current occupants | `pond run /accounts tenancy list` |
| Add a new connection (rare) | `pond run /accounts connection add --name=… --service=… --first-active=…` |

### Billing cycle

| Task | Command |
|---|---|
| List policies in effect | `pond run /accounts policy list` |
| Add a new cycle | `pond run /accounts cycle add --name=2026H1 --start=… --bill-date=… --policy=NAME` |
| Preview bills (no changes) | `pond run /accounts bills preview --cycle=2026H1` |
| Issue bills (write to books) | `pond run /accounts bills issue --cycle=2026H1` |
| Reverse a bad cycle | `pond run /accounts bills reverse --cycle=2026H1 --reason=…` |
| Statement for one connection | `pond run /accounts statement --connection=ID --cycle=2026H1` |

### Reports

| Task | Command |
|---|---|
| Who owes me money? | `pond run /accounts who-owes` |
| One customer's balance | `pond run /accounts balance --customer=ID` |
| One customer's full history | `pond run /accounts customer show ID` |
| Profit / loss for a cycle | `pond run /accounts pnl --cycle=2026H1` |
| Trial balance | `pond run /accounts trial-balance` |
| Aging of unpaid balances | `pond run /accounts aging` |
| Cycle expense totals | `pond run /accounts cycle totals --cycle=2026H1` |
| Recent journal activity | `pond run /accounts journal show --limit=20` |

### Health check

| Task | Command |
|---|---|
| Sanity-check the books | `pond run /accounts verify` |

---

## 10. Troubleshooting

### "command not found: pond"
Your terminal doesn't know where the `pond` program lives. Open a
fresh terminal window, or ask your tech contact to fix the path.

### "POND environment variable not set"
You forgot to set `$POND` (and/or `$BOOKS`) for this terminal session.
Run:

```bash
export BOOKS=$HOME/caspar-books-pond
export POND=$BOOKS
```

and try again. (Adding those two lines to your shell startup file
means you don't have to retype them.)

### `bills issue` refused to run
The most common reasons:

1. **A connection has no covering tenancy.** The system doesn't know
   who to bill. Run `pond run /accounts connection show ID` for the
   connection in question and add a tenancy ([§4.4](#44-a-new-tenant-moves-in)).
2. **The cycle is already issued.** Use `bills reverse` first
   ([§7.3](#73-reverse-an-issued-cycle)) if you need to redo it.
3. **A connection is active but listed as inactive in the cycle, or
   vice versa.** Check with `cycle show ID|NAME`.
4. **The cycle's billing policy is missing or out of effect.** Confirm
   with `cycle show ID|NAME`. Call your tech contact; this is a setup
   issue, not a data-entry one.

### Numbers don't match what I expect
Run:

```bash
pond run /accounts verify
```

It checks: every transaction balances; AR per customer matches the
sub-ledger; expense rows sum to the cycle totals; no tenancy gaps or
overlaps for issued cycles; every issued cycle's policy is still
valid.

It will list specific problems. Common fixes are a missing
`tenancy start`, a void you forgot to apply, or a typo in a recent
`expense add`. If the message doesn't make sense, [call your tech
contact](#74-when-to-call-the-tech-contact).

### I voided the wrong thing
Voids can themselves be voided. Run
`pond run /accounts payment void --id=THE_VOID_ID` to reverse the
reversal. (Yes, the books grow over time. That's fine — the size is
tiny and the audit trail is the point.)

### I deactivated a customer who still owed money
You didn't delete them — they're still in the books with their balance
intact. Find them:

```bash
pond run /accounts customer list --active=false
```

then bring them back to the active list:

```bash
pond run /accounts customer edit ID --active=true
```

### I'm not sure what changed today
```bash
pond run /accounts journal show --from=$(date +%Y-%m-%d) --limit=50
```

---

## 11. Glossary

**Accounts Receivable (AR)** — money owed *to you* by customers. Code
`1100`. Each customer has their own sub-ledger.

**Bill** — the amount one connection owes for one cycle.

**Bill date** — the day you mail the statements. Used to decide which
tenancy is responsible if there's been a turnover mid-cycle.

**Billing cycle** — a six-month period (April–September or
October–March) over which expenses are tallied and shares calculated.
Identified like `2026H1` (April–September 2026) or `2026H2` (October
2026–March 2027).

**Billing policy** — a named record of how a cycle's total cost is
divided among connections (margin, weights, denominator). Each cycle
pins one policy. You don't create or edit policies — your tech
contact does. You just use `--policy=NAME` when adding a cycle.

**Books / pond** — the folder on disk that holds all the data,
pointed to by `$BOOKS`.

**Commercial (connection)** — a real-world property of the meter:
this connection is a commercial water user. Today's policy treats
commercial connections as 2 votes in the share split. Stays the same
regardless of who is currently the customer.

**Connection** — a physical water hookup at a fixed address. Stable;
changes only when service is permanently retired.

**Customer** — a person or organization that pays bills. Stable as a
person; can move between connections over time. Carries balances even
after moving out.

**Issued (cycle)** — the cycle's bills have been written into the
books. Cycles start out un-issued; once issued, fixes go through
`bills reverse` rather than `cycle edit`.

**Journal** — the immutable list of every financial transaction. Every
two-sided entry lives here forever.

**Reversal / reversing entry** — a new transaction that cancels out a
previous one by swapping debit and credit. The way the books "edit"
something without losing history.

**Statement** — the per-connection bill document showing prior balance,
new charges, payments received, and total due. Generated with
`statement --connection=ID --cycle=…`; the PDF tool turns it into a
printable bill.

**Tenancy** — the link between a connection and the customer
responsible for it during a date range. Start one when a tenant moves
in, end it when they move out.

**Trial balance** — a report listing every account with its current
total. Used by accountants to confirm the books are in balance (debits
equal credits).

**Verify** — a built-in health check (`pond run /accounts verify`)
that catches common problems.

**Void** — to mark a payment or expense as cancelled, by writing a
reversing transaction. The original record stays in place.

---

*Questions, mistakes, weird situations? Save what you typed, save what
the screen said, and reach out to your tech contact. The books are
always recoverable.*
