# Historical Ledger migration

`ledger-migrate` converts the four authoritative billing CSV files into
deterministic, balanced Ledger journals:

```sh
go run ./cmd/ledger-migrate \
  -users users.csv \
  -business business.csv \
  -cycles cycles.csv \
  -payments payments.csv \
  -output ledger-history
```

The output directory must not already exist. It contains:

- `accounts.journal` — chart of accounts and customer receivable accounts;
- `books.journal` — includes the chart and every historical cycle; and
- `cycles/YYYY-MM.journal` — expenses, customer assessments, and payments for
  one six-month service cycle, named by closing month.

Payments are assigned to the service cycle containing their payment date.
Historical effective-connection shares that were never billed to a recorded
customer remain unbilled and are reported in cycle comments and command
summaries for later policy work. Migration fails for payments outside cycle
coverage, unknown or duplicate accounts, duplicate payments, or cycle gaps.
The source CSV files remain authoritative and should be retained.
