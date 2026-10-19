package wat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func recordStatement(t *testing.T, books *Books, statement *Statement) {
	t.Helper()
	for index := range statement.Rows {
		draft, entry, err := books.StatementDraft(statement, index)
		require.NoError(t, err)
		entry.Postings[1].Account = "Equity:218:Owner"
		_, err = draft.Preview(entry)
		require.NoError(t, err)
		_, err = draft.Save()
		require.NoError(t, err)
	}
}

func saveBankEntry(t *testing.T, books *Books, date, description, amount string) string {
	t.Helper()
	draft, err := books.NewDraft()
	require.NoError(t, err)
	entry := Entry{
		Date: date, Payee: description,
		Postings: []Posting{
			{Account: "Equity:218:Owner", Amount: amount},
			{Account: "Bank:131.2:Checking", Amount: "-" + amount},
		},
	}
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	return draft.id
}

func TestFinishReconciliationLocksTransactions(t *testing.T) {
	books, statement, _ := statementFixture(t)
	recordStatement(t, books, statement)
	value, err := books.Reconciliation(statement)
	require.NoError(t, err)
	require.Equal(t, len(statement.Rows), value.RowsRecorded)
	require.Equal(t, int64(0), value.BookOpening)
	require.Equal(t, int64(10000), value.StatementCredits)
	require.Equal(t, int64(3234), value.StatementDebits)
	require.Equal(t, int64(10000), value.BookCredits)
	require.Equal(t, int64(3234), value.BookDebits)
	require.Equal(t, int64(6766), value.BookClosing)
	require.Equal(t, int64(6766), value.AdjustedStatement)
	require.Zero(t, value.Difference)
	require.Empty(t, value.Items)

	require.NoError(t, books.FinishReconciliation(statement))
	path := books.reconciliationPath(statement.Hash, "reconciliation.json")
	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	require.NoFileExists(t, books.reconciliationPath(statement.Hash, "reconciliation-work.json"))
	finished, err := books.Reconciliation(statement)
	require.NoError(t, err)
	require.NotNil(t, finished.Checkpoint)
	require.Len(t, finished.Checkpoint.Transactions, len(statement.Rows))
	require.Empty(t, finished.Checkpoint.Outstanding)
	require.ErrorContains(t, books.FinishReconciliation(statement), "already finished")

	entries, err := books.JournalEntries()
	require.NoError(t, err)
	require.Len(t, entries, len(statement.Rows))
	for _, entry := range entries {
		require.Contains(t, entry.ReadOnly, "Locked by finished bank reconciliation")
		_, _, err := books.EditDraft(entry.ID)
		require.ErrorContains(t, err, "locked by finished bank reconciliation")
	}
	_, _, err = books.EditStatementDraft(statement, 0)
	require.ErrorContains(t, err, "locked by finished bank reconciliation")
	require.NoError(t, books.verifyReconciliations())

	journalPath := bankJournal(t, books, statement.BankAccount)
	journal := read(t, journalPath)
	require.NoError(t, os.WriteFile(journalPath,
		[]byte(strings.Replace(journal, "OPENING DEPOSIT", "Changed deposit", 1)), 0600))
	require.ErrorContains(t, books.verifyReconciliations(), "changed after Finish")
	_, err = Open(books.Config)
	require.ErrorContains(t, err, "verify reconciliations")
}

func TestReconciliationPersistsExplicitOutstandingItems(t *testing.T) {
	books, statement, _ := statementFixture(t)
	outstandingID := saveBankEntry(t, books, "2021-04-10", "Uncleared check", "25.00")
	recordStatement(t, books, statement)
	value, err := books.Reconciliation(statement)
	require.NoError(t, err)
	require.Equal(t, int64(4266), value.BookClosing)
	require.Equal(t, int64(10000), value.BookCredits)
	require.Equal(t, int64(5734), value.BookDebits)
	require.Equal(t, int64(6766), value.AdjustedStatement)
	require.Equal(t, int64(-2500), value.Difference)
	require.Len(t, value.Items, 1)
	require.Equal(t, outstandingID, value.Items[0].ID)
	require.False(t, value.Items[0].Outstanding)
	require.ErrorContains(t, books.FinishReconciliation(statement), "unexplained difference must be zero")

	require.NoError(t, books.SetOutstanding(statement, outstandingID, true))
	value, err = books.Reconciliation(statement)
	require.NoError(t, err)
	require.True(t, value.Items[0].Outstanding)
	require.Zero(t, value.OutstandingCredit)
	require.Equal(t, int64(2500), value.OutstandingDebit)
	require.Equal(t, int64(4266), value.AdjustedStatement)
	require.Zero(t, value.Difference)

	reopened, err := Open(books.Config)
	require.NoError(t, err)
	value, err = reopened.Reconciliation(statement)
	require.NoError(t, err)
	require.True(t, value.Items[0].Outstanding, "working selection must survive leaving the screen and restarting")
	require.NoError(t, reopened.FinishReconciliation(statement))
	checkpoint, err := reopened.loadCheckpoint(statement)
	require.NoError(t, err)
	require.Len(t, checkpoint.Outstanding, 1)
	require.Equal(t, outstandingID, checkpoint.Outstanding[0].ID)
	_, _, err = reopened.EditDraft(outstandingID)
	require.NoError(t, err, "outstanding entries are not cleared or locked by this statement")
}

func TestReconciliationRequiresMatchingBookOpening(t *testing.T) {
	books, statement, _ := statementFixture(t)
	saveBankEntry(t, books, "2021-03-31", "Preexisting mistake", "25.00")
	recordStatement(t, books, statement)
	_, err := books.Reconciliation(statement)
	require.ErrorContains(t, err, "cannot begin reconciliation")
	require.ErrorContains(t, err, "book opening balance -25.00")
	require.ErrorContains(t, err, "statement opening 0.00")
	require.ErrorContains(t, books.FinishReconciliation(statement), "cannot begin reconciliation")
	require.NoFileExists(t, books.reconciliationPath(statement.Hash, "reconciliation.json"))
}

func TestReconciliationRequiresRecordedRowsAndEarlierStatements(t *testing.T) {
	books, statement, _ := statementFixture(t)
	require.ErrorContains(t, books.FinishReconciliation(statement), "record every statement row")

	root := books.Config.root
	earlierPDF := filepath.Join(root, "earlier.pdf")
	earlierJSON := filepath.Join(root, "earlier.json")
	earlierData := strings.ReplaceAll(statementJSON, "2021-04-", "2021-02-")
	earlierData = strings.ReplaceAll(earlierData, "2021-05-", "2021-03-")
	require.NoError(t, os.WriteFile(earlierPDF, []byte("%PDF-1.7\nearlier\n"), 0600))
	require.NoError(t, os.WriteFile(earlierJSON, []byte(earlierData), 0600))
	_, err := books.LoadStatement(earlierPDF, StatementOptions{Extraction: earlierJSON}, nil)
	require.NoError(t, err)
	_, err = books.Reconciliation(statement)
	require.ErrorContains(t, err, "finish the earlier statement")
}

func TestReconciliationOrderingIsIndependentPerBankAccount(t *testing.T) {
	books, statement, _ := statementFixture(t)
	addBankAccount(t, books, "Bank:131.2:Savings")
	root := books.Config.root
	earlierPDF := filepath.Join(root, "earlier-savings.pdf")
	earlierJSON := filepath.Join(root, "earlier-savings.json")
	earlierData := strings.NewReplacer(
		"2021-04-", "2021-02-",
		"2021-05-", "2021-03-",
		`"account_id":"Checking"`, `"account_id":"Savings"`,
	).Replace(statementJSON)
	require.NoError(t, os.WriteFile(earlierPDF, []byte("%PDF-1.7\nearlier savings\n"), 0600))
	require.NoError(t, os.WriteFile(earlierJSON, []byte(earlierData), 0600))
	earlier, err := books.LoadStatement(earlierPDF, StatementOptions{Extraction: earlierJSON}, nil)
	require.NoError(t, err)
	require.Equal(t, "Bank:131.2:Savings", earlier.BankAccount)

	_, err = books.Reconciliation(statement)
	require.NoError(t, err, "an unfinished statement for another account must not block reconciliation")
}

func TestReconciliationRejectsChangedStatementEvidence(t *testing.T) {
	books, statement, _ := statementFixture(t)
	recordStatement(t, books, statement)
	require.NoError(t, books.FinishReconciliation(statement))
	cache := filepath.Dir(filepath.Join(books.Config.root, statement.Source))
	extracted := read(t, filepath.Join(cache, "extracted.json"))
	require.NoError(t, os.WriteFile(filepath.Join(cache, "extracted.json"),
		[]byte(strings.Replace(extracted, `"end":"2021-05-31"`, `"end":"2021-06-01"`, 1)), 0600))
	changed, err := books.cachedStatement(statement.Hash)
	require.NoError(t, err)
	_, err = books.loadCheckpoint(changed)
	require.ErrorContains(t, err, "no longer matches")
}

func TestReconciliationRejectsIncompleteCheckpoint(t *testing.T) {
	books, statement, _ := statementFixture(t)
	recordStatement(t, books, statement)
	require.NoError(t, books.FinishReconciliation(statement))
	path := books.reconciliationPath(statement.Hash, "reconciliation.json")
	var checkpoint ReconciliationCheckpoint
	require.NoError(t, decodeJSONFile(path, &checkpoint))
	checkpoint.Transactions = checkpoint.Transactions[1:]
	require.NoError(t, writePrivateJSON(path, checkpoint))
	_, err := books.loadCheckpoint(statement)
	require.ErrorContains(t, err, "cover every statement row")
	_, err = Open(books.Config)
	require.ErrorContains(t, err, "verify reconciliations")
}

func TestFinishReconciliationHonorsWriterLock(t *testing.T) {
	books, statement, _ := statementFixture(t)
	recordStatement(t, books, statement)
	lock := bankJournal(t, books, statement.BankAccount) + ".wat-lock"
	require.NoError(t, os.WriteFile(lock, []byte("busy"), 0600))
	require.ErrorContains(t, books.FinishReconciliation(statement), "acquire journal lock")
	require.NoFileExists(t, books.reconciliationPath(statement.Hash, "reconciliation.json"))
	require.NoError(t, os.Remove(lock))
	require.NoError(t, books.FinishReconciliation(statement))
}
