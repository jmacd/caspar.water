package wat

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func configureVendors(t *testing.T, books *Books, text string) {
	t.Helper()
	configureVendorFiles(t, books, text)
}

func configureVendorFiles(t *testing.T, books *Books, texts ...string) {
	t.Helper()
	books.Config.Vendors = nil
	for index, text := range texts {
		path := filepath.Join(books.Config.root, fmt.Sprintf("vendors-%d.csv", index+1))
		require.NoError(t, os.WriteFile(path, []byte(text), 0600))
		books.Config.Vendors = append(books.Config.Vendors, path)
	}
}

func TestCheckRequiresRegisteredVendor(t *testing.T) {
	journal := `2021-01-01 PG&E | Electricity
    Expense:615.000:Utility  $12.34
    Equity:218.000:Owner  $-12.34
`
	books, _ := fixture(t, journal)
	configureVendors(t, books, "name\nPG&E\n")
	require.NoError(t, books.Check())

	configureVendors(t, books, "name\nElectric Company\n")
	err := books.Check()
	require.ErrorContains(t, err, `transaction payee "PG&E" is not registered`)
	require.ErrorContains(t, err, "configured vendor files")
}

func TestCheckMergesVendorFiles(t *testing.T) {
	journal := `2021-01-01 PG&E | Electricity
    Expense:615.000:Utility  $12.34
    Equity:218.000:Owner  $-12.34
`
	books, _ := fixture(t, journal)
	configureVendorFiles(t, books, "name\nFeiner Fixings\n", "name\nPG&E\n")
	require.NoError(t, books.Check())

	configureVendorFiles(t, books, "name\nPG&E\n", "name\nPG&E\n")
	err := books.Check()
	require.ErrorContains(t, err, `vendor "PG&E" is defined in both`)
}

func TestCheckAcceptsRegisteredVendorWithSplitExpenses(t *testing.T) {
	journal := `2021-01-01 Feiner Fixings | Water treatment
    Expense:615.000:Utility  $40.00
    Expense:615.000:Utility  $200.00
    Equity:218.000:Owner  $-240.00
`
	books, _ := fixture(t, journal)
	configureVendors(t, books, "name\nFeiner Fixings\n")
	require.NoError(t, books.Check())
}

func TestCheckAcceptsBankDeposit(t *testing.T) {
	journal := `2021-01-01 Deposit | Customer payment
    Bank:131.200:Checking  $100.00
    Equity:218.000:Owner  $-100.00
`
	books, _ := fixture(t, journal)
	configureVendors(t, books, "name\nPG&E\n")
	require.NoError(t, books.Check())
}

func TestCheckRequiresRegisteredNonexpenseCounterparty(t *testing.T) {
	journal := `2021-01-01 Owner | Contribution
    Bank:131.200:Checking  $100.00
    Equity:218.000:Owner  $-100.00
`
	books, _ := fixture(t, journal)
	configureVendors(t, books, "name\nPG&E\n")
	require.ErrorContains(t, books.Check(), `transaction payee "Owner" is not registered`)

	require.NoError(t, os.WriteFile(
		books.Config.ManualJournal,
		[]byte(`2021-01-01 Journal | Owner contribution
    Bank:131.200:Checking  $100.00
    Equity:218.000:Owner  $-100.00
`),
		0600,
	))
	require.NoError(t, books.Check())
}

func TestCheckAcceptsJournalExpenseWithoutMetadata(t *testing.T) {
	journal := `2021-01-01 Journal | Owner-contributed utility expense
    Expense:615.000:Utility  $12.34
    Equity:218.000:Owner  $-12.34
`
	books, _ := fixture(t, journal)
	configureVendors(t, books, "name\nPG&E\n")
	require.NoError(t, books.Check())
}

func TestCheckRequiresStrictPayeeMemoSyntax(t *testing.T) {
	for _, description := range []string{
		"PG&E",
		"PG&E|Electricity",
		"PG&E  | Electricity",
		"PG&E |  Electricity",
		"PG&E | Electricity | January",
		"PG&E |",
		"| Electricity",
	} {
		t.Run(description, func(t *testing.T) {
			journal := "2021-01-01 " + description + "\n" +
				"    Expense:615.000:Utility  $12.34\n" +
				"    Equity:218.000:Owner  $-12.34\n"
			books, _ := fixture(t, journal)
			configureVendors(t, books, "name\nPG&E\n")
			require.ErrorContains(t, books.Check(), "transaction description must")
		})
	}
}

func TestCheckReservedPayeeSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, journal, message string
	}{
		{
			name: "deposit expense",
			journal: `2021-01-01 Deposit | Vendor refund
    Bank:131.200:Checking  $12.34
    Expense:615.000:Utility  $-12.34
`,
			message: "must not contain Expense postings",
		},
		{
			name: "deposit without bank",
			journal: `2021-01-01 Deposit | Customer payment
    Equity:218.000:Owner  $12.34
    Income:421.000:Misc  $-12.34
`,
			message: "requires a Bank posting",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			books, _ := fixture(t, tc.journal)
			configureVendors(t, books, "name\nPG&E\n")
			require.ErrorContains(t, books.Check(), tc.message)
		})
	}
}

func TestVendorCSVValidation(t *testing.T) {
	for _, tc := range []struct {
		name, csv, message string
	}{
		{name: "header", csv: "vendor\nPG&E\n", message: "header must be exactly"},
		{name: "extra column", csv: "name\nPG&E,extra\n", message: "exactly one name"},
		{name: "blank", csv: "name\n\n", message: "at least one vendor"},
		{name: "whitespace", csv: "name\n PG&E\n", message: "invalid canonical name"},
		{name: "delimiter", csv: "name\nPG&E | Utility\n", message: "invalid canonical name"},
		{name: "journal", csv: "name\nJournal\n", message: "reserved transaction payee"},
		{name: "deposit", csv: "name\nDeposit\n", message: "reserved transaction payee"},
		{name: "duplicate", csv: "name\nPG&E\nPG&E\n", message: "duplicates vendor"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			require.NoError(t, err)
			path := filepath.Join(dir, "vendors.csv")
			require.NoError(t, os.WriteFile(path, []byte(tc.csv), 0600))
			_, err = readVendors(path)
			require.ErrorContains(t, err, tc.message)
		})
	}
}
