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

const testAccounts = `commodity $
account Assets:Bank
account Equity:Capital
account Expenses:Power
account Income:Other
`

func fixture(t *testing.T, journal string) (*Books, string) {
	t.Helper()
	if _, err := exec.LookPath("hledger"); err != nil {
		t.Skip("hledger integration tests require hledger on PATH")
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "books"), 0700))
	for name, text := range map[string]string{
		"wat.yaml": `version: 1
name: Test Water
year: 2021
journal: books/books.journal
write_journal: books/2021.journal
sources: sources
`,
		"books/books.journal":    "include accounts.journal\ninclude 2021.journal\n",
		"books/accounts.journal": testAccounts,
		"books/2021.journal":     journal,
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
		Postings: []Posting{{Account: "Expenses:Power", Amount: "12.34"}, {Account: "Assets:Bank", Amount: "-12.34"}},
	}
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestConfig(t *testing.T) {
	base := "version: 1\nname: Water\nyear: 2021\njournal: books.journal\nwrite_journal: 2021.journal\nsources: sources\n"
	for _, tc := range []struct {
		name, yaml string
		valid      bool
	}{
		{"valid", base, true},
		{"unknown", base + "typo: true\n", false},
		{"bank account", base + "bank_account: Assets:Bank\n", true},
		{"blank bank account", base + "bank_account: ' '\n", false},
		{"multiline bank account", base + "bank_account: \"Assets:Bank\\nEquity:Capital\"\n", false},
		{"virtual bank account", base + "bank_account: '[Assets:Bank]'\n", false},
		{"documents", base + "---\nname: other\n", false},
		{"version", strings.Replace(base, "version: 1", "version: 2", 1), false},
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
			require.Equal(t, filepath.Join(directory, "2021.journal"), cfg.WriteJournal)
		})
	}
}

func TestBankAccountConfiguration(t *testing.T) {
	books, _ := fixture(t, "")
	for _, account := range []string{"", "Assets:Bank", "Asset:Bank", "Assets:Unknown"} {
		t.Run(account, func(t *testing.T) {
			cfg := books.Config
			cfg.BankAccount = account
			_, err := Open(cfg)
			if account == "" || account == "Assets:Bank" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, "bank_account is not a declared account")
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
	accounts := []string{"Expenses:Power", "Assets:Bank"}
	for _, tc := range []struct {
		name   string
		change func(*Entry)
	}{
		{"date", func(e *Entry) { e.Date = "2022-01-01" }},
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
				{"Assets:Bank", "92233720368547758.07"},
				{"Assets:Bank", "0.01"},
				{"Assets:Bank", "-92233720368547758.07"},
				{"Assets:Bank", "-0.01"},
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
	entry.Postings = []Posting{{"Expenses:Power", "6"}, {"Expenses:Power", "6.34"}, {"Assets:Bank", "-12.34"}}
	text, err := entry.render(2021, accounts, "test-id")
	require.NoError(t, err)
	require.Contains(t, text, "2021-04-05 Electric company | Power bill")
	require.Contains(t, text, "; id: test-id")
	require.Contains(t, text, "; source: sources/power.pdf")
	require.Contains(t, text, "Assets:Bank  $-12.34")
}

func TestSavePreservesBytesAndBackup(t *testing.T) {
	for _, original := range []string{"", "; retained comment", "; retained\r\n\r\n"} {
		t.Run(fmt.Sprintf("%q", original), func(t *testing.T) {
			books, _ := fixture(t, original)
			require.NoError(t, os.Chmod(books.Config.WriteJournal, 0640))
			draft, err := books.NewDraft()
			require.NoError(t, err)
			text, err := draft.Preview(testEntry())
			require.NoError(t, err)
			require.Equal(t, original, read(t, books.Config.WriteJournal))
			backup, err := draft.Save()
			require.NoError(t, err)
			require.Equal(t, original, read(t, backup))
			saved := read(t, books.Config.WriteJournal)
			require.True(t, strings.HasPrefix(saved, original))
			require.True(t, strings.HasSuffix(saved, text))
			require.NoError(t, books.Check())
			info, err := os.Stat(books.Config.WriteJournal)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0640), info.Mode().Perm())
			_, err = draft.Save()
			require.ErrorContains(t, err, "preview")
			require.Equal(t, saved, read(t, books.Config.WriteJournal))
			_, err = os.Stat(books.Config.WriteJournal + ".wat-lock")
			require.True(t, os.IsNotExist(err))
			require.NotContains(t, saved, "* Electric")
		})
	}
}

func TestStaleDraftAndLock(t *testing.T) {
	for _, changed := range []string{"books/2021.journal", "books/accounts.journal", "include-set", "lock"} {
		t.Run(changed, func(t *testing.T) {
			books, root := fixture(t, "; original\n")
			draft, err := books.NewDraft()
			require.NoError(t, err)
			_, err = draft.Preview(testEntry())
			require.NoError(t, err)
			switch changed {
			case "include-set":
				require.NoError(t, os.WriteFile(filepath.Join(root, "books/extra.journal"), []byte("; new\n"), 0600))
				require.NoError(t, os.WriteFile(books.Config.Journal, []byte("include accounts.journal\ninclude 2021.journal\ninclude extra.journal\n"), 0600))
			case "lock":
				require.NoError(t, os.WriteFile(books.Config.WriteJournal+".wat-lock", []byte("other writer"), 0600))
			default:
				path := filepath.Join(root, changed)
				require.NoError(t, os.WriteFile(path, []byte(read(t, path)+"; external change\n"), 0600))
			}
			before := read(t, books.Config.WriteJournal)
			_, err = draft.Save()
			require.Error(t, err)
			require.Equal(t, before, read(t, books.Config.WriteJournal))
		})
	}
}

func TestAssertionFailureDoesNotChangeBooks(t *testing.T) {
	original := "2021-12-31 Bank checkpoint\n    Assets:Bank  $0 = $0\n    Equity:Capital  $0\n"
	books, _ := fixture(t, original)
	draft, err := books.NewDraft()
	require.NoError(t, err)
	_, err = draft.Preview(testEntry())
	require.NoError(t, err)
	_, err = draft.Save()
	require.ErrorContains(t, err, "validation")
	require.Equal(t, original, read(t, books.Config.WriteJournal))
}

func TestUnsupportedIncludes(t *testing.T) {
	t.Run("absolute", func(t *testing.T) {
		books, root := fixture(t, "")
		includes := "include accounts.journal\ninclude " + books.Config.WriteJournal + "\n"
		require.NoError(t, os.WriteFile(filepath.Join(root, "books/books.journal"), []byte(includes), 0600))
		draft, err := books.NewDraft()
		require.NoError(t, err)
		_, err = draft.Preview(testEntry())
		require.NoError(t, err)
		_, err = draft.Save()
		require.ErrorContains(t, err, "escapes")
		require.Empty(t, read(t, books.Config.WriteJournal))
	})
	t.Run("symlink", func(t *testing.T) {
		books, root := fixture(t, "")
		alias := filepath.Join(root, "books/alias.journal")
		require.NoError(t, os.Symlink(books.Config.WriteJournal, alias))
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
    Assets:Bank  $100
    Equity:Capital  $-100

2021-04-05 Power
    Expenses:Power  $10
    Assets:Bank  $-10

2022-01-01 Future
    Assets:Bank  $900
    Income:Other  $-900
`
	require.NoError(t, os.WriteFile(filepath.Join(root, "books/2021.journal"), []byte(history), 0600))
	for _, name := range []string{"trial-balance", "balance-sheet"} {
		report, err := books.Report(name)
		require.NoError(t, err)
		require.Contains(t, report, "$90")
		require.NotContains(t, report, "$990")
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
