package wat

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupQuickBooksRecordsMatchesReferenceAcrossReports(t *testing.T) {
	groups, err := groupQuickBooksRecords([]quickBooksRecord{
		{
			ID: "register:2", Source: "register.xls", Date: "2021-12-15",
			CashSource: true,
			Fields: map[string]string{
				"Ref No.": "1001", "Payee": "Feiner Fixings",
				"Payment": "240.00", "Type": "Check",
			},
		},
		{
			ID: "report:8", Source: "report.xlsx", Date: "2021-12-15",
			Fields: map[string]string{
				"Num": "1001", "Name": "Feiner Fixings",
				"Amount": "240.00", "Transaction type": "Check",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "reference", groups[0].MatchBasis)
	require.False(t, groups[0].Ambiguous)
	require.False(t, groups[0].SourceAmountsMayOverlap)
	require.Len(t, groups[0].Records, 2)
}

func TestGroupQuickBooksRecordsMatchesDescriptionAcrossReports(t *testing.T) {
	groups, err := groupQuickBooksRecords([]quickBooksRecord{
		{
			ID: "register:2", Source: "register.xls", Date: "2021-12-23",
			CashSource: true,
			Fields: map[string]string{
				"Payee": "PG&E", "Memo": "Online payment",
				"Payment": "43.95", "Type": "Expense",
			},
		},
		{
			ID: "report:8", Source: "report.xlsx", Date: "2021-12-23",
			Fields: map[string]string{
				"Name": "PG&E", "Description": "Online  payment",
				"Amount": "43.95", "Transaction type": "Expense",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "description", groups[0].MatchBasis)
	require.False(t, groups[0].Ambiguous)
}

func TestGroupQuickBooksRecordsMatchesSplitLinesByReference(t *testing.T) {
	groups, err := groupQuickBooksRecords([]quickBooksRecord{
		{
			ID: "expenses:2", Source: "expenses.xlsx", Date: "2021-07-30",
			Fields: map[string]string{
				"Num": "12", "Name": "Owner labor", "Amount": "400.00",
				"Transaction type": "Journal",
			},
		},
		{
			ID: "income:2", Source: "income.xlsx", Date: "2021-07-30",
			Fields: map[string]string{
				"Num": "12", "Name": "Invested labor", "Amount": "-300.00",
				"Transaction type": "Journal",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "reference", groups[0].MatchBasis)
	require.False(t, groups[0].Ambiguous)
	require.True(t, groups[0].SourceAmountsMayOverlap)
	require.Len(t, groups[0].Records, 2)
}

func TestGroupQuickBooksRecordsDoesNotMatchDateAndAmountAlone(t *testing.T) {
	groups, err := groupQuickBooksRecords([]quickBooksRecord{
		{
			ID: "one:2", Source: "one.xlsx", Date: "2021-01-02",
			Fields: map[string]string{"Name": "Vendor A", "Amount": "10.00"},
		},
		{
			ID: "two:2", Source: "two.xlsx", Date: "2021-01-02",
			Fields: map[string]string{"Name": "Vendor B", "Amount": "10.00"},
		},
	})
	require.NoError(t, err)
	require.Len(t, groups, 2)
	for _, group := range groups {
		require.Equal(t, "single", group.MatchBasis)
		require.False(t, group.Ambiguous)
	}
}

func TestGroupQuickBooksRecordsMarksSameSourceCollisionAmbiguous(t *testing.T) {
	groups, err := groupQuickBooksRecords([]quickBooksRecord{
		{
			ID: "report:2", Source: "report.xlsx", Date: "2021-01-02",
			Fields: map[string]string{
				"Num": "10", "Name": "Vendor", "Amount": "10.00",
			},
		},
		{
			ID: "report:3", Source: "report.xlsx", Date: "2021-01-02",
			Fields: map[string]string{
				"Num": "10", "Name": "Vendor", "Amount": "10.00",
			},
		},
	})
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.True(t, groups[0].Ambiguous)
	require.Equal(t, "reference", groups[0].MatchBasis)
}

func TestGroupQuickBooksRecordsRejectsDuplicateSourceID(t *testing.T) {
	record := quickBooksRecord{
		ID: "report:2", Source: "report.xlsx", Date: "2021-01-02",
		Fields: map[string]string{"Amount": "10.00"},
	}
	_, err := groupQuickBooksRecords([]quickBooksRecord{record, record})
	require.ErrorContains(t, err, `duplicate QuickBooks source id "report:2"`)
}

func TestGroupQuickBooksRecordsRejectsInvalidDate(t *testing.T) {
	_, err := groupQuickBooksRecords([]quickBooksRecord{{
		ID: "report:2", Source: "report.xlsx", Date: "01/02/2021",
		Fields: map[string]string{"Amount": "10.00"},
	}})
	require.ErrorContains(t, err, `invalid normalized date "01/02/2021"`)
}
