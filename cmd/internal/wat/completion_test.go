package wat

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatchingAccounts(t *testing.T) {
	bank := "Bank:131.2:Checking-2021"
	land := "Asset:121:Land-131"
	plant := "Asset:101:307-Wells"
	accounts := []string{bank, land, plant, "Asset:307:Well", "Expense:408:Tax"}
	for _, test := range []struct {
		query string
		want  []string
	}{
		{"13", []string{bank}},
		{"131.2", []string{bank}},
		{" 131 ", []string{bank}},
		{"121", []string{land}},
		{"101", []string{plant}},
		{"2", nil},
		{"2021", nil},
		{"307", []string{"Asset:307:Well"}},
		{"131-Cash", nil},
		{"131.3", nil},
		{"cash", nil},
		{"PROPERTY", nil},
		{"checking-2021", []string{bank}},
		{"well", []string{plant, "Asset:307:Well"}},
		{"missing", nil},
		{"", nil},
		{"  ", nil},
		{bank, nil},
	} {
		t.Run(test.query, func(t *testing.T) {
			require.Equal(t, test.want, matchingAccounts(accounts, test.query))
		})
	}
	require.Equal(t, []string{"131.2 Cash"}, matchingAccounts([]string{"131.2 Cash"}, "13"))
	designations := map[string]string{
		bank: "Asset 131.2 Cash in Bank",
		land: "Asset 121 Nonutility Property",
	}
	require.Equal(t, []string{bank}, matchingAccountsWithDesignations(accounts, designations, "cash"))
	require.Equal(t, []string{land}, matchingAccountsWithDesignations(accounts, designations, "property"))
}

func TestMatchingText(t *testing.T) {
	values := []string{"Electric company", "Water company", "City [red] Water"}
	require.Equal(t, values[:2], matchingText(values, "COMPANY"))
	require.Equal(t, []string{values[2]}, matchingText(values, " [red] "))
	require.Nil(t, matchingText(values, "Electric company"))
	require.Nil(t, matchingText(values, "missing"))
	require.Nil(t, matchingText(values, ""))
}

func TestCompletionHistory(t *testing.T) {
	books, _ := fixture(t, `2020-06-01 Old supplier | Meter purchase
    Bank:131.2:Checking  $-1.00
    Expense:615:Utility  $1.00

2021-01-01 Electric company | Power bill
    Bank:131.2:Checking  $-1.00
    Expense:615:Utility  $1.00

2021-02-01 Electric company | Power bill
    Bank:131.2:Checking  $-1.00
    Expense:615:Utility  $1.00

2021-03-01 Payee only
    Bank:131.2:Checking  $-1.00
    Expense:615:Utility  $1.00
`)
	draft, err := books.NewDraft()
	require.NoError(t, err)
	require.Equal(t, []string{"Electric company", "Old supplier", "Payee only"}, draft.history.payees)
	require.Equal(t, []string{"Meter purchase", "Power bill"}, draft.history.descriptions)

	entry := testEntry()
	entry.Payee, entry.Description = "New supplier", "New invoice"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	fresh, err := books.NewDraft()
	require.NoError(t, err)
	require.Contains(t, fresh.history.payees, entry.Payee)
	require.Contains(t, fresh.history.descriptions, entry.Description)
	require.NotContains(t, draft.history.payees, entry.Payee)
}

func TestCompletionHistoryErrors(t *testing.T) {
	books, root := fixture(t, "")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	require.Empty(t, draft.history.payees)
	require.Empty(t, draft.history.descriptions)
	books.binary = filepath.Join(root, "missing-hledger")
	_, err = books.completionHistory()
	require.ErrorContains(t, err, "load transaction completion history")
	fake := filepath.Join(root, "fake-hledger")
	books.binary = fake
	for _, output := range []string{"invalid JSON", "null", "{}"} {
		require.NoError(t, os.WriteFile(fake, []byte("#!/bin/sh\nprintf '"+output+"'\n"), 0700))
		_, err = books.completionHistory()
		require.ErrorContains(t, err, "decode hledger transactions")
	}
}
