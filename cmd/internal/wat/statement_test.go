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
  {"booking_date":"2021-04-01","description":"OPENING DEPOSIT","currency":"USD","amount":"100.00","account_id":"Checking"},
  {"booking_date":"2021-04-05","description":"POWER CO","currency":"USD","amount":"-12.34","account_id":"Checking"},
  {"booking_date":"2021-05-05","description":"POWER CO","currency":"USD","amount":"-20.00","account_id":"Checking"}
 ],
 "verification":{"opening_balance":"0.00","closing_balance":"67.66"},
 "warnings":["Compare with the PDF"]
}`

func statementFixture(t *testing.T) (*Books, *Statement, string) {
	t.Helper()
	books, root := fixture(t, "")
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
	_, err = books.LoadStatement(pdf, StatementOptions{Extraction: filepath.Join(books.Config.root, "extraction.json")}, nil)
	require.ErrorContains(t, err, "already cached")
	require.Empty(t, read(t, books.Config.ManualJournal))

	for _, test := range []struct{ old, replacement, message string }{
		{`"67.66"`, `"67.65"`, "do not match"},
		{`"USD"`, `"GBP"`, "exact USD"},
		{`"-12.34"`, `"-12.345"`, "exact USD"},
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

func TestStatementCanCrossReportYearBoundary(t *testing.T) {
	books, statement, _ := statementFixture(t)
	dir := filepath.Dir(filepath.Join(books.Config.root, statement.Source))
	crossYear := strings.NewReplacer(
		`"start":"2021-04-01"`, `"start":"2021-12-01"`,
		`"end":"2021-05-31"`, `"end":"2022-01-31"`,
		`"booking_date":"2021-04-01"`, `"booking_date":"2021-12-01"`,
		`"booking_date":"2021-04-05"`, `"booking_date":"2021-12-05"`,
		`"booking_date":"2021-05-05"`, `"booking_date":"2022-01-05"`,
	).Replace(statementJSON)
	decoded, err := books.decodeStatement([]byte(crossYear), statement.Hash, dir)
	require.NoError(t, err)
	require.Equal(t, "2021-12-01", decoded.Period.Start)
	require.Equal(t, "2022-01-05", decoded.Rows[2].Date)
	require.Equal(t, books.Config.BankAccounts[0].Account, decoded.BankAccount)
}

func TestStatementIgnoresZeroAmountBalanceMarkers(t *testing.T) {
	books, statement, _ := statementFixture(t)
	dir := filepath.Dir(filepath.Join(books.Config.root, statement.Source))
	withMarker := strings.Replace(statementJSON, `"transactions":[`, `"transactions":[
  {"booking_date":"2021-04-01","description":"BEGINNING BALANCE","currency":"USD","amount":"0.00","account_id":"Checking"},`, 1)

	decoded, err := books.decodeStatement([]byte(withMarker), statement.Hash, dir)
	require.NoError(t, err)
	require.Equal(t, statement.Rows, decoded.Rows, "a zero marker must not shift or create postable rows")
	require.Contains(t, decoded.Warnings, "Ignored 1 zero-amount statement row; no ledger posting is required.")
}

func TestStatementAccountIdentifierRoutesToUniqueBankJournal(t *testing.T) {
	books, root := fixture(t, "")
	savings := addBankAccount(t, books, "Bank:131.2:Savings", "savings.journal")
	extraction := strings.ReplaceAll(statementJSON, `"account_id":"Checking"`, `"account_id":"0000-Savings"`)
	pdf := filepath.Join(root, "savings.pdf")
	jsonPath := filepath.Join(root, "savings.json")
	require.NoError(t, os.WriteFile(pdf, []byte("%PDF-1.7\nsavings\n"), 0600))
	require.NoError(t, os.WriteFile(jsonPath, []byte(extraction), 0600))

	statement, err := books.LoadStatement(pdf, StatementOptions{Extraction: jsonPath}, nil)
	require.NoError(t, err)
	require.Equal(t, savings.Account, statement.BankAccount)
	cache := read(t, filepath.Join(filepath.Dir(filepath.Join(root, statement.Source)), "extracted.json"))
	require.Contains(t, cache, `"wat_bank_account":"Bank:131.2:Savings"`)

	draft, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	entry.Payee = "Power company"
	entry.Postings[1].Account = "Expense:615:Utility"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	require.Contains(t, read(t, savings.Journal), savings.Account)
	require.Empty(t, read(t, books.Config.BankAccounts[0].Journal))
	require.Empty(t, read(t, books.Config.ManualJournal))
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
	books.Config.Statement = StatementConfig{Python: "missing-python", APIBase: "http://invalid.test", Model: "ollama/unused"}
	cacheRoot := filepath.Join(books.Config.Sources, "statements")
	require.NoError(t, os.Mkdir(filepath.Join(cacheRoot, ".extract-unfinished"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(cacheRoot, "notes.txt"), []byte("not a statement"), 0600))
	statements, err := books.CachedStatements()
	require.NoError(t, err)
	require.Equal(t, []*Statement{first, later}, statements, "sort by extracted activity dates, not PDF fingerprints")
	require.Empty(t, read(t, books.Config.ManualJournal), "browsing must not post anything")
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
			require.Empty(t, read(t, books.Config.ManualJournal))
		})
	}
}

func TestStatementRecordingLearningAndProvenance(t *testing.T) {
	books, s, _ := statementFixture(t)
	draft, entry, err := books.StatementDraft(s, 1)
	require.NoError(t, err)
	require.Equal(t, "Bank:131.2:Checking", entry.Postings[0].Account)
	require.Equal(t, "-12.34", entry.Postings[0].Amount)
	require.Equal(t, "12.34", entry.Postings[1].Amount)
	require.Empty(t, entry.Postings[1].Account)
	entry.Postings[1].Account = "Expense:615:Utility"
	entry.Payee = "Electric company"
	text, err := draft.Preview(entry)
	require.NoError(t, err)
	for _, mutated := range []string{
		strings.ReplaceAll(text, "12.34", "13.34"),
		strings.Replace(text, "2021-04-05", "2021-04-06", 1),
		strings.Replace(text, "Bank:131.2:Checking", "Equity:218:Owner", 1),
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
	staleEntry.Postings[1].Account = "Expense:615:Utility"
	_, err = draft.Save()
	require.NoError(t, err)
	_, err = stale.Preview(staleEntry)
	require.ErrorContains(t, err, "journal changed")

	status, err := books.StatementStatuses(s)
	require.NoError(t, err)
	require.True(t, status[1].Recorded)
	require.Equal(t, "Expense:615:Utility", status[2].Suggestion, "learn using original bank description even if payee was edited")
	require.Empty(t, status[0].Suggestion, "an incoming deposit must not be guessed as income")
	_, _, err = books.StatementDraft(s, 1)
	require.ErrorContains(t, err, "already recorded")
	draft, entry, err = books.StatementDraft(s, 2)
	require.NoError(t, err)
	require.Equal(t, "Expense:615:Utility", entry.Postings[1].Account)
	entry.Postings[1].Account = "Equity:218:Owner"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	future := &Statement{Hash: "other", BankAccount: s.BankAccount, Rows: []StatementRow{{Date: "2021-06-01", Description: "POWER CO", Cents: -100}}}
	status, err = books.StatementStatuses(future)
	require.NoError(t, err)
	require.Empty(t, status[0].Suggestion, "conflicting accepted assignments must not become a rule")
	status, err = books.StatementStatuses(&Statement{Hash: "other", BankAccount: s.BankAccount, Rows: []StatementRow{s.Rows[1]}})
	require.NoError(t, err)
	require.True(t, status[0].PossibleMatch, "overlapping/manual rows must be flagged, not silently posted or skipped")
	require.False(t, status[0].Recorded)
}

func TestStatementSplitAndChangedRecordedRow(t *testing.T) {
	books, s, _ := statementFixture(t)
	draft, entry, err := books.StatementDraft(s, 0)
	require.NoError(t, err)
	entry.Postings[1] = Posting{"Equity:218:Owner", "-60.00"}
	entry.Postings = append(entry.Postings, Posting{"Expense:615:Utility", "-40.00"})
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
	books.Config.Statement = StatementConfig{Python: fake, APIBase: "http://invalid.test:11434", Model: "ollama/test"}
	s, err := books.LoadStatement(pdf, StatementOptions{}, nil)
	require.NoError(t, err)
	require.Len(t, s.Rows, 3)
	books.Config.Statement.Python = filepath.Join(root, "missing-python")
	_, err = books.LoadStatement(pdf, StatementOptions{}, nil)
	require.NoError(t, err, "the second run must use cached data, not invoke extraction")
}

func TestStatementParserRuntimeOverride(t *testing.T) {
	books, _, _ := statementFixture(t)
	root := books.Config.root
	pdf := filepath.Join(root, "runtime-override.pdf")
	require.NoError(t, os.WriteFile(pdf, []byte("%PDF-1.7\nruntime override source\n"), 0600))
	fake := filepath.Join(root, "runtime-parser")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\ncat '"+filepath.Join(root, "extraction.json")+"'\n"), 0700))
	books.Config.Statement = StatementConfig{
		Python:  filepath.Join(root, "missing-python"),
		APIBase: "http://invalid.test:11434",
		Model:   "ollama/test",
	}
	t.Setenv("WAT_STATEMENT_PYTHON", fake)

	statement, err := books.LoadStatement(pdf, StatementOptions{}, nil)
	require.NoError(t, err)
	require.Len(t, statement.Rows, 3)
	require.Equal(t, books.Config.Statement.Python, filepath.Join(root, "missing-python"), "runtime override must not rewrite accounting configuration")
}

func TestStatementInboxCopiesRegularPDFsIntoPendingCache(t *testing.T) {
	books, root := fixture(t, "")
	extraction := filepath.Join(root, "inbox-extraction.json")
	require.NoError(t, os.WriteFile(extraction, []byte(statementJSON), 0600))

	inbox := filepath.Join(root, "inbox")
	require.NoError(t, os.Mkdir(inbox, 0700))
	firstBytes := []byte("%PDF-1.7\nfirst inbox statement\n")
	secondBytes := []byte("%PDF-1.7\nsecond inbox statement\n")
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "02-second.PDF"), secondBytes, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "01-first.pdf"), firstBytes, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "notes.txt"), []byte("ignored"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(inbox, "duplicate.pdf"), firstBytes, 0600))

	var progress strings.Builder
	require.NoError(t, books.ImportStatementInbox(inbox, &progress))
	require.Contains(t, progress.String(), "01-first.pdf")
	require.Contains(t, progress.String(), "02-second.PDF")
	pending, err := books.PendingStatements()
	require.NoError(t, err)
	require.Len(t, pending, 2, "identical PDF content must share one stable pending cache")
	require.Equal(t, "01-first.pdf", pending[0].Name)
	statements, err := books.CachedStatements()
	require.NoError(t, err)
	require.Empty(t, statements, "copying inbox files must not run statement extraction")
	for _, statement := range pending {
		cached, err := os.ReadFile(filepath.Join(books.Config.root, statement.Source))
		require.NoError(t, err)
		require.Contains(t, []string{string(firstBytes), string(secondBytes)}, string(cached))
	}

	require.NoError(t, os.RemoveAll(inbox))
	pending, err = books.PendingStatements()
	require.NoError(t, err)
	require.Len(t, pending, 2, "pending statements must not depend on the host inbox")

	extracted, err := books.ExtractPendingStatement(pending[0].Hash, StatementOptions{Extraction: extraction}, nil)
	require.NoError(t, err)
	require.Equal(t, pending[0].Hash, extracted.Hash)
	pending, err = books.PendingStatements()
	require.NoError(t, err)
	require.Len(t, pending, 1, "extracting one statement must leave the other statement pending")
	statements, err = books.CachedStatements()
	require.NoError(t, err)
	require.Len(t, statements, 1)
}

func TestStatementInboxRejectsSymlinksAndNonregularPDFs(t *testing.T) {
	books, root := fixture(t, "")
	inbox := filepath.Join(root, "inbox")
	require.NoError(t, os.Mkdir(inbox, 0700))
	outside := filepath.Join(root, "outside.pdf")
	require.NoError(t, os.WriteFile(outside, []byte("%PDF-1.7\noutside\n"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(inbox, "linked.pdf")))
	require.ErrorContains(t, books.ImportStatementInbox(inbox, nil), "not a regular file")

	require.NoError(t, os.Remove(filepath.Join(inbox, "linked.pdf")))
	require.NoError(t, os.Mkdir(filepath.Join(inbox, "directory.pdf"), 0700))
	require.ErrorContains(t, books.ImportStatementInbox(inbox, nil), "not a regular file")

	linkedInbox := filepath.Join(root, "linked-inbox")
	require.NoError(t, os.Symlink(inbox, linkedInbox))
	require.ErrorContains(t, books.ImportStatementInbox(linkedInbox, nil), "not a regular directory")
	require.Empty(t, read(t, books.Config.ManualJournal))
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
		require.NoError(t, json.Unmarshal([]byte(`{"tpostings":[{"paccount":"Bank:131.2:Checking","ptype":"RegularPosting","pamount":[{"acommodity":"$","aquantity":`+tc.quantity+`}]}]}`), &tx))
		cents, err := bankAmount(tx, "Bank:131.2:Checking")
		if tc.failure != "" {
			require.ErrorContains(t, err, tc.failure)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.expected, cents)
		}
	}
}
