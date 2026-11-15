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
		var input statementAccountInput
		require.NoError(t, json.Unmarshal([]byte(payload.Messages[1].Content), &input))
		require.Equal(t, "PGANDE/WEB ONLINE 20.00", input.Row.Description)
		require.Equal(t, "-20.00", input.Row.Amount)
		require.Contains(t, input.DeclaredPayees, "PG&E")
		require.Contains(t, input.ConfiguredAccounts, "Expense:615.000:Utility")
		require.NotContains(t, input.ConfiguredAccounts, statement.BankAccount)
		require.Equal(t, []statementAccountExample{{
			ID: 1, Description: "PG&E | Electricity", Accounts: []string{"Expense:615.000:Utility"},
		}}, input.Examples)
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
			"message": chatMessage{Role: "assistant", Content: `{"account":"Expense:615.000:Utility","example":1,"explanation":"PGANDE is PG&E; the trailing 20.00 is the payment amount."}`},
		}))
	}))
	defer server.Close()
	books.Config.Statement = StatementConfig{APIBase: server.URL, Model: "ollama/test-model"}
	draft, entry, err := books.StatementDraft(statement, 2)
	require.NoError(t, err)
	require.Zero(t, requests.Load(), "constructing drafts must not make network requests")
	require.Empty(t, entry.Postings[1].Account, "exact description learning does not recognize this alias")
	suggestion, err := books.suggestStatementAccount(context.Background(), draft, entry)
	require.NoError(t, err)
	require.Equal(t, "Expense:615.000:Utility", suggestion.Account)
	require.Contains(t, suggestion.hint(), "AI suggestion: Expense:615.000:Utility")
	require.Contains(t, suggestion.hint(), "PG&E")
	require.Equal(t, int32(1), requests.Load())
	require.Empty(t, entry.Postings[1].Account, "the completion does not mutate the entry")
	require.Equal(t, originalJournal, read(t, bankJournal(t, books, statement.BankAccount)))
	require.Equal(t, originalExtraction, read(t, extractionPath), "original evidence must remain unchanged")
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
	draft, entry, err := books.StatementDraft(statement, 2)
	require.NoError(t, err)
	input, err := books.statementAccountContext(draft, entry)
	require.NoError(t, err)
	require.Len(t, input.Examples, 2)
	require.Equal(t, "PG&E | Electricity", input.Examples[0].Description)
	require.Equal(t, "PG&E | Split", input.Examples[1].Description)
	require.Equal(t, []string{"Equity:218.000:Owner", "Expense:615.000:Utility"}, input.Examples[1].Accounts)
}

func TestStatementAccountAIAbstentionAndFailures(t *testing.T) {
	books, statement := statementAccountFixture(t)
	draft, entry, err := books.StatementDraft(statement, 2)
	require.NoError(t, err)
	for _, test := range []struct {
		name, content, wantError string
		status                   int
	}{
		{name: "abstain", content: `{"account":"","example":0,"explanation":"Vendor is ambiguous."}`, status: http.StatusOK},
		{name: "invalid JSON", content: "not JSON", status: http.StatusOK, wantError: "decode AI account suggestion"},
		{name: "invented account", content: `{"account":"Expense:999.000:Invented","example":1,"explanation":"Guess."}`, status: http.StatusOK, wantError: "undeclared account"},
		{name: "transport failure", content: "model unavailable", status: http.StatusServiceUnavailable, wantError: "503"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(test.status)
				require.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
					"message": chatMessage{Role: "assistant", Content: test.content},
				}))
			}))
			defer server.Close()
			books.Config.Statement = StatementConfig{APIBase: server.URL, Model: "ollama/test-model"}
			suggestion, err := books.suggestStatementAccount(context.Background(), draft, entry)
			if test.wantError != "" {
				require.ErrorContains(t, err, test.wantError)
			} else {
				require.NoError(t, err)
				require.Empty(t, suggestion.Account)
				require.Contains(t, suggestion.hint(), "no confident account match")
				require.Contains(t, suggestion.hint(), "Vendor is ambiguous")
			}
			require.Empty(t, entry.Postings[1].Account)
		})
	}
}

func TestStatementAccountSuggestionRequiresGroundedUnambiguousOutput(t *testing.T) {
	input := statementAccountInput{
		ConfiguredAccounts: []string{"Expense:615.000:Utility", "Equity:218.000:Owner"},
		Examples: []statementAccountExample{
			{ID: 1, Description: "PG&E | Electricity", Accounts: []string{"Expense:615.000:Utility"}},
			{ID: 2, Description: "Other | Split", Accounts: []string{"Expense:615.000:Utility", "Equity:218.000:Owner"}},
		},
	}
	valid := `{"account":"Expense:615.000:Utility","example":1,"explanation":"Same vendor."}`
	for _, test := range []struct{ name, content, wantError string }{
		{"missing evidence", strings.Replace(valid, `"example":1`, `"example":0`, 1), "supporting single-account"},
		{"wrong account", strings.Replace(valid, "Expense:615.000:Utility", "Equity:218.000:Owner", 1), "supporting single-account"},
		{"split example", strings.Replace(valid, `"example":1`, `"example":2`, 1), "supporting single-account"},
		{"missing explanation", `{"account":"","example":0}`, "explanation"},
		{"multiline explanation", strings.Replace(valid, "Same vendor.", `Same\nvendor.`, 1), "explanation"},
		{"extra field", strings.Replace(valid, `"example":1`, `"example":1,"payee":"PG&E"`, 1), "unknown field"},
		{"multiple objects", valid + valid, "exactly one JSON object"},
		{"abstention with citation", `{"account":"","example":1,"explanation":"Unknown."}`, "must not cite"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeStatementAccountSuggestion(test.content, input)
			require.ErrorContains(t, err, test.wantError)
		})
	}
	input.Examples = append(input.Examples, statementAccountExample{
		ID: 3, Description: "PG&E | Different allocation", Accounts: []string{"Equity:218.000:Owner"},
	})
	_, err := decodeStatementAccountSuggestion(valid, input)
	require.ErrorContains(t, err, "conflicting journal history")
}

func TestStatementAccountAISkipsRequestsWithoutExamples(t *testing.T) {
	books, statement, _ := statementFixture(t)
	books.Config.Statement = StatementConfig{APIBase: "http://invalid.test", Model: "ollama/test"}
	draft, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	suggestion, err := books.suggestStatementAccount(context.Background(), draft, entry)
	require.NoError(t, err)
	require.Empty(t, suggestion.Account)
	entry.Postings[1].Account = "Expense:615.000:Utility"
	_, err = books.suggestStatementAccount(context.Background(), draft, entry)
	require.ErrorContains(t, err, "unmatched statement draft")
}
