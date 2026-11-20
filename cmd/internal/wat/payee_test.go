package wat

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func configurePayees(t *testing.T, books *Books, names ...string) {
	t.Helper()
	text := ""
	for _, name := range names {
		text += "payee " + name + "\n"
	}
	require.NoError(t, os.WriteFile(books.Config.Payees, []byte(text), 0600))
}

func TestCheckUsesHledgerDeclaredPayees(t *testing.T) {
	journal := `2021-01-01 PG&E | Electricity
    Expense:615.000:Utility  $12.34
    Equity:218.000:Owner  $-12.34
`
	books, _ := fixture(t, journal)
	configurePayees(t, books, "PG&E")
	require.NoError(t, books.Check())

	configurePayees(t, books, "Electric Company")
	err := books.Check()
	require.ErrorContains(t, err, `payee "PG&E" has not been declared`)
	require.ErrorContains(t, err, "payee PG&E")
}

func TestCheckAcceptsDeclaredPayeeWithSplitExpenses(t *testing.T) {
	journal := `2021-01-01 Feiner Fixings | Water treatment
    Expense:615.000:Utility  $40.00
    Expense:615.000:Utility  $200.00
    Equity:218.000:Owner  $-240.00
`
	books, _ := fixture(t, journal)
	configurePayees(t, books, "Feiner Fixings")
	require.NoError(t, books.Check())
}

func TestCheckAcceptsBankDeposit(t *testing.T) {
	journal := `2021-01-01 Deposit | Customer payment
    Bank:131.200:Checking  $100.00
    Equity:218.000:Owner  $-100.00
`
	books, _ := fixture(t, journal)
	configurePayees(t, books, "Deposit")
	require.NoError(t, books.Check())
}

func TestCheckAcceptsJournalExpenseWithoutMetadata(t *testing.T) {
	journal := `2021-01-01 Journal | Owner-contributed utility expense
    Expense:615.000:Utility  $12.34
    Equity:218.000:Owner  $-12.34
`
	books, _ := fixture(t, journal)
	configurePayees(t, books, "Journal")
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
			configurePayees(t, books, "PG&E")
			require.ErrorContains(t, books.Check(), "transaction description must")
		})
	}
}

func TestCheckDepositSemantics(t *testing.T) {
	for _, tc := range []struct {
		name, journal, message string
	}{
		{
			name: "expense",
			journal: `2021-01-01 Deposit | Vendor refund
    Bank:131.200:Checking  $12.34
    Expense:615.000:Utility  $-12.34
`,
			message: "must not contain Expense postings",
		},
		{
			name: "without bank",
			journal: `2021-01-01 Deposit | Customer payment
    Equity:218.000:Owner  $12.34
    Income:421.000:Misc  $-12.34
`,
			message: "requires a Bank posting",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			books, _ := fixture(t, tc.journal)
			configurePayees(t, books, "Deposit")
			require.ErrorContains(t, books.Check(), tc.message)
		})
	}
}
