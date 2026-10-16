package wat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostingAccountDepth(t *testing.T) {
	for _, tc := range []struct {
		account string
		valid   bool
	}{
		{"Asset", false},
		{"Asset:131.2-Cash-in-Bank", false},
		{"Asset:131.2-Cash-in-Bank:SBMC", true},
		{"Asset:131.2-Cash-in-Bank:SBMC:Checking", true},
		{"Expense:650.1-Contract-Water-Quality-and-Testing-Work", false},
		{"Expense:650.1-Contract-Water-Quality-and-Testing-Work:Vendor", true},
		{"Asset::SBMC", false},
		{":131.2:SBMC", false},
		{"Asset:131.2:", false},
		{"Asset:131.2:   ", false},
		{"Asset:131.2:SBMC:", false},
	} {
		t.Run(tc.account, func(t *testing.T) {
			require.Equal(t, tc.valid, detailedAccount(tc.account))
		})
	}
}

func addAccountDeclarations(t *testing.T, books *Books, text string) {
	t.Helper()
	path := filepath.Join(books.Config.root, "books", "accounts.journal")
	require.NoError(t, os.WriteFile(path, []byte(read(t, path)+text), 0600))
}

func TestPostingChoicesRequireDepthNotLeaves(t *testing.T) {
	books, _ := fixture(t, "")
	addAccountDeclarations(t, books, "account Expense:615-Power\naccount Expense:615-Power:PGE\naccount Expense:615-Power:PGE:Shop\n")
	accounts, err := books.Accounts()
	require.NoError(t, err)
	require.NotContains(t, accounts, "Expense:615-Power")
	require.Contains(t, accounts, "Expense:615-Power:PGE", "three-level accounts remain postable even when they have children")
	require.Contains(t, accounts, "Expense:615-Power:PGE:Shop")
	require.Equal(t, []string{"Expense:615-Power:PGE", "Expense:615-Power:PGE:Shop"}, matchingAccounts(accounts, "615"))
	help := editorHelp(2021, accounts)
	require.NotContains(t, help, "Expense:615-Power\n")
	require.Contains(t, help, "Expense:615-Power:PGE\n")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	entry := testEntry()
	entry.Postings[0].Account = "Expense:615-Power:PGE"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	require.NoError(t, books.Check())
}

func TestExistingShallowEntriesCanBeMigratedIndividually(t *testing.T) {
	books, _ := fixture(t, "")
	addAccountDeclarations(t, books, "account Expense:615-Power\naccount Expense:615-Power:PGE\n")
	entry := testEntry()
	entry.Postings[0].Account = "Expense:615-Power"
	first := entry.journalText("first")
	second := entry.journalText("second")
	original := first + "\n" + second
	require.NoError(t, os.WriteFile(books.Config.WriteJournal, []byte(original), 0600))
	_, err := Open(books.Config)
	require.NoError(t, err, "legacy entries must not prevent opening the app to correct them")
	require.ErrorContains(t, books.Check(), "at least three nonempty levels")
	for _, id := range []string{"first", "second"} {
		draft, revised, err := books.EditDraft(id)
		require.NoError(t, err)
		require.Equal(t, "Expense:615-Power", revised.Postings[0].Account)
		_, err = draft.Save()
		require.Error(t, err, "verifying the old source must not make it savable without validation")
		_, err = draft.Preview(revised)
		require.ErrorContains(t, err, "at least three")
		require.ErrorContains(t, draft.PreviewText(revised.journalText(id)), "at least three")
		revised.Postings[0].Account = "Expense:615-Power:PGE"
		_, err = draft.Preview(revised)
		require.NoError(t, err)
		_, err = draft.Save()
		require.NoError(t, err)
		if id == "first" {
			require.Contains(t, read(t, books.Config.WriteJournal), second, "other legacy entries must stay untouched")
			require.Error(t, books.Check())
		}
	}
	require.NoError(t, books.Check())
	require.Equal(t, 2, strings.Count(read(t, books.Config.WriteJournal), "Expense:615-Power:PGE"))
}

func TestNewEntriesRejectShallowAccountsInFormAndText(t *testing.T) {
	books, _ := fixture(t, "")
	addAccountDeclarations(t, books, "account Expense:615-Power\naccount Expense:615-Power:PGE\n")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	entry := testEntry()
	entry.Postings[0].Account = "Expense:615-Power"
	_, err = draft.Preview(entry)
	require.ErrorContains(t, err, "at least three")
	require.ErrorContains(t, draft.PreviewText(entry.journalText(draft.id)), "at least three")
	_, err = draft.Save()
	require.Error(t, err)
	require.Empty(t, read(t, books.Config.WriteJournal))
	entry.Postings[0].Account = "Expense:615-Power:PGE"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	require.NoError(t, books.Check())
}

func TestBankAccountRenamePreservesStatementIdentity(t *testing.T) {
	books, statement, _ := statementFixture(t)
	draft, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	entry.Postings[1].Account = "Expenses:Power:Utility"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	newAccount := "Assets:131.2-Cash-in-Bank:SBMC"
	addAccountDeclarations(t, books, "account Assets:131.2-Cash-in-Bank\naccount "+newAccount+"\n")
	original := read(t, books.Config.WriteJournal)
	require.NoError(t, os.WriteFile(books.Config.WriteJournal, []byte(strings.ReplaceAll(original, books.Config.BankAccount, newAccount)), 0600))
	cfg := books.Config
	cfg.BankAccount = "Assets:131.2-Cash-in-Bank"
	_, err = Open(cfg)
	require.ErrorContains(t, err, "at least three levels")
	cfg.BankAccount = newAccount
	renamed, err := Open(cfg)
	require.NoError(t, err)
	statuses, err := renamed.StatementStatuses(statement)
	require.NoError(t, err)
	require.True(t, statuses[1].Recorded)
	revision, revised, err := renamed.EditStatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, draft.id, revision.id)
	require.Equal(t, statement.key(1), revised.BankRow)
	require.Equal(t, newAccount, revised.Postings[0].Account)
	revised.Description = "After account rename"
	_, err = revision.Preview(revised)
	require.NoError(t, err)
	_, err = revision.Save()
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(read(t, cfg.WriteJournal), "; bank-row: "+statement.key(1)))
}
