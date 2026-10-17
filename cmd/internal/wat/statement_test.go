package wat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const statementJSON = `{
 "wat_statement_period":{"start":"2021-04-01","end":"2021-05-31"},
 "transactions":[
  {"booking_date":"2021-04-01","description":"OPENING DEPOSIT","currency":"USD","amount":"100.00"},
  {"booking_date":"2021-04-05","description":"POWER CO","currency":"USD","amount":"-12.34"},
  {"booking_date":"2021-05-05","description":"POWER CO","currency":"USD","amount":"-20.00"}
 ],
 "verification":{"opening_balance":"0.00","closing_balance":"67.66"},
 "warnings":["Compare with the PDF"]
}`

func statementFixture(t *testing.T) (*Books, *Statement, string) {
	t.Helper()
	books, root := fixture(t, "")
	books.Config.BankAccount = "Assets:Bank:Checking"
	pdf, extraction := filepath.Join(root, "statement.pdf"), filepath.Join(root, "extraction.json")
	require.NoError(t, os.WriteFile(pdf, []byte("%PDF-1.7\nsynthetic source for cache tests\n"), 0600))
	require.NoError(t, os.WriteFile(extraction, []byte(statementJSON), 0600))
	s, err := books.LoadStatement(pdf, StatementOptions{Extraction: extraction}, nil)
	require.NoError(t, err)
	return books, s, pdf
}

func TestStatementCacheAndValidation(t *testing.T) {
	books, s, pdf := statementFixture(t)
	require.Len(t, s.Rows, 3)
	require.Equal(t, &StatementPeriod{Start: "2021-04-01", End: "2021-05-31"}, s.Period)
	require.Equal(t, int64(-1234), s.Rows[1].Cents)
	again, err := books.LoadStatement(pdf, StatementOptions{}, nil)
	require.NoError(t, err)
	require.Equal(t, s, again, "cached extraction must not need Python or Ollama settings")
	_, err = books.LoadStatement(pdf, StatementOptions{Pages: []int{1}}, nil)
	require.ErrorContains(t, err, "already cached")
	require.Empty(t, read(t, books.Config.WriteJournal))

	for _, test := range []struct{ old, replacement, message string }{
		{`"67.66"`, `"67.65"`, "do not match"},
		{`"USD"`, `"GBP"`, "exact USD"},
		{`"-12.34"`, `"-12.345"`, "exact USD"},
		{`"2021-04-05"`, `"2022-04-05"`, "within 2021"},
		{`"POWER CO"`, `"POWER; CO"`, "description"},
	} {
		t.Run(test.message+test.replacement, func(t *testing.T) {
			_, err := books.decodeStatement([]byte(strings.ReplaceAll(statementJSON, test.old, test.replacement)), s.Hash, filepath.Dir(filepath.Join(books.Config.root, s.Source)))
			require.ErrorContains(t, err, test.message)
		})
	}
	require.NoError(t, os.WriteFile(filepath.Join(books.Config.root, s.Source), []byte("%PDF-changed"), 0600))
	_, err = books.LoadStatement(pdf, StatementOptions{}, nil)
	require.ErrorContains(t, err, "fingerprint")
}

func TestStatementPeriodValidation(t *testing.T) {
	books, statement, _ := statementFixture(t)
	dir := filepath.Dir(filepath.Join(books.Config.root, statement.Source))
	for _, test := range []struct{ old, replacement, message string }{
		{`"start":"2021-04-01"`, `"start":"2021-04-31"`, "valid YYYY-MM-DD"},
		{`"end":"2021-05-31"`, `"end":"2021-05-32"`, "valid YYYY-MM-DD"},
		{`"start":"2021-04-01"`, `"start":"2021-06-01"`, "in order"},
		{`"start":"2021-04-01"`, `"start":"2021-04-02"`, "row 1 is outside"},
		{`"end":"2021-05-31"`, `"end":"2021-05-04"`, "row 3 is outside"},
	} {
		t.Run(test.replacement, func(t *testing.T) {
			_, err := books.decodeStatement([]byte(strings.Replace(statementJSON, test.old, test.replacement, 1)), statement.Hash, dir)
			require.ErrorContains(t, err, test.message)
		})
	}
	legacy := strings.Replace(statementJSON, ` "wat_statement_period":{"start":"2021-04-01","end":"2021-05-31"},`, "", 1)
	decoded, err := books.decodeStatement([]byte(legacy), statement.Hash, dir)
	require.NoError(t, err)
	require.Nil(t, decoded.Period, "do not infer statement dates from transactions")
	require.Equal(t, statement.Rows, decoded.Rows)
	for index := range statement.Rows {
		require.Equal(t, statement.key(index), decoded.key(index), "period metadata must not change row identity")
	}
}

func TestCachedStatementsResumeWithoutInputFiles(t *testing.T) {
	books, first, pdf := statementFixture(t)
	root := books.Config.root
	laterPDF := filepath.Join(root, "later.pdf")
	laterExtraction := filepath.Join(root, "later.json")
	require.NoError(t, os.WriteFile(laterPDF, []byte("%PDF-1.7\nlater synthetic statement\n"), 0600))
	laterJSON := strings.ReplaceAll(strings.ReplaceAll(statementJSON, "2021-04-", "2021-11-"), "2021-05-", "2021-12-")
	require.NoError(t, os.WriteFile(laterExtraction, []byte(laterJSON), 0600))
	later, err := books.LoadStatement(laterPDF, StatementOptions{Extraction: laterExtraction}, nil)
	require.NoError(t, err)
	for _, path := range []string{pdf, filepath.Join(root, "extraction.json"), laterPDF, laterExtraction} {
		require.NoError(t, os.Remove(path))
	}
	books.Config.Statement = StatementConfig{Python: "missing-python", OllamaURL: "http://invalid.test", Model: "unused"}
	cacheRoot := filepath.Join(books.Config.Sources, "statements")
	require.NoError(t, os.Mkdir(filepath.Join(cacheRoot, ".extract-unfinished"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(cacheRoot, "notes.txt"), []byte("not a statement"), 0600))
	statements, err := books.CachedStatements()
	require.NoError(t, err)
	require.Equal(t, []*Statement{first, later}, statements, "sort by extracted activity dates, not PDF fingerprints")
	require.Empty(t, read(t, books.Config.WriteJournal), "browsing must not post anything")
}

func TestCachedStatementsEmptyWorkspace(t *testing.T) {
	books, _ := fixture(t, "")
	statements, err := books.CachedStatements()
	require.NoError(t, err)
	require.Empty(t, statements)
	_, err = os.Lstat(books.Config.Sources)
	require.True(t, os.IsNotExist(err), "browsing must not create cache directories")
	require.NoError(t, os.MkdirAll(filepath.Join(books.Config.Sources, "statements"), 0700))
	statements, err = books.CachedStatements()
	require.NoError(t, err)
	require.Empty(t, statements)
}

func TestCachedStatementsRejectInvalidCache(t *testing.T) {
	for _, test := range []struct {
		name, message string
		change        func(*testing.T, *Books, string)
	}{
		{"changed PDF", "fingerprint", func(t *testing.T, _ *Books, dir string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "original.pdf"), []byte("%PDF-changed"), 0600))
		}},
		{"missing extraction", "cache is incomplete", func(t *testing.T, _ *Books, dir string) {
			require.NoError(t, os.Remove(filepath.Join(dir, "extracted.json")))
		}},
		{"invalid extraction", "decode statement", func(t *testing.T, _ *Books, dir string) {
			require.NoError(t, os.WriteFile(filepath.Join(dir, "extracted.json"), []byte("{"), 0600))
		}},
		{"linked PDF", "nonregular files", func(t *testing.T, _ *Books, dir string) {
			path := filepath.Join(dir, "original.pdf")
			require.NoError(t, os.Rename(path, path+".saved"))
			require.NoError(t, os.Symlink(path+".saved", path))
		}},
		{"linked cache directory", "not a regular directory", func(t *testing.T, _ *Books, dir string) {
			require.NoError(t, os.Rename(dir, dir+".saved"))
			require.NoError(t, os.Symlink(dir+".saved", dir))
		}},
		{"linked source directory", "not a regular directory", func(t *testing.T, books *Books, _ string) {
			require.NoError(t, os.Rename(books.Config.Sources, books.Config.Sources+".saved"))
			require.NoError(t, os.Symlink(books.Config.Sources+".saved", books.Config.Sources))
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			books, statement, _ := statementFixture(t)
			dir := filepath.Dir(filepath.Join(books.Config.root, statement.Source))
			test.change(t, books, dir)
			_, err := books.CachedStatements()
			require.ErrorContains(t, err, test.message)
			require.Empty(t, read(t, books.Config.WriteJournal))
		})
	}
}

func TestStatementRecordingLearningAndProvenance(t *testing.T) {
	books, s, _ := statementFixture(t)
	draft, entry, err := books.StatementDraft(s, 1)
	require.NoError(t, err)
	require.Equal(t, "Assets:Bank:Checking", entry.Postings[0].Account)
	require.Equal(t, "-12.34", entry.Postings[0].Amount)
	require.Equal(t, "12.34", entry.Postings[1].Amount)
	require.Empty(t, entry.Postings[1].Account)
	entry.Postings[1].Account = "Expenses:Power:Utility"
	entry.Payee = "Electric company"
	text, err := draft.Preview(entry)
	require.NoError(t, err)
	for _, mutated := range []string{
		strings.ReplaceAll(text, "12.34", "13.34"),
		strings.Replace(text, "2021-04-05", "2021-04-06", 1),
		strings.Replace(text, "Assets:Bank:Checking", "Equity:Capital:Owner", 1),
		strings.Replace(text, "bank-row:", "missing-row:", 1),
		strings.Replace(text, "bank-description:", "missing-description:", 1),
	} {
		require.Error(t, draft.PreviewText(mutated), mutated)
		require.Empty(t, draft.text)
	}
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	stale, staleEntry, err := books.StatementDraft(s, 1)
	require.NoError(t, err)
	staleEntry.Postings[1].Account = "Expenses:Power:Utility"
	_, err = draft.Save()
	require.NoError(t, err)
	_, err = stale.Preview(staleEntry)
	require.ErrorContains(t, err, "journal changed")

	status, err := books.StatementStatuses(s)
	require.NoError(t, err)
	require.True(t, status[1].Recorded)
	require.Equal(t, "Expenses:Power:Utility", status[2].Suggestion, "learn using original bank description even if payee was edited")
	require.Empty(t, status[0].Suggestion, "an incoming deposit must not be guessed as income")
	_, _, err = books.StatementDraft(s, 1)
	require.ErrorContains(t, err, "already recorded")
	draft, entry, err = books.StatementDraft(s, 2)
	require.NoError(t, err)
	require.Equal(t, "Expenses:Power:Utility", entry.Postings[1].Account)
	entry.Postings[1].Account = "Equity:Capital:Owner"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	future := &Statement{Hash: "other", Rows: []StatementRow{{Date: "2021-06-01", Description: "POWER CO", Cents: -100}}}
	status, err = books.StatementStatuses(future)
	require.NoError(t, err)
	require.Empty(t, status[0].Suggestion, "conflicting accepted assignments must not become a rule")
	status, err = books.StatementStatuses(&Statement{Hash: "other", Rows: []StatementRow{s.Rows[1]}})
	require.NoError(t, err)
	require.True(t, status[0].PossibleMatch, "overlapping/manual rows must be flagged, not silently posted or skipped")
	require.False(t, status[0].Recorded)
}

func TestStatementSplitAndChangedRecordedRow(t *testing.T) {
	books, s, _ := statementFixture(t)
	draft, entry, err := books.StatementDraft(s, 0)
	require.NoError(t, err)
	entry.Postings[1] = Posting{"Equity:Capital:Owner", "-60.00"}
	entry.Postings = append(entry.Postings, Posting{"Expenses:Power:Utility", "-40.00"})
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	s.Rows[0].Description = "DIFFERENT SOURCE ROW"
	_, err = books.StatementStatuses(s)
	require.ErrorContains(t, err, "no longer matches")
}

func TestStatementParserInvocation(t *testing.T) {
	books, _, _ := statementFixture(t)
	root := books.Config.root
	pdf := filepath.Join(root, "another.pdf")
	require.NoError(t, os.WriteFile(pdf, []byte("%PDF-1.7\nanother synthetic source\n"), 0600))
	fake := filepath.Join(root, "parser")
	// A synthetic subprocess stands in for Python; tests never send bank data.
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\ncat '"+filepath.Join(root, "extraction.json")+"'\n"), 0700))
	books.Config.Statement = StatementConfig{Python: fake, OllamaURL: "http://invalid.test:11434", Model: "test"}
	s, err := books.LoadStatement(pdf, StatementOptions{}, nil)
	require.NoError(t, err)
	require.Len(t, s.Rows, 3)
	books.Config.Statement.Python = filepath.Join(root, "missing-python")
	_, err = books.LoadStatement(pdf, StatementOptions{}, nil)
	require.NoError(t, err, "the second run must use cached data, not invoke extraction")
}

func TestStatementCacheRejectsSymlinksAndPartialData(t *testing.T) {
	books, s, pdf := statementFixture(t)
	cache := filepath.Dir(filepath.Join(books.Config.root, s.Source))
	require.NoError(t, os.Remove(filepath.Join(cache, "extracted.json")))
	_, err := books.LoadStatement(pdf, StatementOptions{}, nil)
	require.ErrorContains(t, err, "cache is incomplete")
	outside := t.TempDir()
	linked := filepath.Join(books.Config.root, "linked")
	require.NoError(t, os.Symlink(outside, linked))
	books.Config.Sources = linked
	_, err = books.LoadStatement(pdf, StatementOptions{}, nil)
	require.ErrorContains(t, err, "not a regular directory")
	files, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestBankAmountExactQuantities(t *testing.T) {
	for _, tc := range []struct {
		quantity string
		expected int64
		failure  string
	}{
		{`{"decimalMantissa":1234000000000000000000,"decimalPlaces":20}`, 1234, ""},
		{`{"decimalMantissa":1,"decimalPlaces":3}`, 0, "fractional cents"},
		{`{"decimalMantissa":9223372036854775808,"decimalPlaces":2}`, 0, "exceeds supported"},
	} {
		var tx journalTransaction
		require.NoError(t, json.Unmarshal([]byte(`{"tpostings":[{"paccount":"Assets:Bank:Checking","ptype":"RegularPosting","pamount":[{"acommodity":"$","aquantity":`+tc.quantity+`}]}]}`), &tx))
		cents, err := bankAmount(tx, "Assets:Bank:Checking")
		if tc.failure != "" {
			require.ErrorContains(t, err, tc.failure)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.expected, cents)
		}
	}
}
