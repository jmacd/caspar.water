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

const testChart = `commodity $
account Asset:131.2-Cash-in-Bank
account Equity:218-Proprietary-Capital
account Expense:615-Power
account Income:421-Non-Utility-income
`

const testAccounts = `account Bank  ; type: A
account Bank:131.2:Checking
account Equity:218:Owner
account Expense:615:Utility
account Income:421:Misc
`

func fixture(t *testing.T, journal string) (*Books, string) {
	t.Helper()
	if _, err := exec.LookPath("hledger"); err != nil {
		t.Skip("hledger integration tests require hledger on PATH")
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "books"), 0700))
	for name, text := range map[string]string{
		"wat.yaml": `version: 2
name: Test Water
report_year: 2021
journal: books/books.journal
account_chart: books/chart.journal
manual_journal: books/2021.journal
sources: sources
bank_accounts:
  - account: Bank:131.2:Checking
    journal: books/bank.journal
`,
		"books/books.journal":    "include chart.journal\ninclude accounts.journal\ninclude 2021.journal\ninclude bank.journal\n",
		"books/chart.journal":    testChart,
		"books/accounts.journal": testAccounts,
		"books/2021.journal":     journal,
		"books/bank.journal":     "",
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

func addBankAccount(t *testing.T, books *Books, account, filename string) BankAccountConfig {
	t.Helper()
	path := filepath.Join(books.Config.root, "books", filename)
	require.NoError(t, os.WriteFile(path, nil, 0600))
	master := read(t, books.Config.Journal)
	require.NoError(t, os.WriteFile(books.Config.Journal, []byte(master+"include "+filename+"\n"), 0600))
	accounts := filepath.Join(books.Config.root, "books", "accounts.journal")
	require.NoError(t, os.WriteFile(accounts, []byte(read(t, accounts)+"account "+account+"\n"), 0600))
	bank := BankAccountConfig{Account: account, Journal: path}
	books.Config.BankAccounts = append(books.Config.BankAccounts, bank)
	return bank
}

func TestConfig(t *testing.T) {
	base := "version: 2\nname: Water\nreport_year: 2021\njournal: books.journal\naccount_chart: accounts.journal\nmanual_journal: manual.journal\nsources: sources\n"
	for _, tc := range []struct {
		name, yaml string
		valid      bool
	}{
		{"valid", base, true},
		{"unknown", base + "typo: true\n", false},
		{"bank account", base + "bank_accounts:\n  - account: Bank:131.2:SBMC:1234\n    journal: bank.journal\n", true},
		{"duplicate bank identifier", base + "bank_accounts:\n  - account: Bank:131.2:First:1234\n    journal: first.journal\n  - account: Bank:131.2:Second:1234\n    journal: second.journal\n", false},
		{"duplicate writable journal", base + "bank_accounts:\n  - account: Bank:131.2:SBMC:1234\n    journal: manual.journal\n", false},
		{"azure statement", base + "statement:\n  python: python3\n  api_base: https://example.openai.azure.com/\n  api_version: 2025-04-01-preview\n  model: azure/example\n", true},
		{"duplicate statement endpoint", base + "statement:\n  api_base: https://example.openai.azure.com/\n  ollama_url: http://localhost:11434\n  model: azure/example\n", false},
		{"blank bank account", base + "bank_accounts:\n  - account: ' '\n    journal: bank.journal\n", false},
		{"non-Bank statement account", base + "bank_accounts:\n  - account: Asset:131.2:Checking\n    journal: bank.journal\n", false},
		{"multiline bank account", base + "bank_accounts:\n  - account: \"Bank:131.2:Checking\\nEquity:218:Owner\"\n    journal: bank.journal\n", false},
		{"virtual bank account", base + "bank_accounts:\n  - account: '[Bank:131.2:Checking]'\n    journal: bank.journal\n", false},
		{"documents", base + "---\nname: other\n", false},
		{"version", strings.Replace(base, "version: 2", "version: 1", 1), false},
		{"year", strings.Replace(base, "2021", "0", 1), false},
		{"missing", strings.Replace(base, "sources: sources\n", "", 1), false},
		{"escape", strings.Replace(base, "journal: books.journal", "journal: ../books.journal", 1), false},
		{"duplicate-key", base + "year: 2022\n", false},
		{"malformed-extra", base + "---\ninvalid: [\n", false},
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
			require.Equal(t, filepath.Join(directory, "books.journal"), cfg.Journal)
			require.Equal(t, filepath.Join(directory, "accounts.journal"), cfg.AccountChart)
			require.Equal(t, filepath.Join(directory, "manual.journal"), cfg.ManualJournal)
		})
	}
}

func TestBankAccountConfiguration(t *testing.T) {
	books, _ := fixture(t, "")
	for _, account := range []string{"Bank:131.2:Checking", "Bank:131.2:Missing", "Bank:999:Unknown"} {
		t.Run(account, func(t *testing.T) {
			cfg := books.Config
			cfg.BankAccounts[0].Account = account
			_, err := Open(cfg)
			if account == "Bank:131.2:Checking" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "configured bank account must be a declared")
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
			_, err := entry.render(2021, accounts, "test-id")
			require.Error(t, err)
		})
	}
	entry := testEntry()
	entry.Date = "2022-01-01"
	entry.Postings = []Posting{{"Expense:615:Utility", "6"}, {"Expense:615:Utility", "6.34"}, {"Bank:131.2:Checking", "-12.34"}}
	text, err := entry.render(2021, accounts, "test-id")
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
	for _, changed := range []string{"books/2021.journal", "books/accounts.journal", "books/chart.journal", "include-set", "lock"} {
		t.Run(changed, func(t *testing.T) {
			books, root := fixture(t, "; original\n")
			draft, err := books.NewDraft()
			require.NoError(t, err)
			_, err = draft.Preview(testEntry())
			require.NoError(t, err)
			switch changed {
			case "include-set":
				require.NoError(t, os.WriteFile(filepath.Join(root, "books/extra.journal"), []byte("; new\n"), 0600))
				require.NoError(t, os.WriteFile(books.Config.Journal, []byte("include accounts.journal\ninclude 2021.journal\ninclude bank.journal\ninclude extra.journal\n"), 0600))
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
		includes := "include chart.journal\ninclude accounts.journal\ninclude bank.journal\ninclude " + books.Config.ManualJournal + "\n"
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

func TestReportsIncludeOpeningBalancesButExcludeFuture(t *testing.T) {
	books, root := fixture(t, "")
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
	require.NoError(t, os.WriteFile(filepath.Join(root, "books/2021.journal"), []byte(history), 0600))
	for _, name := range []string{"trial-balance", "balance-sheet"} {
		report, err := books.Report(name)
		require.NoError(t, err)
		require.Contains(t, report, "$90")
		require.NotContains(t, report, "$990")
		if name == "balance-sheet" {
			require.Contains(t, report, "Equity:218:Owner")
			require.Contains(t, report, "$100")
			require.Contains(t, report, "$-10")
		}
	}
	report, err := books.Report("income-statement")
	require.NoError(t, err)
	require.Contains(t, report, "$10")
	require.NotContains(t, report, "$900")
	report, err = books.Report("journal")
	require.NoError(t, err)
	require.Contains(t, report, "Power")
	require.NotContains(t, report, "Opening")
	require.NotContains(t, report, "Future")
	_, err = books.Report("--output-file=oops")
	require.ErrorContains(t, err, "unknown report")
}

func TestHledgerConfigurationIsIgnored(t *testing.T) {
	books, root := fixture(t, "")
	require.NoError(t, os.WriteFile(filepath.Join(root, "hledger.conf"), []byte("[general]\n--file=missing.journal\n"), 0600))
	require.NoError(t, books.Check())
}
