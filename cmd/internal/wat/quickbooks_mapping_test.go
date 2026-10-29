package wat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

func TestQuickBooksMappingsRoundTrip(t *testing.T) {
	books, _ := fixture(t, "")
	require.Empty(t, mustQuickBooksMappings(t, books))
	require.NoError(t, books.saveQuickBooksMappings([]quickBooksAccountMapping{
		{Label: "Utilities", Account: "Expense:615:Utility"},
		{Label: "Checking", Account: "Bank:131.2:Checking"},
	}))
	mappings := mustQuickBooksMappings(t, books)
	require.Equal(t, []quickBooksAccountMapping{
		{Label: "Checking", Account: "Bank:131.2:Checking"},
		{Label: "Utilities", Account: "Expense:615:Utility"},
	}, mappings)
	info, err := os.Stat(books.Config.QuickBooksMappings)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	require.Contains(t, read(t, books.Config.QuickBooksMappings), "version: 1")
}

func TestQuickBooksMappingsRejectInvalidPolicy(t *testing.T) {
	books, root := fixture(t, "")
	require.ErrorContains(
		t,
		books.saveQuickBooksMappings([]quickBooksAccountMapping{{
			Label: "Insurance", Account: "Expense:684:Insurance",
		}}),
		"unconfigured account",
	)
	require.NoError(t, os.WriteFile(
		books.Config.QuickBooksMappings,
		[]byte("version: 1\nmappings:\n  - label: Insurance\n    account: Expense:684:Insurance\n"),
		0600,
	))
	mappings := mustQuickBooksMappings(t, books)
	require.Equal(t, "Expense:684:Insurance", mappings[0].Account)
	review, err := books.quickBooksMappingReview(nil, 2021)
	require.NoError(t, err)
	require.Equal(t, []string{"Insurance"}, review.labels)
	require.Equal(t, "not present in selected-year sources", review.details["Insurance"])
	require.ErrorContains(t, books.saveQuickBooksMappings(mappings), "unconfigured account")
	require.NoError(t, os.Remove(books.Config.QuickBooksMappings))
	target := filepath.Join(root, "mapping-target.yaml")
	require.NoError(t, os.WriteFile(target, []byte("version: 1\nmappings: []\n"), 0600))
	require.NoError(t, os.Symlink(target, books.Config.QuickBooksMappings))
	_, err = books.quickBooksMappings()
	require.ErrorContains(t, err, "nonsymlink regular file")
}

func TestQuickBooksMappingReviewInventoriesLabels(t *testing.T) {
	books, _ := fixture(t, "")
	review, err := books.quickBooksMappingReview([]quickBooksRecord{{
		ID: "report:2", Source: "report.xlsx", Date: "2021-01-02",
		Section: "Utilities",
		Fields: map[string]string{
			"Account Name":         "Water Company Expenses:Utilities",
			"Distribution account": "Utilities",
			"Amount":               "10.00",
		},
	}}, 2021)
	require.NoError(t, err)
	require.Equal(t, []string{"Utilities"}, review.labels)
	require.Equal(
		t,
		"report section=1",
		review.details["Utilities"],
	)
	require.False(t, review.dirty())
	review.mappings["Utilities"] = "Expense:615:Utility"
	require.True(t, review.dirty())
	require.Equal(t, []quickBooksAccountMapping{{
		Label: "Utilities", Account: "Expense:615:Utility",
	}}, review.savedMappings())
}

func TestQuickBooksMappingTerminalShowsUnresolvedLabels(t *testing.T) {
	books, _ := fixture(t, "")
	review, err := books.quickBooksMappingReview([]quickBooksRecord{{
		ID: "report:2", Source: "report.xlsx", Date: "2021-01-02",
		Section: "Utilities",
		Fields:  map[string]string{"Amount": "10.00"},
	}}, 2021)
	require.NoError(t, err)
	ui := &terminal{app: tview.NewApplication(), books: books}
	ui.quickBooksMappings(review)
	list, ok := ui.app.GetFocus().(*tview.List)
	require.True(t, ok)
	require.Equal(t, 3, list.GetItemCount())
	label, status := list.GetItemText(0)
	require.Equal(t, "Utilities", label)
	require.Contains(t, status, "UNRESOLVED")
}

func TestQuickBooksMappedAccountsUsesOneDecisionLabelPerRecord(t *testing.T) {
	group := testQuickBooksGroup(
		"group-1",
		quickBooksRecord{
			ID: "report:2", Date: "2021-01-02", Section: "Utilities",
			Fields: map[string]string{"Amount": "10.00"},
		},
		quickBooksRecord{
			ID: "register:2", Date: "2021-01-02",
			Fields: map[string]string{"Account": "Checking", "Payment": "10.00"},
		},
	)
	accounts, err := quickBooksMappedAccounts(group, []quickBooksAccountMapping{
		{Label: "Utilities", Account: "Expense:615:Utility"},
		{Label: "Checking", Account: "Bank:131.2:Checking"},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]bool{
		"Expense:615:Utility": true,
		"Bank:131.2:Checking": true,
	}, accounts)
}

func mustQuickBooksMappings(t *testing.T, books *Books) []quickBooksAccountMapping {
	t.Helper()
	mappings, err := books.quickBooksMappings()
	require.NoError(t, err)
	return mappings
}
