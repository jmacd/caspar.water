package wat

import (
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/stretchr/testify/require"
)

func enableTestPayees(t *testing.T, books *Books) *Books {
	t.Helper()
	original := read(t, books.Config.Payees)
	backup, err := InitPayeeRegistry(books.Config)
	require.NoError(t, err)
	require.Equal(t, original, read(t, backup))
	require.Equal(t, original, read(t, books.Config.Payees))
	_, err = InitPayeeRegistry(books.Config)
	require.ErrorIs(t, err, os.ErrExist)
	config := read(t, books.Config.configPath) + "\npayee_registry: true\n"
	require.NoError(t, os.WriteFile(books.Config.configPath, []byte(config), 0600))
	cfg, err := LoadConfig(books.Config.configPath)
	require.NoError(t, err)
	books, err = Open(cfg)
	require.NoError(t, err)
	return books
}

func TestPayeeRegistryRejectsAmbiguousAliases(t *testing.T) {
	const valid = "version: 1\npayees:\n  - name: Power company\n    aliases: [POWER ONLINE]\n"
	for _, tc := range []struct{ name, text, message string }{
		{"unknown field", valid + "other: true\n", "field other"},
		{"duplicate key", valid + "version: 1\n", "already defined"},
		{"second document", valid + "---\nversion: 1\n", "exactly one"},
		{"missing list", "version: 1\n", "payees array"},
		{"duplicate name", valid + "  - name: Power company\n", "ambiguous"},
		{"alias owns canonical", valid + "  - name: Other\n    aliases: [power COMPANY]\n", "ambiguous"},
		{"duplicate alias", strings.Replace(valid, "[POWER ONLINE]", "[POWER ONLINE, POWER ONLINE]", 1), "duplicate"},
		{"conflicting alias", valid + "  - name: Other\n    aliases: [' power online ']\n", "invalid"},
		{"case conflict", valid + "  - name: Other\n    aliases: [power online]\n", "ambiguous"},
		{"check number", strings.Replace(valid, "POWER ONLINE", "CHECK 1234", 1), "check numbers"},
		{"compact check number", strings.Replace(valid, "POWER ONLINE", "CHECK1234", 1), "check numbers"},
		{"delimiter", strings.Replace(valid, "POWER ONLINE", "Power | memo", 1), "invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodePayeeRegistry([]byte(tc.text), nil)
			require.ErrorContains(t, err, tc.message)
		})
	}
	_, err := decodePayeeRegistry([]byte(valid), []string{"POWER ONLINE"})
	require.ErrorContains(t, err, "conflicts with customer")
	registry, err := decodePayeeRegistry([]byte(valid), []string{"Power company"})
	require.NoError(t, err, "a customer and vendor may share the same canonical name")
	books := &Books{payeeRegistry: registry}
	books.Config.Statement.TransactionPrompts = []StatementTransactionPrompt{
		{Payee: "Power company", Prompt: "First rule"},
		{Payee: "POWER ONLINE", Prompt: "Conflicting rule"},
	}
	_, err = books.canonicalTransactionPrompts()
	require.ErrorContains(t, err, "multiple transaction prompt rules")
}

const aliasJournal = `2021-04-05 POWER ONLINE | Original bill
    ; id: original
    Expense:615.000:Utility  $12.34
    Bank:131.200:Checking  $-12.34
`

func payeeAliasFixture(t *testing.T) *Books {
	t.Helper()
	books, _ := fixture(t, aliasJournal)
	books = enableTestPayees(t, books)
	_, saved, err := books.AddPayeeAlias("Electric company", "POWER ONLINE")
	require.NoError(t, err)
	require.True(t, saved)
	return books
}

func TestPayeeRegistryHistoryAndDraftsPreserveOriginalEvidence(t *testing.T) {
	books := payeeAliasFixture(t)
	require.NoError(t, books.Check(), "both historical aliases and canonical names remain declared")
	entries, err := books.JournalEntries()
	require.NoError(t, err)
	require.Equal(t, "Electric company | Original bill", entries[0].Description)
	require.Equal(t, "POWER ONLINE | Original bill", entries[0].OriginalDescription)
	require.Len(t, FilterJournalEntries(entries, "Electric company"), 1)
	require.Len(t, FilterJournalEntries(entries, "power online"), 1)
	require.Len(t, FilterJournalEntries(entries, " power  online "), 1)
	require.Equal(t, aliasJournal, read(t, books.Config.ManualJournal))
	raw, err := books.Report("journal")
	require.NoError(t, err)
	require.Contains(t, raw, "POWER ONLINE", "raw hledger reports remain an audit view")

	revision, value, err := books.EditDraft("original")
	require.NoError(t, err)
	require.Equal(t, "POWER ONLINE", value.Payee, "viewing or editing must not silently rename saved transactions")
	text, err := revision.Preview(value)
	require.NoError(t, err)
	require.Contains(t, text, "POWER ONLINE | Original bill")
	require.Equal(t, aliasJournal, read(t, books.Config.ManualJournal))

	stale, err := books.NewDraft()
	require.NoError(t, err)
	_, err = stale.Preview(testEntry())
	require.NoError(t, err)
	_, _, err = books.AddPayeeAlias("Electric company", "UTILITY WEB")
	require.NoError(t, err)
	_, err = stale.Save()
	require.ErrorContains(t, err, "input changed", "alias changes invalidate earlier drafts")
	entries, err = books.JournalEntries()
	require.NoError(t, err)
	require.Len(t, FilterJournalEntries(entries, "UTILITY WEB"), 1, "other aliases find the same canonical history")

	draft, err := books.NewDraft()
	require.NoError(t, err)
	require.Equal(t, []string{"Electric company"}, draft.history.matchingPayees("power online"))
	require.Equal(t, []string{"Electric company"}, draft.history.matchingPayees(" power  online "))
	require.NotContains(t, draft.history.payees, "POWER ONLINE")
	entry := testEntry()
	entry.Payee = "power online"
	text, err = draft.Preview(entry)
	require.NoError(t, err)
	require.Contains(t, text, "Electric company | Power bill")
	_, err = draft.Save()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(read(t, books.Config.ManualJournal), aliasJournal))

	registry := read(t, books.Config.PayeeRegistryFile)
	_, _, err = books.AddPayeeAlias("Journal", "POWER ONLINE")
	require.ErrorContains(t, err, "ambiguous")
	require.Equal(t, registry, read(t, books.Config.PayeeRegistryFile))
	require.NoError(t, os.Remove(books.Config.Payees))
	reopened, err := Open(books.Config)
	require.NoError(t, err)
	require.NoError(t, reopened.Check(), "native declarations can be regenerated from YAML")
	require.NoError(t, os.Remove(books.Config.PayeeRegistryFile))
	_, err = Open(books.Config)
	require.ErrorContains(t, err, "payee registry", "an enabled missing registry is never an empty replacement")
}

func TestPayeeAliasesApplyToStatementDraftsNotBankEvidence(t *testing.T) {
	books, statement, _ := statementFixture(t)
	books = enableTestPayees(t, books)
	_, _, err := books.AddPayeeAlias("Electric company", "POWER CO")
	require.NoError(t, err)
	statement.checkPayees = map[int]statementCheckPayee{1: {Row: 2, Payee: "Wrong guess", Memo: "Wrong memo"}}
	draft, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, "Electric company", entry.Payee, "explicit aliases precede AI guesses")
	require.Empty(t, entry.Description)
	require.Equal(t, "POWER CO", statement.Rows[1].Description)
	require.Equal(t, descriptionKey("POWER CO"), entry.BankDescription)
	require.Equal(t, statement.key(1), entry.BankRow)
	entry.Postings[1].Account = "Expense:615.000:Utility"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.True(t, statuses[1].Recorded, "canonical naming must preserve the original bank-row match")
}

func TestPayeeRegistryDoesNotDuplicateOrRedirectCustomers(t *testing.T) {
	books := enableTestPayees(t, customerFixture(t))
	registry, err := books.Payees()
	require.NoError(t, err)
	require.NotContains(t, registry.declarations(), "Example Customer")
	_, _, err = books.AddPayeeAlias("Electric company", "Example Customer")
	require.ErrorContains(t, err, "conflicts with customer")
	_, _, err = books.AddPayeeAlias("Electric company", "POWER ONLINE")
	require.NoError(t, err)
	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	_, err = draft.AddCustomer("POWER ONLINE", []string{"Example address"})
	require.NoError(t, err)
	_, err = draft.Save()
	require.ErrorContains(t, err, "conflicts with customer")
	require.NoError(t, books.Check())
}

func TestPayeeAliasesMatchHistoryWithoutGuessingAccounts(t *testing.T) {
	books, statement, _ := statementFixture(t)
	history := strings.Replace(aliasJournal, "2021-04-05", "2021-03-05", 1)
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(history), 0600))
	books = enableTestPayees(t, books)
	for _, alias := range []string{"POWER ONLINE", "POWER CO"} {
		_, _, err := books.AddPayeeAlias("Electric company", alias)
		require.NoError(t, err)
	}
	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.Equal(t, "Expense:615.000:Utility", statuses[1].Suggestion)
	history += `
2021-03-06 Electric company | Different classification
    Equity:218.000:Owner  $5
    Bank:131.200:Checking  $-5
`
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(history), 0600))
	statuses, err = books.StatementStatuses(statement)
	require.NoError(t, err)
	require.Empty(t, statuses[1].Suggestion, "alias matching must retain the conflicting-account safeguard")
}

func TestPayeeAliasHistorySearchInTerminal(t *testing.T) {
	books := payeeAliasFixture(t)
	frame, key := customerTestTerminal(t, books, func(ui *terminal) { ui.journalEntries() })
	require.Contains(t, frame.text, "Electric company")
	require.NotContains(t, frame.text, "POWER ONLINE")
	key(tcell.KeyRune, '/')
	for _, r := range "POWER ONLINE" {
		frame = key(tcell.KeyRune, r)
	}
	require.Contains(t, frame.text, "Electric company")
	key(tcell.KeyEnter, 0)
	key(tcell.KeyRune, '/')
	key(tcell.KeyCtrlU, 0)
	for _, r := range "no matching vendor" {
		frame = key(tcell.KeyRune, r)
	}
	require.NotContains(t, frame.text, "Electric company")
	frame = key(tcell.KeyEscape, 0)
	require.Contains(t, frame.text, "Electric company")
	require.Equal(t, aliasJournal, read(t, books.Config.ManualJournal))
}
