package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jmacd/caspar.water/cmd/internal/billing/ledger"
	"github.com/spf13/afero"
)

var (
	usersFile    = flag.String("users", "users.csv", "authoritative customer CSV")
	businessFile = flag.String("business", "business.csv", "authoritative business CSV")
	cyclesFile   = flag.String("cycles", "cycles.csv", "authoritative billing-cycle CSV")
	paymentsFile = flag.String("payments", "payments.csv", "authoritative payment CSV")
	outputDir    = flag.String("output", "ledger-history", "new output directory")
)

func main() {
	flag.Parse()

	fs := afero.NewOsFs()
	result, err := ledger.Generate(ledger.Inputs{
		UsersFile:    *usersFile,
		BusinessFile: *businessFile,
		CyclesFile:   *cyclesFile,
		PaymentsFile: *paymentsFile,
	}, fs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate ledger history:", err)
		os.Exit(1)
	}
	if err := ledger.Write(*outputDir, result, fs); err != nil {
		fmt.Fprintln(os.Stderr, "write ledger history:", err)
		os.Exit(1)
	}

	for _, cycle := range result.Cycles {
		fmt.Printf(
			"%s expenses=%s assessments=%s payments=%s unassigned=%s\n",
			cycle.Name,
			displayCents(cycle.Expenses),
			displayCents(cycle.Assessments),
			displayCents(cycle.Payments),
			displayCents(cycle.Unassigned),
		)
	}
}

func displayCents(cents int64) string {
	return fmt.Sprintf("$%d.%02d", cents/100, cents%100)
}
