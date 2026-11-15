package wat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func statementAccountFixture(t *testing.T) (*Books, *Statement) {
	t.Helper()
	books, statement, _ := statementFixture(t)
	configurePayees(t, books, "PG&E")
	draft, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	entry.Payee, entry.Description = "PG&E", "Electricity"
	entry.Postings[1].Account = "Expense:615.000:Utility"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	statement.Rows[2].Description = "PGANDE/WEB ONLINE 20.00"
	statement.Rows = append(statement.Rows, StatementRow{Date: "2021-05-06", Description: "PGANDE/WEB ONLINE 21.00", Cents: -2100})
	statement.ClosingCents = 4666
	extraction := strings.Replace(statementJSON,
		`{"booking_date":"2021-05-05","description":"POWER CO","currency":"USD","amount":"-20.00","account_id":"Checking"}`,
		`{"booking_date":"2021-05-05","description":"PGANDE/WEB ONLINE 20.00","currency":"USD","amount":"-20.00","account_id":"Checking"},
         {"booking_date":"2021-05-06","description":"PGANDE/WEB ONLINE 21.00","currency":"USD","amount":"-21.00","account_id":"Checking"}`, 1)
	extraction = strings.Replace(extraction, `"67.66"`, `"46.66"`, 1)
	require.NoError(t, os.WriteFile(filepath.Join(books.Config.root, filepath.Dir(statement.Source), "extracted.json"), []byte(extraction), 0600))
	return books, statement
}

func TestStatementAccountAISuggestsKnownVendorWithAmount(t *testing.T) {
	books, statement := statementAccountFixture(t)
	originalJournal := read(t, bankJournal(t, books, statement.BankAccount))
	extractionPath := filepath.Join(books.Config.root, filepath.Dir(statement.Source), "extracted.json")
	originalExtraction := read(t, extractionPath)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		requests.Add(1)
		require.Equal(t, "/api/chat", request.URL.Path)
		var payload struct {
			Model    string        `json:"model"`
			Messages []chatMessage `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Equal(t, "test-model", payload.Model)
		require.Len(t, payload.Messages, 2)
		var batch struct {
			Rows []statementAccountInput `json:"rows"`
		}
		require.NoError(t, json.Unmarshal([]byte(payload.Messages[1].Content), &batch))
		require.Len(t, batch.Rows, 2, "all unmatched rows belong in one request")
		input := batch.Rows[0]
		require.Equal(t, "PGANDE/WEB ONLINE 20.00", input.Row.Description)
		require.Equal(t, "-20.00", input.Row.Amount)
		require.Contains(t, input.DeclaredPayees, "PG&E")
		require.Contains(t, input.ConfiguredAccounts, "Expense:615.000:Utility")
		require.NotContains(t, input.ConfiguredAccounts, statement.BankAccount)
		require.Equal(t, []statementAccountExample{{
			ID: 1, Description: "PG&E | Electricity", Accounts: []string{"Expense:615.000:Utility"},
		}}, input.Examples)
		require.Equal(t, 4, batch.Rows[1].Row.Row)
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
			"message": chatMessage{Role: "assistant", Content: `{"suggestions":[
                {"row":3,"account":"Expense:615.000:Utility","example":1,"explanation":"PGANDE is PG&E; the trailing 20.00 is the payment amount."},
                {"row":4,"account":"Expense:615.000:Utility","example":1,"explanation":"Same PG&E vendor."}]}`},
		}))
	}))
	defer server.Close()
	books.Config.Statement = StatementConfig{APIBase: server.URL, Model: "ollama/test-model"}
	_, entry, err := books.StatementDraft(statement, 2)
	require.NoError(t, err)
	require.Zero(t, requests.Load(), "constructing drafts must not make network requests")
	require.Empty(t, entry.Postings[1].Account, "exact description learning does not recognize this alias")
	err = books.EnsureStatementAccountSuggestions(context.Background(), statement)
	require.NoError(t, err)
	suggestion := statement.accountSuggestions[2]
	require.Equal(t, "Expense:615.000:Utility", suggestion.Account)
	require.Contains(t, suggestion.hint(), "AI suggestion: Expense:615.000:Utility")
	require.Contains(t, suggestion.hint(), "PG&E")
	require.Equal(t, int32(1), requests.Load())
	require.Empty(t, entry.Postings[1].Account, "the completion does not mutate the entry")
	require.Equal(t, originalJournal, read(t, bankJournal(t, books, statement.BankAccount)))
	require.Equal(t, originalExtraction, read(t, extractionPath), "original evidence must remain unchanged")
	again, err := books.cachedStatement(statement.Hash)
	require.NoError(t, err)
	require.True(t, again.accountSuggestionsLoaded, "inference must survive reopening the statement")
	require.NoError(t, books.EnsureStatementAccountSuggestions(context.Background(), again))
	for _, index := range []int{2, 3} {
		_, entry, err := books.StatementDraft(again, index)
		require.NoError(t, err)
		require.Equal(t, "Expense:615.000:Utility", entry.Postings[1].Account)
	}
	require.Equal(t, int32(1), requests.Load(), "reopening the statement and entering rows must not request inference again")
}

func TestStatementAccountContextPreservesBankDirectionAndSplitEvidence(t *testing.T) {
	books, statement := statementAccountFixture(t)
	addBankAccount(t, books, "Bank:131.200:Savings")
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(`
2021-04-06 PG&E | Refund
    Bank:131.200:Checking  $5.00
    Expense:615.000:Utility  $-5.00

2021-04-07 PG&E | Another bank
    Bank:131.200:Savings  $-5.00
    Expense:615.000:Utility  $5.00

2021-04-08 PG&E | Split
    Bank:131.200:Checking  $-5.00
    Expense:615.000:Utility  $3.00
    Equity:218.000:Owner  $2.00

2021-04-09 PG&E | Transfer
    Bank:131.200:Checking  $-5.00
    Bank:131.200:Savings  $5.00
`), 0600))
	inputs, err := books.statementAccountInputs(statement)
	require.NoError(t, err)
	require.Len(t, inputs, 3)
	require.Equal(t, "PG&E | Refund", inputs[0].Examples[0].Description)
	input := inputs[1]
	require.Len(t, input.Examples, 2)
	require.Equal(t, "PG&E | Electricity", input.Examples[0].Description)
	require.Equal(t, "PG&E | Split", input.Examples[1].Description)
	require.Equal(t, []string{"Equity:218.000:Owner", "Expense:615.000:Utility"}, input.Examples[1].Accounts)
}

func TestStatementAccountAIAbstentionAndFailures(t *testing.T) {
	for _, test := range []struct {
		name, content, wantError string
		status                   int
	}{
		{name: "abstain", content: `{"suggestions":[{"row":3,"account":"","example":0,"explanation":"Vendor is ambiguous."},{"row":4,"account":"","example":0,"explanation":"Vendor is ambiguous."}]}`, status: http.StatusOK},
		{name: "invalid JSON", content: "not JSON", status: http.StatusOK, wantError: "decode AI account suggestion"},
		{name: "invented account", content: `{"suggestions":[{"row":3,"account":"Expense:999.000:Invented","example":1,"explanation":"Guess."},{"row":4,"account":"","example":0,"explanation":"Unknown."}]}`, status: http.StatusOK, wantError: "undeclared account"},
		{name: "transport failure", content: "model unavailable", status: http.StatusServiceUnavailable, wantError: "503"},
	} {
		t.Run(test.name, func(t *testing.T) {
			books, statement := statementAccountFixture(t)
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(test.status)
				require.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
					"message": chatMessage{Role: "assistant", Content: test.content},
				}))
			}))
			defer server.Close()
			books.Config.Statement = StatementConfig{APIBase: server.URL, Model: "ollama/test-model"}
			err := books.EnsureStatementAccountSuggestions(context.Background(), statement)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
				require.False(t, statement.accountSuggestionsLoaded)
				require.NoFileExists(t, filepath.Join(books.Config.root, filepath.Dir(statement.Source), statementAccountSuggestionsFile))
			} else {
				require.NoError(t, err)
				suggestion := statement.accountSuggestions[2]
				require.Empty(t, suggestion.Account)
				require.Contains(t, suggestion.hint(), "no confident account match")
				require.Contains(t, suggestion.hint(), "Vendor is ambiguous")
			}
			_, entry, err := books.StatementDraft(statement, 2)
			require.NoError(t, err)
			require.Empty(t, entry.Postings[1].Account)
		})
	}
}

func TestStatementAccountSuggestionRequiresGroundedUnambiguousOutput(t *testing.T) {
	input := statementAccountInput{
		Row:                statementCheckPayeeRequestRow{Row: 3},
		Payee:              "PGANDE/WEB ONLINE 20.00",
		ConfiguredAccounts: []string{"Expense:615.000:Utility", "Equity:218.000:Owner"},
		Examples: []statementAccountExample{
			{ID: 1, Description: "PG&E | Electricity", Accounts: []string{"Expense:615.000:Utility"}},
			{ID: 2, Description: "Other | Split", Accounts: []string{"Expense:615.000:Utility", "Equity:218.000:Owner"}},
		},
	}
	valid := `{"suggestions":[{"row":3,"account":"Expense:615.000:Utility","example":1,"explanation":"Same vendor."}]}`
	for _, test := range []struct{ name, content, wantError string }{
		{"missing evidence", strings.Replace(valid, `"example":1`, `"example":0`, 1), "supporting single-account"},
		{"wrong account", strings.Replace(valid, "Expense:615.000:Utility", "Equity:218.000:Owner", 1), "supporting single-account"},
		{"split example", strings.Replace(valid, `"example":1`, `"example":2`, 1), "supporting single-account"},
		{"missing explanation", `{"suggestions":[{"row":3,"account":"","example":0}]}`, "explanation"},
		{"multiline explanation", strings.Replace(valid, "Same vendor.", `Same\nvendor.`, 1), "explanation"},
		{"extra field", strings.Replace(valid, `"example":1`, `"example":1,"payee":"PG&E"`, 1), "unknown field"},
		{"multiple objects", valid + valid, "exactly one JSON object"},
		{"abstention with citation", `{"suggestions":[{"row":3,"account":"","example":1,"explanation":"Unknown."}]}`, "must not cite"},
		{"missing row", `{"suggestions":[]}`, "cover every requested row"},
		{"missing array", `{}`, "cover every requested row"},
		{"unexpected row", strings.Replace(valid, `"row":3`, `"row":99`, 1), "unexpected or duplicate row"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeStatementAccountSuggestions(test.content, []statementAccountInput{input})
			require.ErrorContains(t, err, test.wantError)
		})
	}
	input.Examples = append(input.Examples, statementAccountExample{
		ID: 3, Description: "PG&E | Different allocation", Accounts: []string{"Equity:218.000:Owner"},
	})
	_, err := decodeStatementAccountSuggestions(valid, []statementAccountInput{input})
	require.ErrorContains(t, err, "conflicting journal history")
	input.Examples = input.Examples[:2]
	second := input
	second.Row.Row = 4
	repeated := strings.TrimSuffix(valid, "]}") + `,{"row":3,"account":"","example":0,"explanation":"Unknown."}]}`
	_, err = decodeStatementAccountSuggestions(repeated, []statementAccountInput{input, second})
	require.ErrorContains(t, err, "unexpected or duplicate row")
}

func TestStatementAccountAISkipsRequestsWithoutExamples(t *testing.T) {
	books, statement, _ := statementFixture(t)
	books.Config.Statement = StatementConfig{APIBase: "http://invalid.test", Model: "ollama/test"}
	err := books.EnsureStatementAccountSuggestions(context.Background(), statement)
	require.NoError(t, err)
	require.True(t, statement.accountSuggestionsLoaded)
	require.Empty(t, statement.accountSuggestions)
}

func TestStatementAccountCacheRejectsChangedEvidence(t *testing.T) {
	books, statement := statementAccountFixture(t)
	inputs, err := books.statementAccountInputs(statement)
	require.NoError(t, err)
	cache := statementAccountCache{
		Version: statementAccountCacheVersion, StatementHash: statement.Hash, BankAccount: statement.BankAccount,
		Model: "ollama/test", CreatedAt: "2026-11-14T00:00:00Z",
		Rows: statementAccountRows(statement), Inputs: inputs,
		Suggestions: []statementAccountRowSuggestion{
			{Row: 3, statementAccountSuggestion: statementAccountSuggestion{
				Account: "Expense:615.000:Utility", Example: 1, Explanation: "Known vendor.",
			}},
			{Row: 4, statementAccountSuggestion: statementAccountSuggestion{
				Account: "", Example: 0, Explanation: "Uncertain.",
			}},
		},
	}

	data, err := json.Marshal(cache)
	require.NoError(t, err)
	dir := filepath.Dir(filepath.Join(books.Config.root, statement.Source))
	for _, test := range []struct{ name, old, replacement, wantError string }{
		{"statement fingerprint", statement.Hash, strings.Repeat("0", 64), "do not match the statement"},
		{"bank binding", statement.BankAccount, "Bank:131.200:Other", "do not match the statement"},
		{"bank evidence", "PGANDE/WEB ONLINE 20.00", "Changed source", "do not match the statement"},
		{"missing output", `"row":4,"account":""`, `"row":3,"account":""`, "unexpected or duplicate row"},
		{"invented account", `"account":"Expense:615.000:Utility"`, `"account":"Expense:999.000:Invented"`, "undeclared account"},
	} {
		t.Run(test.name, func(t *testing.T) {
			changed := strings.Replace(string(data), test.old, test.replacement, 1)
			require.NotEqual(t, string(data), changed)
			require.NoError(t, os.WriteFile(filepath.Join(dir, statementAccountSuggestionsFile), []byte(changed), 0600))
			again := *statement
			err := loadStatementAccountSuggestions(&again, dir)
			require.ErrorContains(t, err, test.wantError)
			require.False(t, again.accountSuggestionsLoaded, "invalid caches must not be partially applied")
		})
	}
}

func TestStatementAccountLearningRejectsCheckReferences(t *testing.T) {
	books, statement := statementAccountFixture(t)
	configurePayees(t, books, "PG&E", "1012")
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(`
2021-04-10 1012 | Prior payment
    Bank:131.200:Checking  $-20.00
    Equity:218.000:Owner  $20.00
`), 0600))
	for _, description := range []string{
		"1012", "1013", "Check 1013", "CHECK #1013", "Cheque No. 1013",
		"1013 05/05/2021 $20.00",
	} {
		t.Run(description, func(t *testing.T) {
			statement.Rows[2].Description = description
			statuses, err := books.StatementStatuses(statement)
			require.NoError(t, err)
			require.Empty(t, statuses[2].Suggestion, "even an identical check number is not account evidence")
			inputs, err := books.statementAccountInputs(statement)
			require.NoError(t, err)
			require.Len(t, inputs, 1, "reference-only rows must not be sent to the model")
			require.Equal(t, 4, inputs[0].Row.Row)
			require.Len(t, inputs[0].Examples, 1)
			require.Equal(t, "PG&E | Electricity", inputs[0].Examples[0].Description,
				"reference-only historical payees must not be offered as examples")
		})
	}
	statement.checkPayees = map[int]statementCheckPayee{2: {Payee: "PG&E", Confidence: 0.99}}
	inputs, err := books.statementAccountInputs(statement)
	require.NoError(t, err)
	require.Len(t, inputs, 2, "a check with an identified payee can still receive a suggestion")
	require.Equal(t, "PG&E", inputs[0].Payee)
	require.NoError(t, validateStatementAccountSuggestion(statementAccountSuggestion{
		Account: "Expense:615.000:Utility", Example: 1, Explanation: "Identified PG&E payee.",
	}, inputs[0]))
}

func TestStatementAccountValidationRejectsNumericVendorEvidence(t *testing.T) {
	input := statementAccountInput{
		Payee:              "1013",
		ConfiguredAccounts: []string{"Expense:615.000:Utility"},
		Examples: []statementAccountExample{{
			ID: 1, Description: "1012 | Prior payment", Accounts: []string{"Expense:615.000:Utility"},
		}},
	}
	suggestion := statementAccountSuggestion{
		Account: "Expense:615.000:Utility", Example: 1, Explanation: "Sequential check numbers.",
	}
	require.ErrorContains(t, validateStatementAccountSuggestion(suggestion, input), "not a check number")
	input.Payee = "PG&E"
	require.ErrorContains(t, validateStatementAccountSuggestion(suggestion, input), "check number as vendor evidence")
	suggestion = statementAccountSuggestion{Explanation: "Check number alone does not identify a vendor."}
	require.NoError(t, validateStatementAccountSuggestion(suggestion, input))
}

func TestStatementRefreshRetriesCheckPayeesBeforeAccounts(t *testing.T) {
	books, statement := statementAccountFixture(t)
	statement.Rows[2].Description = "1013"
	dir := filepath.Dir(filepath.Join(books.Config.root, statement.Source))
	extractionPath := filepath.Join(dir, "extracted.json")
	extraction := strings.Replace(read(t, extractionPath), "PGANDE/WEB ONLINE 20.00", "1013", 1)
	require.NoError(t, os.WriteFile(extractionPath, []byte(extraction), 0600))
	journal := bankJournal(t, books, statement.BankAccount)
	originalJournal := read(t, journal)
	cache := statementCheckPayeeCache{
		Version: 1, StatementHash: statement.Hash, Model: "ollama/test", CreatedAt: "2026-11-14T00:00:00Z",
		Candidates: []int{3}, DeclaredPayees: []string{"PG&E"},
		DeclaredAccounts: []string{"Equity:218.000:Owner", "Expense:615.000:Utility", "Income:421.000:Misc"},
		Payees:           []statementCheckPayee{},
	}
	data, err := json.Marshal(cache)
	require.NoError(t, err)
	require.NoError(t, publishStatementCheckPayees(dir, data))
	applyStatementCheckPayees(statement, &cache)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		call := requests.Add(1)
		var payload struct{ Messages []chatMessage }
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		var input struct{ Rows []statementAccountInput }
		require.NoError(t, json.Unmarshal([]byte(payload.Messages[1].Content), &input))
		if call == 1 {
			require.Len(t, input.Rows, 1, "the unidentified numeric row must not reach account inference")
		} else {
			require.Len(t, input.Rows, 2)
			require.Equal(t, "1013", input.Rows[0].Row.Description)
			require.Equal(t, "PG&E", input.Rows[0].Payee, "account inference must receive the newly recognized payee")
		}
		response := statementAccountResponse{Suggestions: []statementAccountRowSuggestion{}}
		for _, row := range input.Rows {
			suggestion := statementAccountSuggestion{Explanation: "Unknown vendor."}
			if call > 1 {
				suggestion = statementAccountSuggestion{
					Account: "Expense:615.000:Utility", Example: 1, Explanation: "Identified PG&E payee.",
				}
			}
			response.Suggestions = append(response.Suggestions, statementAccountRowSuggestion{
				Row: row.Row.Row, statementAccountSuggestion: suggestion,
			})
		}
		content, err := json.Marshal(response)
		require.NoError(t, err)
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
			"message": chatMessage{Role: "assistant", Content: string(content)},
		}))
	}))
	defer server.Close()
	books.Config.Statement = StatementConfig{APIBase: server.URL, Model: "ollama/test"}
	require.NoError(t, books.EnsureStatementAccountSuggestions(context.Background(), statement))
	cache.Payees = []statementCheckPayee{{Row: 3, Payee: "PG&E", Confidence: 0.99}}
	data, err = json.Marshal(cache)
	require.NoError(t, err)
	response := filepath.Join(books.Config.root, "payee-response.json")
	require.NoError(t, os.WriteFile(response, data, 0600))
	fake := filepath.Join(books.Config.root, "fake-payee-python")
	require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\ncat \"$WAT_FAKE_PAYEES\"\n"), 0700))
	t.Setenv("WAT_FAKE_PAYEES", response)
	books.Config.Statement.Python = fake
	require.NoError(t, books.RefreshStatementSuggestions(context.Background(), statement, nil))
	require.Equal(t, int32(2), requests.Load(), "explicit refresh must bypass an earlier cached abstention")
	_, entry, err := books.StatementDraft(statement, 2)
	require.NoError(t, err)
	require.Equal(t, "PG&E", entry.Payee)
	require.Equal(t, "Expense:615.000:Utility", entry.Postings[1].Account)
	reopened, err := books.cachedStatement(statement.Hash)
	require.NoError(t, err)
	require.True(t, reopened.accountSuggestionsLoaded)
	require.NoError(t, books.EnsureStatementAccountSuggestions(context.Background(), reopened))
	require.Equal(t, int32(2), requests.Load(), "normal reopening still reuses refreshed results")
	require.Equal(t, originalJournal, read(t, journal))
	require.Equal(t, extraction, read(t, extractionPath))
	books.Config.Statement.Python = filepath.Join(books.Config.root, "missing-python")
	require.Error(t, books.RefreshStatementSuggestions(context.Background(), statement, nil))
	require.Equal(t, "PG&E", statement.checkPayees[2].Payee, "failed image extraction must retain earlier suggestions")
	require.Equal(t, originalJournal, read(t, journal))
	cache.Payees[0].Payee = "Another vendor"
	data, err = json.Marshal(cache)
	require.NoError(t, err)
	require.NoError(t, publishStatementCheckPayees(dir, data))
	reopened, err = books.cachedStatement(statement.Hash)
	require.NoError(t, err)
	require.False(t, reopened.accountSuggestionsLoaded, "changed check payees invalidate dependent account inference")
	require.Empty(t, reopened.accountSuggestions)
}

func TestStatementAccountOldCacheDoesNotReuseCheckNumberGuess(t *testing.T) {
	books, statement := statementAccountFixture(t)
	inputs, err := books.statementAccountInputs(statement)
	require.NoError(t, err)
	inputs[0].Payee = "1013"
	inputs[0].Examples[0].Description = "1012"
	cache := statementAccountCache{
		Version: 1, StatementHash: statement.Hash, BankAccount: statement.BankAccount,
		Model: "ollama/test", CreatedAt: "2026-11-14T00:00:00Z",
		Rows: statementAccountRows(statement), Inputs: inputs,
		Suggestions: []statementAccountRowSuggestion{
			{Row: 3, statementAccountSuggestion: statementAccountSuggestion{
				Account: "Expense:615.000:Utility", Example: 1, Explanation: "Adjacent check numbers.",
			}},
			{Row: 4, statementAccountSuggestion: statementAccountSuggestion{Explanation: "Unknown."}},
		},
	}
	data, err := json.Marshal(cache)
	require.NoError(t, err)
	dir := filepath.Dir(filepath.Join(books.Config.root, statement.Source))
	require.NoError(t, os.WriteFile(filepath.Join(dir, statementAccountSuggestionsFile), data, 0600))
	again, err := books.cachedStatement(statement.Hash)
	require.NoError(t, err)
	require.False(t, again.accountSuggestionsLoaded, "old inference must be regenerated on statement open")
	_, entry, err := books.StatementDraft(again, 2)
	require.NoError(t, err)
	require.Empty(t, entry.Postings[1].Account)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
			"message": chatMessage{Role: "assistant", Content: `{"suggestions":[
	                {"row":3,"account":"","example":0,"explanation":"Unknown vendor."},
	                {"row":4,"account":"","example":0,"explanation":"Unknown vendor."}]}`},
		}))
	}))
	defer server.Close()
	books.Config.Statement = StatementConfig{APIBase: server.URL, Model: "ollama/test"}
	require.NoError(t, books.EnsureStatementAccountSuggestions(context.Background(), again))
	require.NoError(t, json.Unmarshal([]byte(read(t, filepath.Join(dir, statementAccountSuggestionsFile))), &cache))
	require.Equal(t, statementAccountCacheVersion, cache.Version)
	reopened, err := books.cachedStatement(statement.Hash)
	require.NoError(t, err)
	require.True(t, reopened.accountSuggestionsLoaded)
	require.Empty(t, reopened.accountSuggestions[2].Account)
	cache.Version = 1
	cache.Inputs = nil
	cache.Suggestions = []statementAccountRowSuggestion{}
	data, err = json.Marshal(cache)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, statementAccountSuggestionsFile), data, 0600))
	reopened, err = books.cachedStatement(statement.Hash)
	require.NoError(t, err)
	require.False(t, reopened.accountSuggestionsLoaded, "empty old caches must also be regenerated")
}
