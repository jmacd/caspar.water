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

func configureAccounts(t *testing.T, books *Books, accounts ...string) {
	t.Helper()
	books.Config.Accounts = append(books.Config.Accounts, accounts...)
	require.NoError(t, prepareWorkspace(books.Config))
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
		FormalComment: "Cash held in a financial institution.",
	}, byName["Bank:131.2:Checking"])
	require.Equal(t, "Power", byName["Expense:615:Utility"].FormalName)
}

func TestPostingAccountsRejectUnknownAndMismatchedCPUCNumbers(t *testing.T) {
	for _, tc := range []struct {
		name, declaration, message string
	}{
		{"unknown", "account Expense:999:Unknown\n", "not in the CPUC chart"},
		{"wrong category", "account Expense:131.2:Wrong\n", "classifies it as Asset"},
		{"Bank must be asset", "account Bank:615:Wrong\n", "classifies it as Expense"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			books, _ := fixture(t, "")
			account := strings.TrimSpace(strings.TrimPrefix(tc.declaration, "account "))
			cfg := books.Config
			if strings.HasPrefix(account, "Bank:") {
				cfg.BankAccounts = append(cfg.BankAccounts, BankAccountConfig{
					Account: account,
					Journal: filepath.Join(cfg.root, booksDirectory, bankJournalFilename(account)),
				})
			} else {
				cfg.Accounts = append(cfg.Accounts, account)
			}
			_, err := Open(cfg)
			require.ErrorContains(t, err, tc.message)
		})
	}
}

func TestAccountChartValidation(t *testing.T) {
	for _, tc := range []struct {
		name, chart, message string
	}{
		{
			name: "duplicate number",
			chart: `version: 1
accounts:
  - category: Asset
    number: "131.2"
    name: Cash-in-Bank
  - category: Expense
    number: "131.2"
    name: Conflicting-Expense
`,
			message: "duplicates CPUC account 131.2",
		},
		{
			name: "unknown field",
			chart: `version: 1
accounts:
  - category: Asset
    number: "131.2"
    name: Cash-in-Bank
    explanation: invalid
`,
			message: "field explanation not found",
		},
		{
			name: "multiline comment",
			chart: `version: 1
accounts:
  - category: Asset
    number: "131.2"
    name: Cash-in-Bank
    comment: |-
      First line.
      Second line.
`,
			message: "comment must be single-line",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "accounts.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.chart), 0600))
			_, err := loadAccountChart(path)
			require.ErrorContains(t, err, tc.message)
		})
	}
}

func TestGroupedAccountChart(t *testing.T) {
	chart, err := parseAccountChart([]byte(`version: 2
accounts:
  Asset:
    - number: "131.2"
      name: Cash-in-Bank
  Expense:
    - number: "615"
      name: Power
      comment: Purchased electricity.
`))
	require.NoError(t, err)
	require.Equal(t, "Asset", chart.byNumber["131.2"].Category)
	require.Empty(t, chart.byNumber["131.2"].Comment)
	require.Equal(t, "Purchased electricity.", chart.byNumber["615"].Comment)
}

func TestGroupedAccountChartRejectsInvalidCategoryAndField(t *testing.T) {
	for _, tc := range []struct {
		name, chart, message string
	}{
		{
			name: "invalid category",
			chart: `version: 2
accounts:
  Revenue:
    - number: "421"
      name: Non-Utility-Income
`,
			message: `accounts category "Revenue" must be Asset`,
		},
		{
			name: "unknown account field",
			chart: `version: 2
accounts:
  Asset:
    - number: "131.2"
      name: Cash-in-Bank
      explanation: invalid
`,
			message: "field explanation not found",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseAccountChart([]byte(tc.chart))
			require.ErrorContains(t, err, tc.message)
		})
	}
}

func TestLegacyAccountChartCompatibility(t *testing.T) {
	path := filepath.Join(t.TempDir(), "accounts.journal")
	require.NoError(t, os.WriteFile(path, []byte(
		"account Asset:131.2-Cash-in-Bank\n",
	), 0600))
	chart, err := loadAccountChart(path)
	require.NoError(t, err)
	require.Equal(t, "Cash-in-Bank", chart.byNumber["131.2"].Name)
}

func TestMultipleChartDefinitionsMerge(t *testing.T) {
	books, _ := fixture(t, "")
	path := filepath.Join(books.Config.root, "reference", "supplemental.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
accounts:
  - category: Expense
    number: "999"
    name: Supplemental-Expense
    comment: Defined outside the primary CPUC chart.
`), 0600))
	cfg := books.Config
	cfg.ChartDefPaths = append(cfg.ChartDefPaths, "reference/supplemental.yaml")
	cfg.ChartDefs = append(cfg.ChartDefs, path)
	cfg.Accounts = append(cfg.Accounts, "Expense:999:Local-Supplement")
	merged, err := Open(cfg)
	require.NoError(t, err)
	accounts, err := merged.PostingAccounts()
	require.NoError(t, err)
	var supplemental PostingAccount
	for _, account := range accounts {
		if account.Name == "Expense:999:Local-Supplement" {
			supplemental = account
		}
	}
	require.Equal(t, "Supplemental-Expense", supplemental.FormalName)
	require.Equal(
		t,
		"Defined outside the primary CPUC chart.",
		supplemental.FormalComment,
	)
	draft, err := merged.NewDraft()
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, []byte(`version: 1
accounts:
  - category: Expense
    number: "999"
    name: Supplemental-Expense
    comment: Changed while a transaction was open.
`), 0600))
	require.ErrorContains(t, draft.unchanged(), "account chart definition changed")

	require.NoError(t, os.WriteFile(path, []byte(`version: 1
accounts:
  - category: Expense
    number: "615"
    name: Duplicate-Power
`), 0600))
	_, err = Open(cfg)
	require.ErrorContains(t, err, "duplicates CPUC account 615")
}

func TestPostingChoicesRequireDepthNotLeaves(t *testing.T) {
	books, _ := fixture(t, "")
	configureAccounts(t, books, "Expense:615:PGE", "Expense:615:PGE:Shop")
	accounts, err := books.Accounts()
	require.NoError(t, err)
	require.Contains(t, accounts, "Expense:615:PGE", "three-level accounts remain postable even when they have children")
	require.Contains(t, accounts, "Expense:615:PGE:Shop")
	require.Equal(t,
		[]string{"Expense:615:PGE", "Expense:615:PGE:Shop", "Expense:615:Utility"},
		matchingAccounts(accounts, "615"))
	help := editorHelp(accounts)
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

func TestGeneratedAccountsRejectUndeclaredLegacyEntriesAtOpen(t *testing.T) {
	books, _ := fixture(t, "")
	entry := testEntry()
	entry.Postings[0].Account = "Expense:615-Power"
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(entry.journalText("legacy")), 0600))
	_, err := Open(books.Config)
	require.ErrorContains(t, err, "has not been declared")
}

func TestNewEntriesRejectShallowAccountsInFormAndText(t *testing.T) {
	books, _ := fixture(t, "")
	configureAccounts(t, books, "Expense:615:PGE")
	addAccountDeclarations(t, books, "account Expense:615-Power\n")
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
