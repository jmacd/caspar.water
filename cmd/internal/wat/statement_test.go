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
		{`"USD"`, `"GBP"`, "requires USD currency"},
		{`"-12.34"`, `"-12.345"`, "exact decimal amount"},
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

func TestStatementCanCrossCalendarYearBoundary(t *testing.T) {
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
  {"booking_date":"2021-04-01","description":"BEGINNING BALANCE","currency":"","amount":"0.00","account_id":"Checking"},`, 1)

	decoded, err := books.decodeStatement([]byte(withMarker), statement.Hash, dir)
	require.NoError(t, err)
	require.Equal(t, statement.Rows, decoded.Rows, "a zero marker must not shift or create postable rows")
	require.Contains(t, decoded.Warnings, "Ignored 1 zero-amount statement row; no ledger posting is required.")
}

func TestStatementAccountIdentifierRoutesToUniqueBankJournal(t *testing.T) {
	books, root := fixture(t, "")
	savings := addBankAccount(t, books, "Bank:131.200:Savings")
	extraction := strings.ReplaceAll(statementJSON, `"account_id":"Checking"`, `"account_id":"0000-Savings"`)
	pdf := filepath.Join(root, "savings.pdf")
	jsonPath := filepath.Join(root, "savings.json")
	require.NoError(t, os.WriteFile(pdf, []byte("%PDF-1.7\nsavings\n"), 0600))
	require.NoError(t, os.WriteFile(jsonPath, []byte(extraction), 0600))

	statement, err := books.LoadStatement(pdf, StatementOptions{Extraction: jsonPath}, nil)
	require.NoError(t, err)
	require.Equal(t, savings.Account, statement.BankAccount)
	cache := read(t, filepath.Join(filepath.Dir(filepath.Join(root, statement.Source)), "extracted.json"))
	require.Contains(t, cache, `"wat_bank_account":"Bank:131.200:Savings"`)

	draft, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	entry.Payee = "Power company"
	entry.Postings[1].Account = "Expense:615.000:Utility"
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
	info, err := os.Stat(books.Config.Sources)
	require.NoError(t, err)
	require.True(t, info.IsDir())
	_, err = os.Lstat(filepath.Join(books.Config.Sources, "statements"))
	require.True(t, os.IsNotExist(err), "browsing must not create the statement cache directory")
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
		{"linked check payee cache", "not a regular file", func(t *testing.T, _ *Books, dir string) {
			target := filepath.Join(dir, "check-payees.saved")
			require.NoError(t, os.WriteFile(target, []byte("{}"), 0600))
			require.NoError(t, os.Symlink(target, filepath.Join(dir, statementCheckPayeesFile)))
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
	require.Equal(t, "Bank:131.200:Checking", entry.Postings[0].Account)
	require.Equal(t, "-12.34", entry.Postings[0].Amount)
	require.Equal(t, "12.34", entry.Postings[1].Amount)
	require.Empty(t, entry.Postings[1].Account)
	entry.Postings[1].Account = "Expense:615.000:Utility"
	entry.Payee = "Electric company"
	text, err := draft.Preview(entry)
	require.NoError(t, err)
	for _, mutated := range []string{
		strings.ReplaceAll(text, "12.34", "13.34"),
		strings.Replace(text, "2021-04-05", "2021-04-06", 1),
		strings.Replace(text, "Bank:131.200:Checking", "Equity:218.000:Owner", 1),
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
	staleEntry.Postings[1].Account = "Expense:615.000:Utility"
	_, err = draft.Save()
	require.NoError(t, err)
	_, err = stale.Preview(staleEntry)
	require.ErrorContains(t, err, "journal changed")

	status, err := books.StatementStatuses(s)
	require.NoError(t, err)
	require.True(t, status[1].Recorded)
	require.Equal(t, "Expense:615.000:Utility", status[2].Suggestion, "learn using original bank description even if payee was edited")
	require.Empty(t, status[0].Suggestion, "an incoming deposit must not be guessed as income")
	_, _, err = books.StatementDraft(s, 1)
	require.ErrorContains(t, err, "already recorded")
	draft, entry, err = books.StatementDraft(s, 2)
	require.NoError(t, err)
	require.Equal(t, "Expense:615.000:Utility", entry.Postings[1].Account)
	entry.Postings[1].Account = "Equity:218.000:Owner"
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
	entry.Postings[1] = Posting{"Equity:218.000:Owner", "-60.00"}
	entry.Postings = append(entry.Postings, Posting{"Expense:615.000:Utility", "-40.00"})
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	s.Rows[0].Description = "DIFFERENT SOURCE ROW"
	_, err = books.StatementStatuses(s)
	require.ErrorContains(t, err, "recorded statement row 1 does not match linked journal transaction")
	require.ErrorContains(t, err, draft.id)
	require.ErrorContains(t, err, "statement row: 2021-04-01 | DIFFERENT SOURCE ROW | 100.00 USD")
	require.ErrorContains(t, err, "journal entry: 2021-04-01 | OPENING DEPOSIT | 100.00 USD")
	require.ErrorContains(t, err, "mismatched evidence: original bank description link")
	require.ErrorContains(t, err, "books/bank-checking.journal:")
}

func TestChangedRecordedRowReportsExpectedAndActualDate(t *testing.T) {
	books, s, _ := statementFixture(t)
	draft, entry, err := books.StatementDraft(s, 1)
	require.NoError(t, err)
	entry.Postings[1].Account = "Expense:615.000:Utility"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)

	journal := bankJournal(t, books, s.BankAccount)
	text := read(t, journal)
	require.NoError(t, os.WriteFile(
		journal,
		[]byte(strings.Replace(text, "2021-04-05", "2021-04-06", 1)),
		0600,
	))
	_, err = books.StatementStatuses(s)
	require.ErrorContains(t, err, "recorded statement row 2 does not match linked journal transaction")
	require.ErrorContains(t, err, draft.id)
	require.ErrorContains(t, err, "statement row: 2021-04-05 | POWER CO | -12.34 USD")
	require.ErrorContains(t, err, "journal entry: 2021-04-06 | POWER CO | -12.34 USD")
	require.ErrorContains(t, err, "mismatched evidence: date")
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

func TestStatementAutomaticallyExtractsCheckPayees(t *testing.T) {
	books, root := fixture(t, "")
	require.NoError(t, os.WriteFile(books.Config.Payees, []byte(`payee Deposit
payee Electric company
payee Feiner Fixings
payee Journal
`), 0600))
	pdfBytes := []byte("%PDF-1.7\nsynthetic check statement\n")
	pdf := filepath.Join(root, "checks.pdf")
	require.NoError(t, os.WriteFile(pdf, pdfBytes, 0600))
	extraction := strings.Replace(
		statementJSON,
		`"description":"POWER CO"`,
		`"description":"CHECK 1001"`,
		1,
	)
	extractionPath := filepath.Join(root, "check-extraction.json")
	require.NoError(t, os.WriteFile(extractionPath, []byte(extraction), 0600))
	hash := statementHash(pdfBytes)
	payeesPath := filepath.Join(root, "check-payees-response.json")
	postingAccounts, err := books.PostingAccounts()
	require.NoError(t, err)
	var declaredAccounts []string
	for _, account := range postingAccounts {
		if !strings.HasPrefix(account.Name, "Bank:") {
			declaredAccounts = append(declaredAccounts, account.Name)
		}
	}
	transactionPrompts := []StatementTransactionPrompt{{
		Payee:  "Feiner Fixings",
		Prompt: "Split the total between chlorine and operations.",
	}}
	response, err := json.Marshal(statementCheckPayeeCache{
		Version:            1,
		StatementHash:      hash,
		Model:              "ollama/test",
		CreatedAt:          "2025-01-02T03:04:05Z",
		Candidates:         []int{2},
		DeclaredPayees:     []string{"Deposit", "Electric company", "Feiner Fixings", "Journal"},
		DeclaredAccounts:   declaredAccounts,
		TransactionPrompts: transactionPrompts,
		Payees: []statementCheckPayee{{
			Row: 2, Payee: "Feiner Fixings", Confidence: 0.97,
			Memo: "Chlorine and operations work",
			Postings: []statementSuggestedPosting{
				{Account: "Expense:615.000:Utility", Amount: "5.00"},
				{Account: "Income:421.000:Misc", Amount: "7.34"},
			},
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(payeesPath, response, 0600))
	requestPath := filepath.Join(root, "check-payees-request.json")
	fake := filepath.Join(root, "fake-statement-python")
	require.NoError(t, os.WriteFile(fake, []byte(`#!/bin/sh
set -eu
if printf '%s' "$2" | grep -q CHECK_PAYEE_SYSTEM_PROMPT; then
  cat > "$WAT_FAKE_REQUEST"
  cat "$WAT_FAKE_PAYEES"
else
  cat "$WAT_FAKE_EXTRACTION"
fi
`), 0700))
	t.Setenv("WAT_FAKE_EXTRACTION", extractionPath)
	t.Setenv("WAT_FAKE_PAYEES", payeesPath)
	t.Setenv("WAT_FAKE_REQUEST", requestPath)
	books.Config.Statement = StatementConfig{
		Python: fake, APIBase: "http://invalid.test:11434", Model: "ollama/test",
		TransactionPrompts: transactionPrompts,
	}

	var progress strings.Builder
	statement, err := books.LoadStatement(pdf, StatementOptions{}, &progress)
	require.NoError(t, err)
	require.True(t, statement.checkPayeesLoaded)
	require.Equal(t, "Feiner Fixings", statement.checkPayees[1].Payee)
	require.Contains(t, progress.String(), "sending statement page images")
	require.Contains(t, progress.String(), "http://invalid.test:11434")
	var request statementCheckPayeeRequest
	require.NoError(t, json.Unmarshal([]byte(read(t, requestPath)), &request))
	require.Equal(t, []string{"Deposit", "Electric company", "Feiner Fixings", "Journal"}, request.DeclaredPayees)
	require.Equal(t, declaredAccounts, request.DeclaredAccounts)
	require.Equal(t, transactionPrompts, request.TransactionPrompts)
	cache := filepath.Join(root, filepath.Dir(statement.Source), statementCheckPayeesFile)
	require.FileExists(t, cache)

	_, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, "Feiner Fixings", entry.Payee)
	require.Equal(t, "Chlorine and operations work", entry.Description)
	require.Equal(t, []Posting{
		{Account: "Bank:131.200:Checking", Amount: "-12.34"},
		{Account: "Expense:615.000:Utility", Amount: "5.00"},
		{Account: "Income:421.000:Misc", Amount: "7.34"},
	}, entry.Postings)
	require.Empty(t, read(t, books.Config.ManualJournal), "AI defaults must not write journals")
	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.Contains(t, statuses[1].CheckPayeeEvidence, "Feiner Fixings")
	require.Contains(t, statuses[1].CheckPayeeEvidence, "97% confidence")
	require.NoError(t, books.saveQuickBooksMappings([]quickBooksAccountMapping{{
		Label: "Utilities", Account: "Expense:615.000:Utility",
	}}))

	writeStatementRegister(t, books,
		"4/5/2021,Expense,1001,Feiner Fixings,April power,Utilities,12.34,,87.66\n",
	)
	_, entry, err = books.StatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, "Feiner Fixings", entry.Payee)
	require.Equal(t, "April power", entry.Description, "a nonempty QuickBooks memo is stronger evidence")
	require.Len(t, entry.Postings, 3, "a QuickBooks account must not replace a matched AI split")

	writeStatementRegister(t, books,
		"4/5/2021,Expense,1001,Electric Company,April power,Utilities,12.34,,87.66\n",
	)
	books.quickBooksRegisterYear = make(map[int][]quickBooksRecord)
	_, entry, err = books.StatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, "Electric Company", entry.Payee, "an exact QuickBooks match is stronger evidence")
	require.Equal(t, []Posting{
		{Account: "Bank:131.200:Checking", Amount: "-12.34"},
		{Account: "Expense:615.000:Utility", Amount: "12.34"},
	}, entry.Postings, "a QuickBooks payee conflict must suppress the unrelated AI split")

	books.Config.Statement.Python = filepath.Join(root, "missing-python")
	again, err := books.LoadStatement(pdf, StatementOptions{}, nil)
	require.NoError(t, err, "the validated sidecar must prevent another model call")
	require.Equal(t, "Feiner Fixings", again.checkPayees[1].Payee)
	books.Config.Statement.TransactionPrompts[0].Prompt = "Use a changed split instruction."
	_, err = books.LoadStatement(pdf, StatementOptions{}, nil)
	require.Error(t, err, "changing a transaction prompt must invalidate cached AI defaults")
}

func TestEnsureStatementCheckPayeesEnrichesOlderCache(t *testing.T) {
	books, root := fixture(t, "")
	pdfBytes := []byte("%PDF-1.7\nolder cached check statement\n")
	pdf := filepath.Join(root, "older-checks.pdf")
	extractionPath := filepath.Join(root, "older-check-extraction.json")
	require.NoError(t, os.WriteFile(pdf, pdfBytes, 0600))
	require.NoError(t, os.WriteFile(extractionPath, []byte(strings.Replace(
		statementJSON,
		`"description":"POWER CO"`,
		`"description":"CHECK 1001"`,
		1,
	)), 0600))
	statement, err := books.LoadStatement(pdf, StatementOptions{Extraction: extractionPath}, nil)
	require.NoError(t, err)
	require.False(t, statement.checkPayeesLoaded)
	cacheDir := filepath.Join(root, filepath.Dir(statement.Source))
	require.NoError(t, os.WriteFile(filepath.Join(cacheDir, statementCheckPayeesFile), []byte(`{
 "statement_hash":"`+statement.Hash+`",
 "model":"azure/gpt-5-mini-statements",
 "created_at":"2025-01-02T03:04:05Z",
 "candidates":[2],
 "payees":[]
}`), 0600))
	statement, err = books.cachedStatement(statement.Hash)
	require.NoError(t, err)
	require.False(t, statement.checkPayeesLoaded, "an older empty sidecar must be retried automatically")

	response := filepath.Join(root, "older-check-payees.json")
	postingAccounts, err := books.PostingAccounts()
	require.NoError(t, err)
	var declaredAccounts []string
	for _, account := range postingAccounts {
		if !strings.HasPrefix(account.Name, "Bank:") {
			declaredAccounts = append(declaredAccounts, account.Name)
		}
	}
	responseData, err := json.Marshal(statementCheckPayeeCache{
		Version:          1,
		StatementHash:    statement.Hash,
		Model:            "ollama/test",
		CreatedAt:        "2025-01-02T03:04:05Z",
		Candidates:       []int{2},
		DeclaredPayees:   []string{"Deposit", "Electric company", "Journal"},
		DeclaredAccounts: declaredAccounts,
		Payees: []statementCheckPayee{{
			Row: 2, Payee: "Feiner Fixings", Confidence: 0.97,
		}},
	})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(response, responseData, 0600))
	fake := filepath.Join(root, "fake-check-payee-python")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\ncat \"$WAT_FAKE_PAYEES\"\n"), 0700))
	t.Setenv("WAT_FAKE_PAYEES", response)
	books.Config.Statement = StatementConfig{
		Python: fake, APIBase: "http://invalid.test:11434", Model: "ollama/test",
	}

	require.NoError(t, books.EnsureStatementCheckPayees(statement, nil))
	require.True(t, statement.checkPayeesLoaded)
	require.Equal(t, "Feiner Fixings", statement.checkPayees[1].Payee)
	require.FileExists(t, filepath.Join(cacheDir, statementCheckPayeesFile))
}

func TestStatementTransactionPromptRequiresDeclaredPayee(t *testing.T) {
	books, _ := fixture(t, "")
	books.Config.Statement = StatementConfig{
		Python:  "python3",
		APIBase: "http://invalid.test:11434",
		Model:   "ollama/test",
		TransactionPrompts: []StatementTransactionPrompt{{
			Payee:  "Feiner Fixings",
			Prompt: "Split chlorine from operations.",
		}},
	}
	_, err := books.extractStatementCheckPayees("", &Statement{
		Rows: []StatementRow{{
			Date: "2021-04-05", Description: "CHECK 1001", Cents: -1234,
		}},
	}, nil)
	require.ErrorContains(t, err, `payee "Feiner Fixings" is not declared`)
}

func TestStatementCheckPayeeCacheValidation(t *testing.T) {
	statement := &Statement{
		Hash: strings.Repeat("a", 64),
		Rows: []StatementRow{
			{Date: "2021-04-05", Description: "CHECK 1001", Cents: -1234},
			{Date: "2021-04-06", Description: "CARD PURCHASE", Cents: -2000},
		},
	}
	valid := `{
 "version":1,
 "statement_hash":"` + statement.Hash + `",
 "model":"azure/test",
 "created_at":"2025-01-02T03:04:05Z",
 "candidates":[1],
 "declared_payees":["Alpha Labs","Feiner Fixings"],
 "declared_accounts":["Expense:615.000:Chlorine","Expense:615.000:Operations"],
 "transaction_prompts":[{"payee":"Feiner Fixings","prompt":"Split chlorine from operations."}],
 "payees":[{"row":1,"payee":"Feiner Fixings","confidence":0.97,"memo":"Treatment work","postings":[{"account":"Expense:615.000:Chlorine","amount":"5.00"},{"account":"Expense:615.000:Operations","amount":"7.34"}]}]
}`
	cache, err := decodeStatementCheckPayees([]byte(valid), statement)
	require.NoError(t, err)
	require.Equal(t, "Feiner Fixings", cache.Payees[0].Payee)
	require.Equal(t, "Treatment work", cache.Payees[0].Memo)
	require.Len(t, cache.Payees[0].Postings, 2)
	legacy, err := decodeStatementCheckPayees([]byte(`{
 "statement_hash":"`+statement.Hash+`",
 "model":"azure/test",
 "created_at":"2025-01-02T03:04:05Z",
 "candidates":[1],
 "payees":[{"row":1,"payee":"Feiner Fixings","confidence":0.97}]
}`), statement)
	require.NoError(t, err)
	require.Zero(t, legacy.Version, "older caches must be eligible for automatic re-extraction")

	for _, test := range []struct {
		name, data, message string
	}{
		{"wrong hash", strings.Replace(valid, statement.Hash, strings.Repeat("b", 64), 1), "fingerprint"},
		{"wrong candidates", strings.Replace(valid, `"candidates":[1]`, `"candidates":[2]`, 1), "check rows"},
		{"duplicate row", strings.Replace(valid,
			`"payees":[`,
			`"payees":[{"row":1,"payee":"Other","confidence":0.8},`, 1),
			"duplicate"},
		{"invalid payee", strings.Replace(valid,
			`"row":1,"payee":"Feiner Fixings"`,
			`"row":1,"payee":"Feiner | Fixings"`, 1),
			"invalid payee"},
		{"invalid confidence", strings.Replace(valid, "0.97", "1.01", 1), "invalid confidence"},
		{"duplicate declared payee", strings.Replace(valid,
			`"declared_payees":["Alpha Labs","Feiner Fixings"]`,
			`"declared_payees":["Alpha Labs","Alpha Labs","Feiner Fixings"]`, 1),
			"duplicated"},
		{"unsorted declared payees", strings.Replace(valid,
			`"declared_payees":["Alpha Labs","Feiner Fixings"]`,
			`"declared_payees":["Feiner Fixings","Alpha Labs"]`, 1),
			"not sorted"},
		{"duplicate declared account", strings.Replace(valid,
			`"declared_accounts":["Expense:615.000:Chlorine","Expense:615.000:Operations"]`,
			`"declared_accounts":["Expense:615.000:Chlorine","Expense:615.000:Chlorine"]`, 1),
			"duplicated"},
		{"duplicate transaction prompt", strings.Replace(valid,
			`"transaction_prompts":[{"payee":"Feiner Fixings","prompt":"Split chlorine from operations."}]`,
			`"transaction_prompts":[{"payee":"Feiner Fixings","prompt":"First."},{"payee":"Feiner Fixings","prompt":"Second."}]`, 1),
			"duplicated"},
		{"unknown posting account", strings.Replace(valid,
			`"account":"Expense:615.000:Chlorine"`,
			`"account":"Expense:615.000:Unknown"`, 1),
			"undeclared account"},
		{"duplicate posting account", strings.Replace(valid,
			`"account":"Expense:615.000:Operations"`,
			`"account":"Expense:615.000:Chlorine"`, 1),
			"more than once"},
		{"inexact posting amount", strings.Replace(valid, `"amount":"5.00"`, `"amount":"5.0"`, 1), "invalid amount"},
		{"unbalanced postings", strings.Replace(valid, `"amount":"7.34"`, `"amount":"7.33"`, 1), "do not balance"},
		{"instructions without matched payee", strings.Replace(valid,
			`"payee":"Feiner Fixings","confidence":0.97`,
			`"payee":"Other","confidence":0.97`, 1),
			"unmatched payee"},
		{"unsupported version", strings.Replace(valid, `"version":1`, `"version":2`, 1), "unsupported version"},
		{"extra field", strings.Replace(valid, `"model":"azure/test"`, `"model":"azure/test","unexpected":true`, 1), "unknown field"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeStatementCheckPayees([]byte(test.data), statement)
			require.ErrorContains(t, err, test.message)
		})
	}
}

func TestStatementCheckRowDetection(t *testing.T) {
	require.True(t, statementRowIsCheck(StatementRow{
		Date: "2022-03-04", Description: "CHECK 1006", Cents: -321807,
	}))
	require.True(t, statementRowIsCheck(StatementRow{
		Date: "2022-03-04", Description: "1006 03/04/2022 $3,218.07", Cents: -321807,
	}))
	for _, row := range []StatementRow{
		{Date: "2022-03-05", Description: "1006 03/04/2022 $3,218.07", Cents: -321807},
		{Date: "2022-03-04", Description: "1006 03/04/2022 $3,218.08", Cents: -321807},
		{Date: "2022-03-04", Description: "1006 03/04/2022 $3,218.07", Cents: 321807},
		{Date: "2022-03-04", Description: "ORDER 1006 03/04/2022 $3,218.07", Cents: -321807},
	} {
		require.False(t, statementRowIsCheck(row), row.Description)
	}
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
		require.NoError(t, json.Unmarshal([]byte(`{"tpostings":[{"paccount":"Bank:131.200:Checking","ptype":"RegularPosting","pamount":[{"acommodity":"$","aquantity":`+tc.quantity+`}]}]}`), &tx))
		cents, err := bankAmount(tx, "Bank:131.200:Checking")
		if tc.failure != "" {
			require.ErrorContains(t, err, tc.failure)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.expected, cents)
		}
	}
}
