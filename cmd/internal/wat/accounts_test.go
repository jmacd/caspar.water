package wat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPostingAccountShape(t *testing.T) {
	for _, tc := range []struct {
		account string
		valid   bool
	}{
		{"Asset", false},
		{"Asset:131.2-Cash-in-Bank", false},
		{"Asset:131.2:SBMC", true},
		{"Bank:131.2:SBMC:Checking", true},
		{"Expense:650.1-Contract-Water-Quality-and-Testing-Work", false},
		{"Expense:650.1:Vendor", true},
		{"Asset::SBMC", false},
		{":131.2:SBMC", false},
		{"Asset:131.2:", false},
		{"Asset:131.2:   ", false},
		{"Asset:131.2:SBMC:", false},
		{"Assets:131.2:SBMC", false},
	} {
		t.Run(tc.account, func(t *testing.T) {
			_, valid := parsePostingAccount(tc.account)
			require.Equal(t, tc.valid, valid)
		})
	}
}

func addAccountDeclarations(t *testing.T, books *Books, text string) {
	t.Helper()
	path := filepath.Join(books.Config.root, "books", "accounts.journal")
	require.NoError(t, os.WriteFile(path, []byte(read(t, path)+text), 0600))
}

func TestPostingAccountsResolveThroughCPUCChart(t *testing.T) {
	books, _ := fixture(t, "")
	accounts, err := books.PostingAccounts()
	require.NoError(t, err)
	byName := make(map[string]PostingAccount)
	for _, account := range accounts {
		byName[account.Name] = account
	}
	require.Equal(t, PostingAccount{
		Name: "Bank:131.2:Checking", Category: "Bank", Number: "131.2",
		FormalCategory: "Asset", FormalName: "Cash-in-Bank",
	}, byName["Bank:131.2:Checking"])
	require.Equal(t, "Power", byName["Expense:615:Utility"].FormalName)
}

func TestPostingAccountsRejectUnknownAndMismatchedCPUCNumbers(t *testing.T) {
	for _, tc := range []struct {
		name, declaration, message string
	}{
		{"unknown", "account Expense:999:Unknown\n", "not in account_chart"},
		{"wrong category", "account Expense:131.2:Wrong\n", "classifies it as Asset"},
		{"Bank must be asset", "account Bank:615:Wrong\n", "classifies it as Expense"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			books, _ := fixture(t, "")
			addAccountDeclarations(t, books, tc.declaration)
			_, err := books.Accounts()
			require.ErrorContains(t, err, tc.message)
		})
	}
}

func TestAccountChartRejectsConflictingNumberMappings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.journal")
	require.NoError(t, os.WriteFile(path, []byte(
		"account Asset:131.2-Cash-in-Bank\naccount Expense:131.2-Conflicting-Expense\n",
	), 0600))
	_, err := loadAccountChart(path)
	require.ErrorContains(t, err, "conflicts with CPUC account 131.2")
}

func TestPostingChoicesRequireDepthNotLeaves(t *testing.T) {
	books, _ := fixture(t, "")
	addAccountDeclarations(t, books, "account Expense:615:PGE\naccount Expense:615:PGE:Shop\n")
	accounts, err := books.Accounts()
	require.NoError(t, err)
	require.Contains(t, accounts, "Expense:615:PGE", "three-level accounts remain postable even when they have children")
	require.Contains(t, accounts, "Expense:615:PGE:Shop")
	require.Equal(t,
		[]string{"Expense:615:PGE", "Expense:615:PGE:Shop", "Expense:615:Utility"},
		matchingAccounts(accounts, "615"))
	help := editorHelp(2021, accounts)
	require.Contains(t, help, "Expense:615:PGE\n")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	entry := testEntry()
	entry.Postings[0].Account = "Expense:615:PGE"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	require.NoError(t, books.Check())
}

func TestExistingShallowEntriesCanBeMigratedIndividually(t *testing.T) {
	books, _ := fixture(t, "")
	addAccountDeclarations(t, books, "account Expense:615-Power\naccount Expense:615:PGE\n")
	entry := testEntry()
	entry.Postings[0].Account = "Expense:615-Power"
	first := entry.journalText("first")
	second := entry.journalText("second")
	original := first + "\n" + second
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(original), 0600))
	_, err := Open(books.Config)
	require.NoError(t, err, "legacy entries must not prevent opening the app to correct them")
	require.ErrorContains(t, books.Check(), "Category:CPUC-number:LocalName")
	for _, id := range []string{"first", "second"} {
		draft, revised, err := books.EditDraft(id)
		require.NoError(t, err)
		require.Equal(t, "Expense:615-Power", revised.Postings[0].Account)
		_, err = draft.Save()
		require.Error(t, err, "verifying the old source must not make it savable without validation")
		_, err = draft.Preview(revised)
		require.ErrorContains(t, err, "Category:CPUC-number:LocalName")
		require.ErrorContains(t, draft.PreviewText(revised.journalText(id)), "Category:CPUC-number:LocalName")
		revised.Postings[0].Account = "Expense:615:PGE"
		_, err = draft.Preview(revised)
		require.NoError(t, err)
		_, err = draft.Save()
		require.NoError(t, err)
		if id == "first" {
			require.Contains(t, read(t, books.Config.ManualJournal), second, "other legacy entries must stay untouched")
			require.Error(t, books.Check())
		}
	}
	require.NoError(t, books.Check())
	require.Equal(t, 2, strings.Count(read(t, books.Config.ManualJournal), "Expense:615:PGE"))
}

func TestNewEntriesRejectShallowAccountsInFormAndText(t *testing.T) {
	books, _ := fixture(t, "")
	addAccountDeclarations(t, books, "account Expense:615-Power\naccount Expense:615:PGE\n")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	entry := testEntry()
	entry.Postings[0].Account = "Expense:615-Power"
	_, err = draft.Preview(entry)
	require.ErrorContains(t, err, "Category:CPUC-number:LocalName")
	require.ErrorContains(t, draft.PreviewText(entry.journalText(draft.id)), "Category:CPUC-number:LocalName")
	_, err = draft.Save()
	require.Error(t, err)
	require.Empty(t, read(t, books.Config.ManualJournal))
	entry.Postings[0].Account = "Expense:615:PGE"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	require.NoError(t, books.Check())
}

func TestBankAccountRenameRequiresStatementRebinding(t *testing.T) {
	books, statement, _ := statementFixture(t)
	draft, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	entry.Postings[1].Account = "Expense:615:Utility"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	newAccount := "Bank:131.2:SBMC"
	addAccountDeclarations(t, books, "account "+newAccount+"\n")
	journal := bankJournal(t, books, statement.BankAccount)
	original := read(t, journal)
	require.NoError(t, os.WriteFile(journal, []byte(strings.ReplaceAll(original, statement.BankAccount, newAccount)), 0600))
	cfg := books.Config
	cfg.BankAccounts[0].Account = newAccount
	_, err = Open(cfg)
	require.ErrorContains(t, err, "statement account identifier must match exactly one configured bank account")
}
