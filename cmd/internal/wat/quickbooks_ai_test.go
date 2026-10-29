package wat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuickBooksOllamaCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/api/chat", request.URL.Path)
		var payload struct {
			Model  string `json:"model"`
			Format string `json:"format"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Equal(t, "test-model", payload.Model)
		require.Equal(t, "json", payload.Format)
		content := `{"candidates":[{"source_ids":["source:2"],"date":"2021-01-02","payee":"Vendor",` +
			`"description":"","reference":"","confidence":"high","postings":[` +
			`{"account":"Expense:615:Electricity","amount":"10.00"},` +
			`{"account":"Bank:131.2:SBMC:0000","amount":"-10.00"}]}],` +
			`"unresolved":[],"warnings":[]}`
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
			"message": map[string]string{"role": "assistant", "content": content},
		}))
	}))
	defer server.Close()

	books := &Books{Config: Config{Statement: StatementConfig{
		APIBase: server.URL, Model: "ollama/test-model",
	}}}
	response, err := books.quickBooksCompletion(context.Background(), quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
		Records: []quickBooksRecord{{ID: "source:2", Date: "2021-01-02"}},
	})
	require.NoError(t, err)
	require.Len(t, response.Candidates, 1)
	text, err := renderQuickBooksCandidates(response, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
	})
	require.NoError(t, err)
	require.Contains(t, text, "2021-01-02 Vendor")
	require.Contains(t, text, "Expense:615:Electricity  $10.00")
	require.NotContains(t, text, "source:")
}

func TestQuickBooksCandidatesRejectUnbalancedModelOutput(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			Date: "2021-01-02", Payee: "Vendor", Confidence: "low",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:615:Electricity", Amount: "10.00"},
				{Account: "Bank:131.2:SBMC:0000", Amount: "-9.00"},
			},
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
	})
	require.ErrorContains(t, err, "postings do not balance")
}

func TestQuickBooksCandidatesRejectUnknownAccount(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			Date: "2021-01-02", Payee: "Vendor", Confidence: "low",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:999:Invented", Amount: "10.00"},
				{Account: "Bank:131.2:SBMC:0000", Amount: "-10.00"},
			},
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
	})
	require.ErrorContains(t, err, "unconfigured account")
}

func TestQuickBooksCandidatesRejectCanonicalDuplicate(t *testing.T) {
	candidate := quickBooksAICandidate{
		Date: "2021-01-02", Payee: "Vendor", Confidence: "medium",
		Postings: []quickBooksAIPosting{
			{Account: "Expense:615:Electricity", Amount: "10.0"},
			{Account: "Bank:131.2:SBMC:0000", Amount: "-10.00"},
		},
	}
	reversed := candidate
	reversed.Postings = []quickBooksAIPosting{
		{Account: "Bank:131.2:SBMC:0000", Amount: "-10.0"},
		{Account: "Expense:615:Electricity", Amount: "10.00"},
	}
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{candidate, reversed},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
	})
	require.ErrorContains(t, err, "duplicates another generated transaction")
}

func TestQuickBooksCandidatesRejectAmountAbsentFromSource(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			SourceIDs: []string{"register:2"},
			Date:      "2021-01-02", Payee: "Vendor", Confidence: "medium",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:615:Electricity", Amount: "11.00"},
				{Account: "Bank:131.2:SBMC:0000", Amount: "-11.00"},
			},
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
		Records: []quickBooksRecord{{
			ID: "register:2", Fields: map[string]string{"Payment": "$10.00"},
		}},
	})
	require.ErrorContains(t, err, "total 11.00 is not present")
}

func TestQuickBooksUnresolvedRendersCommentedSkeleton(t *testing.T) {
	text, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{{
			SourceIDs:   []string{"report:2"},
			Date:        "2021-01-02",
			Payee:       "Professional firm",
			Description: "Business service",
			KnownPosting: quickBooksAIKnownPosting{
				QuickBooksAccount: "Professional services",
				Amount:            "10.00",
			},
			MissingFact: "The personal cash-side record is unavailable.",
			Question:    "Was this due to the owner or contributed capital?",
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		Records: []quickBooksRecord{{
			ID: "report:2", Date: "2021-01-02",
			Fields: map[string]string{"Amount": "10.00"},
		}},
	})
	require.NoError(t, err)
	require.Contains(t, text, "; === UNRESOLVED QUICKBOOKS TRANSACTIONS ===")
	require.Contains(t, text, "; 2021-01-02 Professional firm | Business service")
	require.Contains(t, text, ";     [QuickBooks: Professional services]  $10.00")
	require.Contains(t, text, ";     [TODO counterposting]  $-10.00")
	require.Contains(t, text, "; Question: Was this due to the owner or contributed capital?")
	for line := range strings.SplitSeq(text, "\n") {
		if line != "" {
			require.True(t, strings.HasPrefix(line, ";"), line)
		}
	}
}

func TestQuickBooksCandidatesRequireDispositionForEverySource(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		Records: []quickBooksRecord{{
			ID: "report:2", Date: "2021-01-02",
			Fields: map[string]string{"Amount": "10.00"},
		}},
	})
	require.ErrorContains(t, err, `source id "report:2" has no candidate or unresolved disposition`)
}

func TestQuickBooksCandidatesRejectReusedSource(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			SourceIDs: []string{"report:2"},
			Date:      "2021-01-02", Payee: "Vendor", Confidence: "high",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:615:Electricity", Amount: "10.00"},
				{Account: "Bank:131.2:SBMC:0000", Amount: "-10.00"},
			},
		}},
		Unresolved: []quickBooksAIUnresolved{{
			SourceIDs: []string{"report:2"}, Date: "2021-01-02",
			KnownPosting: quickBooksAIKnownPosting{
				QuickBooksAccount: "Electricity", Amount: "10.00",
			},
			MissingFact: "Missing cash side", Question: "Which account paid?",
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
		Records: []quickBooksRecord{{
			ID: "report:2", Date: "2021-01-02",
			Fields: map[string]string{"Amount": "10.00"},
		}},
	})
	require.ErrorContains(t, err, `reuses source id "report:2"`)
}

func TestParseQuickBooksCents(t *testing.T) {
	for value, expected := range map[string]int64{
		"$1,234.56": 123456,
		"(123.45)":  -12345,
		"123.45-":   -12345,
		"10.0":      1000,
	} {
		actual, err := parseQuickBooksCents(value)
		require.NoError(t, err, value)
		require.Equal(t, expected, actual, value)
	}
}

func TestGenerateQuickBooksJournalEndToEnd(t *testing.T) {
	prior := testEntry()
	prior.Date = "2022-01-02"
	priorText := prior.journalText("later-example")
	books, root := fixture(t, priorText)

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Messages []chatMessage `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Len(t, payload.Messages, 2)
		require.Contains(t, payload.Messages[1].Content, `"date":"2021-03-02"`)
		require.Contains(t, payload.Messages[1].Content, `"date":"2022-01-02"`)
		content := `{"candidates":[{"source_ids":["register.csv:2"],` +
			`"date":"2021-03-02","payee":"Electric company",` +
			`"description":"Power bill","reference":"44","confidence":"high","postings":[` +
			`{"account":"Expense:615:Utility","amount":"25.00"},` +
			`{"account":"Bank:131.2:Checking","amount":"-25.00"}]}],` +
			`"unresolved":[],"warnings":[]}`
		require.NoError(t, json.NewEncoder(writer).Encode(map[string]any{
			"message": map[string]string{"role": "assistant", "content": content},
		}))
	}))
	defer server.Close()
	books.Config.Statement = StatementConfig{APIBase: server.URL, Model: "ollama/test"}

	input := filepath.Join(root, "register.csv")
	require.NoError(t, os.WriteFile(input, []byte(
		"Date,Type,Num,Payee,Account,Payment,Deposit,Balance\n"+
			"3/2/2021,Check,44,Electric company,Power,25.00,,100.00\n",
	), 0600))
	var progress strings.Builder
	journal, err := books.GenerateQuickBooksJournal(
		context.Background(), []string{input}, 2021, &progress,
	)
	require.NoError(t, err)
	require.Contains(t, journal, "2021-03-02 Electric company | Power bill")
	require.Contains(t, journal, "Expense:615:Utility  $25.00")
	require.NotContains(t, journal, "register.csv")
	require.Contains(t, progress.String(), "Generated 1 balanced QuickBooks candidates")
	require.Equal(t, priorText, read(t, books.Config.ManualJournal),
		"candidate generation must not change the books")
}
