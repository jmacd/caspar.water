package wat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testQuickBooksGroup(id string, records ...quickBooksRecord) quickBooksTransactionGroup {
	return quickBooksTransactionGroup{
		ID: id, Date: records[0].Date, MatchBasis: "single", Records: records,
	}
}

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
		content := `{"candidates":[{"group_id":"group-1","date":"2021-01-02","payee":"Vendor",` +
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
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "source:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.NoError(t, err)
	require.Len(t, response.Candidates, 1)
	text, err := renderQuickBooksCandidates(response, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "source:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.NoError(t, err)
	require.Contains(t, text, "2021-01-02 Vendor")
	require.Contains(t, text, "Expense:615:Electricity  $10.00")
	require.NotContains(t, text, "source:")
}

func TestQuickBooksCandidatesRejectUnbalancedModelOutput(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "low",
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
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "source:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(t, err, "postings do not balance")
}

func TestQuickBooksCandidatesRejectUnknownAccount(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "low",
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
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "source:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(t, err, "unconfigured account")
}

func TestQuickBooksCandidatesRequireHumanApprovedMapping(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "high",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:664:Maintenance", Amount: "10.00"},
				{Account: "Equity:Owner", Amount: "-10.00"},
			},
		}},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Utility", "Expense:664:Maintenance", "Equity:Owner",
		},
		AccountMappings: []quickBooksAccountMapping{{
			Label: "Utilities", Account: "Expense:615:Utility",
		}},
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "report:2", Date: "2021-01-02", Section: "Utilities",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(t, err, `omits human-approved mapped account "Expense:615:Utility"`)
}

func TestQuickBooksCandidatesRejectCanonicalDuplicate(t *testing.T) {
	candidate := quickBooksAICandidate{
		GroupID: "group-1",
		Date:    "2021-01-02", Payee: "Vendor", Confidence: "medium",
		Postings: []quickBooksAIPosting{
			{Account: "Expense:615:Electricity", Amount: "10.0"},
			{Account: "Bank:131.2:SBMC:0000", Amount: "-10.00"},
		},
	}
	reversed := candidate
	reversed.GroupID = "group-2"
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
		Groups: []quickBooksTransactionGroup{
			testQuickBooksGroup(
				"group-1",
				quickBooksRecord{
					ID: "source:2", Date: "2021-01-02",
					Fields: map[string]string{"Amount": "10.00"},
				},
			),
			testQuickBooksGroup(
				"group-2",
				quickBooksRecord{
					ID: "source:3", Date: "2021-01-02",
					Fields: map[string]string{"Amount": "10.00"},
				},
			),
		},
	})
	require.ErrorContains(t, err, "duplicates another generated transaction")
}

func TestQuickBooksCandidatesRejectAmountAbsentFromSource(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "medium",
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
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "register:2", Date: "2021-01-02",
				Fields: map[string]string{"Payment": "$10.00"},
			},
		)},
	})
	require.ErrorContains(t, err, "total 11.00 is not present")
}

func TestQuickBooksCandidatesRejectOmittedSplitAmount(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "medium",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:615:Electricity", Amount: "10.00"},
				{Account: "Equity:Owner", Amount: "-10.00"},
			},
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year:               2021,
		Month:              "2021-01",
		ConfiguredAccounts: []string{"Expense:615:Electricity", "Equity:Owner"},
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "expense:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
			quickBooksRecord{
				ID: "income:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "-4.00"},
			},
		)},
	})
	require.ErrorContains(t, err, "omits cited QuickBooks transaction amount 4.00")
}

func TestQuickBooksCandidatesAcceptCompleteSplitAmounts(t *testing.T) {
	text, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "medium",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:One", Amount: "60.00"},
				{Account: "Expense:Two", Amount: "40.00"},
				{Account: "Equity:Owner", Amount: "-100.00"},
			},
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{"Expense:One", "Expense:Two", "Equity:Owner"},
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "expense-one:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "60.00"},
			},
			quickBooksRecord{
				ID: "expense-two:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "40.00"},
			},
		)},
	})
	require.NoError(t, err)
	require.Contains(t, text, "Expense:One  $60.00")
	require.Contains(t, text, "Expense:Two  $40.00")
	require.Contains(t, text, "Equity:Owner  $-100.00")
}

func TestQuickBooksUnresolvedRendersCommentedSkeleton(t *testing.T) {
	text, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{{
			GroupID:     "group-1",
			Date:        "2021-01-02",
			Payee:       "Professional firm",
			Description: "Business service",
			KnownPostings: []quickBooksAIKnownPosting{{
				QuickBooksAccount: "Professional services",
				Amount:            "10.00",
			}},
			MissingFact: "The personal cash-side record is unavailable.",
			Question:    "Was this due to the owner or contributed capital?",
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "report:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
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

func TestQuickBooksUnresolvedPreservesSplitPostings(t *testing.T) {
	text, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{{
			GroupID: "group-1", Date: "2021-07-30",
			Description: "Incomplete journal entry",
			KnownPostings: []quickBooksAIKnownPosting{
				{QuickBooksAccount: "Maintenance", Amount: "499.00"},
				{QuickBooksAccount: "Invested labor", Amount: "-299.00"},
			},
			MissingFact: "The remaining journal line is unavailable.",
			Question:    "What account received the remaining 200.00 credit?",
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-07",
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "expense:2", Date: "2021-07-30",
				Fields: map[string]string{"Amount": "499.00"},
			},
			quickBooksRecord{
				ID: "income:2", Date: "2021-07-30",
				Fields: map[string]string{"Amount": "-299.00"},
			},
		)},
	})
	require.NoError(t, err)
	require.Contains(t, text, "; Sources: expense:2, income:2")
	require.Contains(t, text, ";     [QuickBooks: Maintenance]  $499.00")
	require.Contains(t, text, ";     [QuickBooks: Invested labor]  $-299.00")
	require.Contains(t, text, ";     [TODO counterposting]  $-200.00")
}

func TestQuickBooksCandidatesRequireDispositionForEveryGroup(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "report:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(
		t,
		err,
		`transaction group "group-1" has no candidate or unresolved disposition`,
	)
}

func TestQuickBooksCandidatesRejectUnknownGroup(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{{
			GroupID: "unknown", Date: "2021-01-02",
			KnownPostings: []quickBooksAIKnownPosting{{
				QuickBooksAccount: "Electricity", Amount: "10.00",
			}},
			MissingFact: "Missing cash side", Question: "Which account paid?",
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "report:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(t, err, `cites unknown transaction group "unknown"`)
}

func TestQuickBooksCandidatesRejectMissingGroupID(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			Date: "2021-01-02", Payee: "Vendor", Confidence: "high",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:Electricity", Amount: "10.00"},
				{Account: "Equity:Owner", Amount: "-10.00"},
			},
		}},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year:               2021,
		Month:              "2021-01",
		ConfiguredAccounts: []string{"Expense:Electricity", "Equity:Owner"},
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "report:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(t, err, "cites no transaction group")
}

func TestQuickBooksCandidatesRejectGroupDateMismatch(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		Groups: []quickBooksTransactionGroup{{
			ID:   "group-1",
			Date: "2021-01-02",
			Records: []quickBooksRecord{{
				ID: "report:2", Date: "2021-01-03",
				Fields: map[string]string{"Amount": "10.00"},
			}},
		}},
	})
	require.ErrorContains(t, err, "disagrees with source")
}

func TestQuickBooksCandidatesRejectSourceReusedAcrossGroups(t *testing.T) {
	record := quickBooksRecord{
		ID: "report:2", Date: "2021-01-02",
		Fields: map[string]string{"Amount": "10.00"},
	}
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		Groups: []quickBooksTransactionGroup{
			testQuickBooksGroup("group-1", record),
			testQuickBooksGroup("group-2", record),
		},
	})
	require.ErrorContains(t, err, `reuses source id "report:2" across groups`)
}

func TestQuickBooksCandidatesRejectAmbiguousGroup(t *testing.T) {
	group := testQuickBooksGroup(
		"group-1",
		quickBooksRecord{
			ID: "report:2", Date: "2021-01-02",
			Fields: map[string]string{"Amount": "10.00"},
		},
	)
	group.Ambiguous = true
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "high",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:615:Electricity", Amount: "10.00"},
				{Account: "Equity:Owner", Amount: "-10.00"},
			},
		}},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year:               2021,
		Month:              "2021-01",
		ConfiguredAccounts: []string{"Expense:615:Electricity", "Equity:Owner"},
		Groups:             []quickBooksTransactionGroup{group},
	})
	require.ErrorContains(t, err, "ambiguous transaction group")
}

func TestQuickBooksCandidatesRejectReusedGroup(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "high",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:615:Electricity", Amount: "10.00"},
				{Account: "Bank:131.2:SBMC:0000", Amount: "-10.00"},
			},
		}},
		Unresolved: []quickBooksAIUnresolved{{
			GroupID: "group-1", Date: "2021-01-02",
			KnownPostings: []quickBooksAIKnownPosting{{
				QuickBooksAccount: "Electricity", Amount: "10.00",
			}},
			MissingFact: "Missing cash side", Question: "Which account paid?",
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "report:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(t, err, `reuses transaction group "group-1"`)
}

func TestQuickBooksCandidatesRejectBankWithoutCashEvidence(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{{
			GroupID: "group-1",
			Date:    "2021-01-02", Payee: "Vendor", Confidence: "high",
			Postings: []quickBooksAIPosting{
				{Account: "Expense:615:Electricity", Amount: "10.00"},
				{Account: "Bank:131.2:SBMC:0000", Amount: "-10.00"},
			},
		}},
		Unresolved: []quickBooksAIUnresolved{},
		Warnings:   []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
		BankAccounts: []string{"Bank:131.2:SBMC:0000"},
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "report:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(
		t,
		err,
		`posts to bank account "Bank:131.2:SBMC:0000" without a cited cash-source row`,
	)
}

func TestQuickBooksUnresolvedRejectsBankWithoutCashEvidence(t *testing.T) {
	_, err := renderQuickBooksCandidates(quickBooksAIResponse{
		Candidates: []quickBooksAICandidate{},
		Unresolved: []quickBooksAIUnresolved{{
			GroupID: "group-1", Date: "2021-01-02",
			KnownPostings: []quickBooksAIKnownPosting{{
				Account: "Bank:131.2:SBMC:0000", Amount: "-10.00",
			}},
			MissingFact: "Missing expense account", Question: "Which expense account?",
		}},
		Warnings: []string{},
	}, quickBooksAIInput{
		Year: 2021, Month: "2021-01",
		ConfiguredAccounts: []string{
			"Expense:615:Electricity", "Bank:131.2:SBMC:0000",
		},
		BankAccounts: []string{"Bank:131.2:SBMC:0000"},
		Groups: []quickBooksTransactionGroup{testQuickBooksGroup(
			"group-1",
			quickBooksRecord{
				ID: "report:2", Date: "2021-01-02",
				Fields: map[string]string{"Amount": "10.00"},
			},
		)},
	})
	require.ErrorContains(
		t,
		err,
		`presents bank account "Bank:131.2:SBMC:0000" without a cited cash-source row`,
	)
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
	require.NoError(t, books.saveQuickBooksMappings([]quickBooksAccountMapping{{
		Label: "Power", Account: "Expense:615:Utility",
	}}))

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Messages []chatMessage `json:"messages"`
		}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&payload))
		require.Len(t, payload.Messages, 2)
		require.Contains(t, payload.Messages[1].Content, `"date":"2021-03-02"`)
		require.Contains(t, payload.Messages[1].Content, `"date":"2022-01-02"`)
		var modelInput quickBooksAIInput
		require.NoError(t, json.Unmarshal([]byte(payload.Messages[1].Content), &modelInput))
		require.Len(t, modelInput.Groups, 1)
		require.Equal(t, []quickBooksAccountMapping{{
			Label: "Power", Account: "Expense:615:Utility",
		}}, modelInput.AccountMappings)
		content := fmt.Sprintf(`{"candidates":[{"group_id":%q,`,
			modelInput.Groups[0].ID) +
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
