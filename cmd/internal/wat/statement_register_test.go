package wat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func writeStatementRegister(t *testing.T, books *Books, rows string) {
	t.Helper()
	path := filepath.Join(books.Config.root, "register.csv")
	require.NoError(t, os.WriteFile(path, []byte(
		"Date,Type,Ref No.,Payee,Memo,Account,Payment,Deposit,Balance\n"+rows,
	), 0600))
	books.Config.Statement.QuickBooksRegister = path
}

func TestStatementRegisterPrefillsMappedExactMatch(t *testing.T) {
	books, statement, _ := statementFixture(t)
	writeStatementRegister(t, books,
		"4/5/2021,Expense,44,Electric Company,April power,Utilities,12.34,,87.66\n",
	)
	require.NoError(t, books.saveQuickBooksMappings([]quickBooksAccountMapping{{
		Label: "Utilities", Account: "Expense:615.000:Utility",
	}}))

	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.Contains(t, statuses[1].QuickBooksEvidence, "exact match")
	require.Contains(t, statuses[1].QuickBooksEvidence, "Expense:615.000:Utility")

	_, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, "Electric Company", entry.Payee)
	require.Equal(t, "April power", entry.Description)
	require.Equal(t, "44", entry.Reference)
	require.Equal(t, "Expense:615.000:Utility", entry.Postings[1].Account)
	require.Equal(t, "12.34", entry.Postings[1].Amount)
}

func TestStatementRegisterPrefillsTextButNotUnmappedAccount(t *testing.T) {
	books, statement, _ := statementFixture(t)
	writeStatementRegister(t, books,
		"4/1/2021,Deposit,,Customer,Opening deposit,Reimbursable,,100.00,100.00\n",
	)

	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.Contains(t, statuses[0].QuickBooksEvidence, `account "Reimbursable" is unmapped`)

	_, entry, err := books.StatementDraft(statement, 0)
	require.NoError(t, err)
	require.Equal(t, "Customer", entry.Payee)
	require.Equal(t, "Opening deposit", entry.Description)
	require.Empty(t, entry.Postings[1].Account)
}

func TestStatementRegisterLeavesAmbiguousMatchBlank(t *testing.T) {
	books, statement, _ := statementFixture(t)
	writeStatementRegister(t, books,
		"4/5/2021,Expense,1,First vendor,First memo,Utilities,12.34,,87.66\n"+
			"4/5/2021,Expense,2,Second vendor,Second memo,Utilities,12.34,,87.66\n",
	)

	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.Contains(t, statuses[1].QuickBooksEvidence, "2 exact date/amount matches")

	_, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, statement.Rows[1].Description, entry.Payee)
	require.Empty(t, entry.Description)
	require.Empty(t, entry.Reference)
	require.Empty(t, entry.Postings[1].Account)
}

func TestStatementRegisterStrengthensAmbiguousMatchByReference(t *testing.T) {
	row := StatementRow{Date: "2021-04-05", Description: "CHECK 44", Cents: -1234}
	matches := []quickBooksRecord{
		{Fields: map[string]string{"Ref No.": "43"}},
		{Fields: map[string]string{"Ref No.": "44"}},
	}
	require.Equal(t, matches[1:], strengthenStatementRegisterMatches(row, matches))
}

func TestStatementRegisterNoMatchPreservesLearnedDefault(t *testing.T) {
	books, statement, _ := statementFixture(t)
	writeStatementRegister(t, books,
		"4/6/2021,Expense,,Other vendor,Other memo,Utilities,12.34,,87.66\n",
	)
	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.Empty(t, statuses[1].QuickBooksEvidence)
	_, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, statement.Rows[1].Description, entry.Payee)
	require.Empty(t, entry.Postings[1].Account)
}
