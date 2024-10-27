# Invoice generator

This tool is meant for irregularly-generated invoices, as opposed to
the [billing program for recurring payments](../billing/README.md).

## Run the Caspar Water billing program

There are four CSV files named `business.csv`, `cycles.csv`, `payments.csv`, and `users.csv`.

1. Edit the four CSV files with the current cycle's expenses.  If this is an even cycle it includes yearly tax and insurance payments; odd cycles must set zero for these accounts.
2. Download or symlink the CSV files to the top of the repository
3. Create or symlink the file `statements/YYYY-MMM.txt` with body text.
4. Run `go run ./cmd/billing`
5. Distributed the PDFs found in `./Statements/YYYY-MMM`.
