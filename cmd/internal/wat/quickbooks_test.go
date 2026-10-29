package wat

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseQuickBooksCSVReport(t *testing.T) {
	data := []byte(`Joshua MacDonald and Ariana Bayer,,,,,
Account QuickReport,,,,,
All Dates,,,,,
,,,,,
,Distribution account,Transaction date,Name,Amount,Balance
Water Company Yearly,,,,,
Insurance,,,,,
,Bank Account,12/31/2020,Old transaction,-10.00,50.00
,Bank Account,01/15/2021,Vendor A,-12.34,37.66
,Bank Account,2021-02-01T00:00:00.000,Vendor B,50.00,87.66
,Bank Account,01/02/2022,New transaction,-5.00,82.66
`)
	records, err := parseQuickBooksCSV("report.xlsx", data, 2021)
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "2021-01-15", records[0].Date)
	require.Equal(t, "Insurance", records[0].Section)
	require.Equal(t, "-12.34", records[0].Fields["Amount"])
	require.Equal(t, "report.xlsx:9", records[0].ID)
	require.Equal(t, "2021-02-01", records[1].Date)
}

func TestParseQuickBooksCSVRegister(t *testing.T) {
	data := []byte("Date,Type,Num,Payee,Account,Payment,Deposit,Balance\n" +
		"3/2/2021,Check,44,Vendor,Insurance,25.00,,100.00\n")
	records, err := parseQuickBooksCSV("register.xls", data, 2021)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "2021-03-02", records[0].Date)
	require.Equal(t, "25.00", records[0].Fields["Payment"])
}

func TestInspectQuickBooksReportsEveryUndatedNonemptyRow(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "register.csv")
	require.NoError(t, os.WriteFile(input, []byte(
		"QuickBooks register,,,,,,,\n"+
			"Date,Type,Num,Payee,Account,Payment,Deposit,Balance\n"+
			"12/31/2020,Check,40,Old vendor,Repairs,10.00,,100.00\n"+
			"3/2/2021,Check,44,Vendor,Insurance,25.00,,75.00\n"+
			",Split,,,Professional services,5.00,,\n"+
			",,,Register subtotal,,,,\n"+
			",,,,,,,\n"+
			"1/2/2022,Deposit,50,Customer,Income,,100.00,175.00\n",
	), 0600))

	var output strings.Builder
	err := (&Books{}).InspectQuickBooks(
		context.Background(),
		[]string{input},
		2021,
		&output,
		io.Discard,
	)
	require.NoError(t, err)
	require.Contains(t, output.String(), "normalized CSV: 8 rows, 7 nonempty, 8 columns")
	require.Contains(t, output.String(), "before header: 1 rows, 1 nonempty")
	require.Contains(
		t,
		output.String(),
		"data rows: 6 total, 3 dated, 1 section, 0 total, 0 footer, 1 undated nonempty, 1 blank",
	)
	require.Contains(t, output.String(), "date span: 2020-12-31 through 2022-01-02")
	require.Contains(t, output.String(), "dated rows by year: 2020=1 2021=1 2022=1")
	require.Contains(t, output.String(), "selected year 2021: 1 transaction rows")
	require.Contains(
		t,
		output.String(),
		`undated nonempty row 5: Type="Split" | Account="Professional services" | Payment="5.00"`,
	)
	require.Contains(t, output.String(), `section row 6: "Register subtotal"`)
	require.Contains(t, output.String(), `selected-year transaction types: "Check"=1`)
	require.Contains(t, output.String(), "content sha256:")
}

func TestParseQuickBooksCSVClassifiesReportTotalsAndFooter(t *testing.T) {
	parsed, err := parseQuickBooksCSVRows("report.csv", []byte(
		"column_1,Distribution account,Transaction date,Transaction type,Amount\n"+
			"Insurance,,,,\n"+
			",Bank,1/2/2021,Expense,10.00\n"+
			"Total for Insurance,,,,10.00\n"+
			"TOTAL,,,,10.00\n"+
			"\"Monday, October 19, 2026 10:01 PM GMT-07:00\",,,,\n",
	), 2021)
	require.NoError(t, err)
	require.Len(t, parsed.Records, 1)
	require.Len(t, parsed.Sections, 1)
	require.Len(t, parsed.SummaryRows, 2)
	require.Len(t, parsed.FooterRows, 1)
	require.Empty(t, parsed.UndatedRows)
}

func TestParseQuickBooksCSVRejectsUnknownTransactionDate(t *testing.T) {
	_, err := parseQuickBooksCSV(
		"report.csv",
		[]byte("Date,Amount,Balance\nnot-a-date,1.00,1.00\n"),
		2021,
	)
	require.ErrorContains(t, err, `unsupported date "not-a-date"`)
}

func TestQuickBooksHeaderRequiresDateAndMoney(t *testing.T) {
	_, err := quickBooksHeaderRow([][]string{{"Name", "Description"}})
	require.ErrorContains(t, err, "could not find a QuickBooks header")
}

func TestQuickBooksSpreadsheetUsesPondCSV(t *testing.T) {
	root := t.TempDir()
	input := filepath.Join(root, "Account QuickReport (1).xlsx")
	require.NoError(t, os.WriteFile(input, []byte("synthetic"), 0600))
	argsFile := filepath.Join(root, "args")
	pond := filepath.Join(root, "pond")
	require.NoError(t, os.WriteFile(pond, []byte(`#!/bin/sh
printf '%s\n' "$@" >"$WAT_POND_ARGS"
printf '%s\n' 'Transaction date,Amount,Balance' '1/2/2021,-12.34,100.00'
`), 0700))
	t.Setenv("WAT_POND", pond)
	t.Setenv("WAT_POND_ARGS", argsFile)

	var progress strings.Builder
	records, err := (&Books{}).quickBooksRecords(
		context.Background(), []string{input}, 2021, &progress,
	)
	require.NoError(t, err)
	require.Len(t, records, 1)
	args := read(t, argsFile)
	require.Contains(t, args, "host+xls:///")
	require.Contains(t, args, "Account%20QuickReport%20%281%29.xlsx")
	require.Contains(t, args, "--format=csv")
}
