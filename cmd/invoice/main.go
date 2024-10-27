package main

import (
	"flag"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/jmacd/caspar.water/cmd/internal/billing/business"
	"github.com/jmacd/caspar.water/cmd/internal/billing/csv"
	"github.com/jmacd/caspar.water/cmd/internal/billing/currency"
	"github.com/jmacd/caspar.water/cmd/internal/billing/invoice"
	"github.com/jmacd/caspar.water/cmd/internal/billing/user"
	"github.com/jmacd/maroto/pkg/pdf"
	"github.com/spf13/afero"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

var (
	inputFile    = flag.String("input", "invoice.yaml", "input yaml file")
	outputFile   = flag.String("output", "output.pdf", "output pdf file")
	usersFile    = flag.String("users", "users.csv", "csv")
	businessFile = flag.String("business", "business.csv", "csv")
)

type Invoice struct {
	JobName string `yaml:"job_name"`
	Account string `yaml:"account"`
	Date    string `yaml:"date"`
	Items   []Item `yaml:"items"`
}

type Item struct {
	Description string          `yaml:"desc"`
	Amount      currency.Amount `yaml:"amount"`
}

var _ invoice.Document = &Invoice{}

func (inv *Invoice) FullDate() string {
	return inv.Date
}

func (inv *Invoice) InvoiceName() string {
	return strings.ReplaceAll(
		cases.Title(language.English, cases.NoLower).String(inv.JobName),
		" ",
		"")
}

func (*Invoice) BodyText() (string, error) {
	return "", nil
}

func main() {
	fs := afero.NewOsFs()
	flag.Parse()

	data, err := os.ReadFile(*inputFile)
	if err != nil {
		log.Fatalf("cannot read file: %v: %v", *inputFile, err)
	}

	var inv Invoice

	if err = yaml.Unmarshal([]byte(data), &inv); err != nil {
		log.Fatalf("cannot unmarshal data: %v", err)
	}

	// Users
	users, err := csv.ReadFile[user.User](*usersFile, fs)
	if err != nil {
		log.Fatalf("read users file: %v: %v", *usersFile, err)
	}

	uidx := slices.IndexFunc(users, func(u user.User) bool {
		return u.AccountName == inv.Account
	})
	if uidx < 0 {
		log.Fatalf("invalid user account: %v: %v", inv.Account, err)
	}

	// Business
	business, err := csv.ReadFile[business.Business](*businessFile, fs)
	if err != nil {
		log.Fatalf("read business file: %v: %v", *businessFile, err)
	}
	if len(business) != 1 {
		log.Fatalf("business file should have one row")
	}

	print, err := invoice.MakeInvoice(business[0], users[uidx], &inv, inv.mainContent)

	if err := print.OutputFileAndClose(*outputFile); err != nil {
		log.Fatalf("command failed: %v", err)
	}
}

func (inv *Invoice) mainContent(m pdf.Maroto) {
	var lines [][]string
	var total currency.Amount

	for _, item := range inv.Items {
		lines = append(lines, []string{
			item.Description,
			item.Amount.Display(),
		})
		total = currency.Sum(total, item.Amount)
	}
	lines = append(lines, []string{"", ""})
	lines = append(lines, []string{"Total", total.Display()})

	m.Row(2, func() {
		m.TableList([]string{
			"Item",
			"Amount",
			"",
		}, lines, invoice.TableStyle)
	})
}
