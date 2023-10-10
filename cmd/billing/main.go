package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jmacd/caspar.water/cmd/billing/internal/logic"
	"github.com/spf13/afero"
)

var (
	usersFile     = flag.String("users", "users.csv", "csv")
	businessFile  = flag.String("business", "business.csv", "csv")
	cyclesFile    = flag.String("cycles", "cycles.csv", "csv")
	paymentsFile  = flag.String("payments", "payments.csv", "csv")
	statementsDir = flag.String("statements", "statements", "input directory")
)

func main() {
	flag.Parse()

	result, err := logic.Logic(logic.Inputs{
		InitialConnectionCount: 13, // TODO: existing logic
		UsersFile:              *usersFile,
		BusinessFile:           *businessFile,
		CyclesFile:             *cyclesFile,
		PaymentsFile:           *paymentsFile,
		StatementsDir:          *statementsDir,
	}, afero.NewOsFs())
	if err != nil {
		fmt.Printf("command failed: %v", err)
		os.Exit(1)
	}

	if err := logic.Output(result); err != nil {
		fmt.Println("output failed:", err)
		os.Exit(1)
	}

}
