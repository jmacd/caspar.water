package wat

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const testChart = `version: 2
accounts:
  Asset:
    - number: "131.2"
      name: Cash-in-Bank
      comment: Cash held in a financial institution.
  Equity:
    - number: "218"
      name: Proprietary-Capital
  Expense:
    - number: "615"
      name: Power
  Income:
    - number: "421"
      name: Non-Utility-income
`

func fixture(t *testing.T, journal string) (*Books, string) {
	t.Helper()
	if _, err := exec.LookPath("hledger"); err != nil {
		t.Skip("hledger integration tests require hledger on PATH")
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "books"), 0700))
	require.NoError(t, os.Mkdir(filepath.Join(root, "reference"), 0700))
	for name, text := range map[string]string{
		"wat.yaml": `version: 3
name: Test Water
chart_defs:
  - reference/cpuc-u39w.yaml
accounts:
  - Equity:218:Owner
  - Expense:615:Utility
  - Income:421:Misc
bank_accounts:
  - Bank:131.2:Checking
`,
		"reference/cpuc-u39w.yaml": testChart,
		"books/manual.journal":     journal,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0600))
	}
	cfg, err := LoadConfig(filepath.Join(root, "wat.yaml"))
	require.NoError(t, err)
	books, err := Open(cfg)
	require.NoError(t, err)
	return books, root
}

func testEntry() Entry {
	return Entry{
		Date: "2021-04-05", Payee: "Electric company", Description: "Power bill",
		Reference: "check-7", Source: "sources/power.pdf",
		Postings: []Posting{{Account: "Expense:615:Utility", Amount: "12.34"}, {Account: "Bank:131.2:Checking", Amount: "-12.34"}},
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func bankJournal(t *testing.T, books *Books, account string) string {
	t.Helper()
	bank, ok := books.Config.bankAccount(account)
	require.True(t, ok, "bank account must be configured")
	return bank.Journal
}

func addBankAccount(t *testing.T, books *Books, account string) BankAccountConfig {
	t.Helper()
	bank := BankAccountConfig{
		Account: account,
		Journal: filepath.Join(books.Config.root, booksDirectory, bankJournalFilename(account)),
	}
	books.Config.BankAccounts = append(books.Config.BankAccounts, bank)
	require.NoError(t, prepareWorkspace(books.Config))
	return bank
}

func TestConfig(t *testing.T) {
	base := "version: 3\nname: Water\n"
	for _, tc := range []struct {
		name, yaml, accountChart string
		valid                    bool
	}{
		{name: "valid", yaml: base, valid: true},
		{
			name:         "configured chart",
			yaml:         base + "chart_defs:\n  - reference/cpuc-u39w.yaml\n",
			accountChart: "reference/cpuc-u39w.yaml",
			valid:        true,
		},
		{name: "absolute chart", yaml: base + "chart_defs:\n  - /tmp/chart.journal\n"},
		{name: "escaping chart", yaml: base + "chart_defs:\n  - ../chart.journal\n"},
		{name: "empty charts", yaml: base + "chart_defs: []\n"},
		{name: "duplicate chart", yaml: base + "chart_defs:\n  - reference/chart.yaml\n  - reference/./chart.yaml\n"},
		{name: "chart conflicts with manual journal", yaml: base + "chart_defs:\n  - books/manual.journal\n"},
		{name: "ordinary account", yaml: base + "accounts:\n  - Expense:615:Electricity\n", valid: true},
		{name: "unknown", yaml: base + "typo: true\n"},
		{name: "obsolete path", yaml: base + "journal: books/books.journal\n"},
		{name: "bank account", yaml: base + "bank_accounts:\n  - Bank:131.2:SBMC:1234\n", valid: true},
		{name: "duplicate account", yaml: base + "accounts:\n  - Expense:615:Electricity\n  - Expense:615:Electricity\n"},
		{name: "duplicate bank identifier", yaml: base + "bank_accounts:\n  - Bank:131.2:First:1234\n  - Bank:131.2:Second:1234\n"},
		{name: "duplicate bank filename", yaml: base + "bank_accounts:\n  - Bank:131.2:First.Account\n  - Bank:131.2:First-Account\n"},
		{name: "azure statement", yaml: base + "statement:\n  python: python3\n  api_base: https://example.openai.azure.com/\n  api_version: 2025-04-01-preview\n  model: azure/example\n", valid: true},
		{name: "duplicate statement endpoint", yaml: base + "statement:\n  api_base: https://example.openai.azure.com/\n  ollama_url: http://localhost:11434\n  model: azure/example\n"},
		{name: "blank bank account", yaml: base + "bank_accounts:\n  - ' '\n"},
		{name: "non-Bank statement account", yaml: base + "bank_accounts:\n  - Asset:131.2:Checking\n"},
		{name: "multiline bank account", yaml: base + "bank_accounts:\n  - \"Bank:131.2:Checking\\nEquity:218:Owner\"\n"},
		{name: "virtual bank account", yaml: base + "bank_accounts:\n  - '[Bank:131.2:Checking]'\n"},
		{name: "bank mapping object", yaml: base + "bank_accounts:\n  - account: Bank:131.2:Checking\n"},
		{name: "bank in ordinary accounts", yaml: base + "accounts:\n  - Bank:131.2:Checking\n"},
		{name: "path separator", yaml: base + "accounts:\n  - Expense:615:Power/Bill\n"},
		{name: "documents", yaml: base + "---\nname: other\n"},
		{name: "version", yaml: strings.Replace(base, "version: 3", "version: 2", 1)},
		{name: "missing name", yaml: strings.Replace(base, "name: Water\n", "", 1)},
		{name: "duplicate-key", yaml: base + "name: Other\n"},
		{name: "malformed-extra", yaml: base + "---\ninvalid: [\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "wat.yaml")
			require.NoError(t, os.WriteFile(path, []byte(tc.yaml), 0600))
			cfg, err := LoadConfig(path)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			directory, err := filepath.EvalSymlinks(filepath.Dir(path))
			require.NoError(t, err)
			require.Equal(t, filepath.Join(directory, "books", "books.journal"), cfg.Journal)
			accountChart := tc.accountChart
			if accountChart == "" {
				accountChart = "books/chart.journal"
			}
			require.Equal(
				t,
				[]string{filepath.Join(directory, accountChart)},
				cfg.ChartDefs,
			)
			require.Equal(t, filepath.Join(directory, "books", "accounts.journal"), cfg.LocalAccounts)
			require.Equal(t, filepath.Join(directory, "books", "manual.journal"), cfg.ManualJournal)
			require.Equal(t, filepath.Join(directory, "sources"), cfg.Sources)
			require.Equal(
				t,
				filepath.Join(directory, "quickbooks-mappings.yaml"),
				cfg.QuickBooksMappings,
			)
			if len(cfg.BankAccounts) != 0 {
				require.Equal(t, filepath.Join(directory, "books", "bank-sbmc-1234.journal"), cfg.BankAccounts[0].Journal)
			}
		})
	}
}

func TestWorkspaceFilesAreGeneratedFromConfiguration(t *testing.T) {
	books, _ := fixture(t, "")
	require.Equal(t, string(generatedAccounts(books.Config)), read(t, books.Config.LocalAccounts))
	require.Equal(t, string(generatedMaster(books.Config)), read(t, books.Config.Journal))
	files, err := books.run(books.Config.Journal, "files")
	require.NoError(t, err)
	for _, chartDef := range books.Config.ChartDefs {
		require.NotContains(t, files, chartDef)
	}
	require.Contains(t, files, books.Config.LocalAccounts)
	require.Contains(t, files, books.Config.ManualJournal)
	require.Contains(t, files, books.Config.BankAccounts[0].Journal)
}

func TestWorkspaceRejectsSymlinkedGeneratedFiles(t *testing.T) {
	books, root := fixture(t, "")
	require.NoError(t, os.Remove(books.Config.LocalAccounts))
	require.NoError(t, os.Symlink(filepath.Join(root, "wat.yaml"), books.Config.LocalAccounts))
	_, err := Open(books.Config)
	require.ErrorContains(t, err, "generated journal must be a nonsymlink regular file")
}

func TestBankAccountConfiguration(t *testing.T) {
	books, _ := fixture(t, "")
	for _, account := range []string{"Bank:131.2:Checking", "Bank:131.2:Missing", "Bank:999:Unknown"} {
		t.Run(account, func(t *testing.T) {
			cfg := books.Config
			cfg.BankAccounts[0].Account = account
			cfg.BankAccounts[0].Journal = filepath.Join(cfg.root, booksDirectory, bankJournalFilename(account))
			_, err := Open(cfg)
			if account != "Bank:999:Unknown" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "not in the CPUC chart")
			}
		})
	}
}

func TestParseCents(t *testing.T) {
	for input, expected := range map[string]int64{
		"0": 0, "0.01": 1, "12.3": 1230, "-12.34": -1234,
		"+12": 1200, " 12.34 ": 1234, "92233720368547758.07": math.MaxInt64,
		"-92233720368547758.07": -math.MaxInt64,
	} {
		t.Run(input, func(t *testing.T) {
			got, err := ParseCents(input)
			require.NoError(t, err)
			require.Equal(t, expected, got)
		})
	}
	for _, input := range []string{"", ".", ".50", "1.", "1.001", "1,000", "$1.00", "1e2", "--1", "1\n0", "92233720368547758.08"} {
		t.Run("invalid-"+input, func(t *testing.T) {
			_, err := ParseCents(input)
			require.Error(t, err)
		})
	}
}

func TestEntryBalanceStatus(t *testing.T) {
	for _, tc := range []struct {
		name    string
		amounts []string
		want    string
	}{
		{"empty", []string{"", " "}, "Balance: $0.00 | Enter posting amounts"},
		{"credit first", []string{"-100", ""}, "Balance: $-100.00 | To balance: +100.00 USD (debit)"},
		{"partial split", []string{"-100", "40", ""}, "Balance: $-60.00 | To balance: +60.00 USD (debit)"},
		{"split balanced", []string{"-100", "40", "60"}, "Balance: $0.00 | Balanced"},
		{"debit first", []string{"100"}, "Balance: $100.00 | To balance: -100.00 USD (credit)"},
		{"exact cents", []string{"0.10", "0.20", "-0.30"}, "Balance: $0.00 | Balanced"},
		{"invalid", []string{"100", "-"}, "Balance unavailable: posting 2:"},
		{"precision", []string{"1.001"}, "Balance unavailable: posting 1:"},
		{"overflow", []string{"92233720368547758.07", "0.01"}, "Balance unavailable: posting total exceeds"},
		{"negative overflow", []string{"-92233720368547758.07", "-0.02"}, "Balance unavailable: posting total exceeds"},
		{"minimum", []string{"-92233720368547758.07", "-0.01"}, "Balance unavailable: balancing amount exceeds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var entry Entry
			for _, amount := range tc.amounts {
				entry.Postings = append(entry.Postings, Posting{Amount: amount})
			}
			require.Contains(t, entry.balanceStatus(), tc.want)
		})
	}
}

func TestEntryValidation(t *testing.T) {
	accounts := []string{"Expense:615:Utility", "Bank:131.2:Checking"}
	for _, tc := range []struct {
		name   string
		change func(*Entry)
	}{
		{"invalid-date", func(e *Entry) { e.Date = "2021-02-29" }},
		{"payee", func(e *Entry) { e.Payee = " " }},
		{"injection", func(e *Entry) { e.Description = "text\n2021-01-01 attack" }},
		{"delimiter", func(e *Entry) { e.Payee = "name ; comment" }},
		{"tag-truncation", func(e *Entry) { e.Source = "one,two" }},
		{"account", func(e *Entry) { e.Postings[0].Account = "Expenses:Typo" }},
		{"zero", func(e *Entry) { e.Postings[0].Amount = "0" }},
		{"precision", func(e *Entry) { e.Postings[0].Amount = "12.345" }},
		{"balance", func(e *Entry) { e.Postings[0].Amount = "12.35" }},
		{"one-leg", func(e *Entry) { e.Postings = e.Postings[:1] }},
		{"overflow", func(e *Entry) {
			e.Postings = []Posting{
				{"Bank:131.2:Checking", "92233720368547758.07"},
				{"Bank:131.2:Checking", "0.01"},
				{"Bank:131.2:Checking", "-92233720368547758.07"},
				{"Bank:131.2:Checking", "-0.01"},
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entry := testEntry()
			tc.change(&entry)
			_, err := entry.render(accounts, "test-id")
			require.Error(t, err)
		})
	}
	entry := testEntry()
	entry.Date = "2022-01-01"
	entry.Postings = []Posting{{"Expense:615:Utility", "6"}, {"Expense:615:Utility", "6.34"}, {"Bank:131.2:Checking", "-12.34"}}
	text, err := entry.render(accounts, "test-id")
	require.NoError(t, err)
	require.Contains(t, text, "2022-01-01 Electric company | Power bill")
	require.Contains(t, text, "; id: test-id")
	require.Contains(t, text, "; source: sources/power.pdf")
	require.Contains(t, text, "Bank:131.2:Checking  $-12.34")
}

func TestSavePreservesBytesAndBackup(t *testing.T) {
	for _, original := range []string{"", "; retained comment", "; retained\r\n\r\n"} {
		t.Run(fmt.Sprintf("%q", original), func(t *testing.T) {
			books, _ := fixture(t, original)
			require.NoError(t, os.Chmod(books.Config.ManualJournal, 0640))
			draft, err := books.NewDraft()
			require.NoError(t, err)
			text, err := draft.Preview(testEntry())
			require.NoError(t, err)
			require.Equal(t, original, read(t, books.Config.ManualJournal))
			backup, err := draft.Save()
			require.NoError(t, err)
			require.Equal(t, original, read(t, backup))
			saved := read(t, books.Config.ManualJournal)
			require.True(t, strings.HasPrefix(saved, original))
			require.True(t, strings.HasSuffix(saved, text))
			require.NoError(t, books.Check())
			info, err := os.Stat(books.Config.ManualJournal)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0640), info.Mode().Perm())
			_, err = draft.Save()
			require.ErrorContains(t, err, "preview")
			require.Equal(t, saved, read(t, books.Config.ManualJournal))
			_, err = os.Stat(books.Config.ManualJournal + ".wat-lock")
			require.True(t, os.IsNotExist(err))
			require.NotContains(t, saved, "* Electric")
		})
	}
}

func TestStaleDraftAndLock(t *testing.T) {
	for _, changed := range []string{"books/manual.journal", "books/accounts.journal", "reference/cpuc-u39w.yaml", "include-set", "lock"} {
		t.Run(changed, func(t *testing.T) {
			books, root := fixture(t, "; original\n")
			draft, err := books.NewDraft()
			require.NoError(t, err)
			_, err = draft.Preview(testEntry())
			require.NoError(t, err)
			switch changed {
			case "include-set":
				require.NoError(t, os.WriteFile(filepath.Join(root, "books/extra.journal"), []byte("; new\n"), 0600))
				require.NoError(t, os.WriteFile(books.Config.Journal, []byte("include accounts.journal\ninclude manual.journal\ninclude bank-checking.journal\ninclude extra.journal\n"), 0600))
			case "lock":
				require.NoError(t, os.WriteFile(books.Config.ManualJournal+".wat-lock", []byte("other writer"), 0600))
			default:
				path := filepath.Join(root, changed)
				require.NoError(t, os.WriteFile(path, []byte(read(t, path)+"; external change\n"), 0600))
			}
			before := read(t, books.Config.ManualJournal)
			_, err = draft.Save()
			require.Error(t, err)
			require.Equal(t, before, read(t, books.Config.ManualJournal))
		})
	}
}

func TestAssertionFailureDoesNotChangeBooks(t *testing.T) {
	original := "2021-12-31 Bank checkpoint\n    Bank:131.2:Checking  $0 = $0\n    Equity:218:Owner  $0\n"
	books, _ := fixture(t, original)
	draft, err := books.NewDraft()
	require.NoError(t, err)
	_, err = draft.Preview(testEntry())
	require.NoError(t, err)
	_, err = draft.Save()
	require.ErrorContains(t, err, "validation")
	require.Equal(t, original, read(t, books.Config.ManualJournal))
}

func TestUnsupportedIncludes(t *testing.T) {
	t.Run("absolute", func(t *testing.T) {
		books, root := fixture(t, "")
		includes := "include accounts.journal\ninclude bank-checking.journal\ninclude " + books.Config.ManualJournal + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(root, "books/books.journal"), []byte(includes), 0600))
		draft, err := books.NewDraft()
		require.NoError(t, err)
		_, err = draft.Preview(testEntry())
		require.NoError(t, err)
		_, err = draft.Save()
		require.ErrorContains(t, err, "escapes")
		require.Empty(t, read(t, books.Config.ManualJournal))
	})
	t.Run("symlink", func(t *testing.T) {
		books, root := fixture(t, "")
		alias := filepath.Join(root, "books/alias.journal")
		require.NoError(t, os.Symlink(books.Config.ManualJournal, alias))
		require.NoError(t, os.WriteFile(books.Config.Journal, []byte("include accounts.journal\ninclude alias.journal\n"), 0600))
		_, err := books.NewDraft()
		require.ErrorContains(t, err, "nonsymlink")
	})
	t.Run("write-not-included", func(t *testing.T) {
		books, _ := fixture(t, "")
		require.NoError(t, os.WriteFile(books.Config.Journal, []byte("include accounts.journal\n"), 0600))
		_, err := books.NewDraft()
		require.ErrorContains(t, err, "must exist and be included")
	})
}

func TestReportsIncludeAllAccountingDates(t *testing.T) {
	books, _ := fixture(t, "")
	history := `2020-12-31 Opening
    Bank:131.2:Checking  $100
    Equity:218:Owner  $-100

2021-04-05 Power
    Expense:615:Utility  $10
    Bank:131.2:Checking  $-10

2022-01-01 Future
    Bank:131.2:Checking  $900
    Income:421:Misc  $-900
`
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(history), 0600))
	for _, name := range []string{"trial-balance", "balance-sheet"} {
		report, err := books.Report(name)
		require.NoError(t, err)
		require.Contains(t, report, "$990")
		if name == "balance-sheet" {
			require.Contains(t, report, "Equity:218:Owner")
			require.Contains(t, report, "$100")
		}
	}
	report, err := books.Report("income-statement")
	require.NoError(t, err)
	require.Contains(t, report, "$10")
	require.Contains(t, report, "$900")
	report, err = books.Report("journal")
	require.NoError(t, err)
	require.Contains(t, report, "Opening")
	require.Contains(t, report, "Power")
	require.Contains(t, report, "Future")
	_, err = books.Report("--output-file=oops")
	require.ErrorContains(t, err, "unknown report")
}

func TestHledgerConfigurationIsIgnored(t *testing.T) {
	books, root := fixture(t, "")
	require.NoError(t, os.WriteFile(filepath.Join(root, "hledger.conf"), []byte("[general]\n--file=missing.journal\n"), 0600))
	require.NoError(t, books.Check())
}
