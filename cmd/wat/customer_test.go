package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomerCommandsDirectFileMilestone(t *testing.T) {
	if _, err := exec.LookPath("hledger"); err != nil {
		t.Skip("requires hledger")
	}
	root := t.TempDir()
	configPath := filepath.Join(root, "wat.yaml")
	require.NoError(t, os.Mkdir(filepath.Join(root, "reference"), 0700))
	config := `version: 3
name: Test utility
chart_defs: [reference/chart.yaml]
accounts:
  - Equity:218.000:Owner
  - Income:460.100:Service
bank_accounts:
  - Bank:131.200:Checking
`
	chart := strings.Replace(testAccountChart, "  Equity:", `    - number: "141"
      name: Customer-Receivables
  Equity:`, 1) + `  Income:
    - number: "460.1"
      name: Unmetered-Units
`
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "reference", "chart.yaml"), []byte(chart), 0600))
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := command()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"--config", configPath}, args...))
		err := cmd.Execute()
		return output.String(), err
	}
	output, err := run("customer", "init")
	require.NoError(t, err)
	require.Contains(t, output, "customer_registry: true")
	require.NoDirExists(t, filepath.Join(root, "books"), "registry initialization must not prepare or change journals")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Equal(t, config, string(data))
	_, err = run("customer", "init")
	require.ErrorContains(t, err, "will not overwrite")
	require.ErrorContains(t, err, "Set customer_registry: true")
	require.NoError(t, os.WriteFile(configPath, []byte(config+"customer_registry: true\n"), 0600))
	_, err = run("customer", "init")
	require.ErrorIs(t, err, os.ErrExist)
	require.ErrorContains(t, err, "Open 'wat customer' with the same --config")
	require.NotContains(t, err.Error(), "Set customer_registry: true")
	require.NoDirExists(t, filepath.Join(root, "books"), "reinitializing an enabled registry must not open or modify the books")
	for _, args := range [][]string{
		{"payee", "add", "Deposit"},
		{"customer", "add", "--payee", "Example Customer", "--address", "100 Example Street", "--address", "Exampletown, CA"},
		{"connection", "add", "--name", "House", "--address", "100 Example Street"},
		{"service", "open", "--customer", "C-0001", "--connection", "House", "--from", "2021-01-01"},
	} {
		output, err := run(args...)
		require.NoError(t, err, "%v: %s", args, output)
	}
	manual := filepath.Join(root, "books", "manual.journal")
	data, err = os.ReadFile(manual)
	require.NoError(t, err)
	require.Empty(t, data, "setup must not fabricate money")
	payeesPath := filepath.Join(root, "books", "payees.journal")
	manualPayees, err := os.ReadFile(payeesPath)
	require.NoError(t, err)
	require.NotContains(t, string(manualPayees), "Example Customer")
	customerPayees, err := os.ReadFile(filepath.Join(root, "books", "customers.journal"))
	require.NoError(t, err)
	require.Contains(t, string(customerPayees), "payee Example Customer\n")
	for _, args := range [][]string{
		{"customer", "list"}, {"connection", "list"}, {"service", "list"},
		{"customer", "show", "Example Customer", "--as-of", "2021-02-01"},
		{"connection", "show", "CN-0001", "--as-of", "2021-02-01"},
		{"service", "statement", "SA-0001", "--as-of", "2021-02-01"},
		{"check"},
	} {
		output, err := run(args...)
		require.NoError(t, err, "%v: %s", args, output)
		require.NotEmpty(t, output)
	}
	original := `2021-01-15 Example Customer | Service charge
    ; id: charge-1
    Asset:141.000:Customers:SA-0001  $150.00
    Income:460.100:Service  $-150.00

2021-02-01 Deposit | Customer payment
    ; id: receipt-1
    Bank:131.200:Checking  $100.00
    Equity:218.000:Owner  $-100.00
`
	require.NoError(t, os.WriteFile(manual, []byte(original), 0600))
	output, err = run("service", "allocate", "receipt-1", "--to", "SA-0001=100")
	require.NoError(t, err)
	require.Contains(t, output, "Preview only")
	data, err = os.ReadFile(manual)
	require.NoError(t, err)
	require.Equal(t, original, string(data))
	_, err = run("service", "allocate", "receipt-1", "--to", "SA-0001=100", "--save")
	require.NoError(t, err)
	data, err = os.ReadFile(manual)
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(string(data), "id: receipt-1"))
	require.Equal(t, 1, strings.Count(string(data), "Bank:131.200:Checking"))
	output, err = run("service", "statement", "SA-0001", "--from", "2021-02-01", "--as-of", "2021-02-28")
	require.NoError(t, err)
	require.Contains(t, output, "Opening balance: $150.00")
	require.Contains(t, output, "Closing balance: $50.00")
	_, err = run("customer", "edit", "C-0001", "--address", "New mailing address")
	require.NoError(t, err)
	output, err = run("customer", "show", "C-0001", "--as-of", "2021-02-28")
	require.NoError(t, err)
	require.Contains(t, output, "New mailing address")
	require.Contains(t, output, "Closing balance: $50.00")
	_, err = run("service", "open", "--customer", "C-0001", "--connection", "CN-0001", "--from", "2021-02-01")
	require.ErrorContains(t, err, "overlaps")
	_, err = run("customer", "add", "--payee", "Undeclared", "--address", "Somewhere")
	require.NoError(t, err, "customer names are declared automatically from the registry")
	data, err = os.ReadFile(payeesPath)
	require.NoError(t, err)
	require.Equal(t, manualPayees, data)

	_, err = run("service", "close", "SA-0001", "--through", "2021-01-31")
	require.NoError(t, err)
	output, err = run("service", "settle", "SA-0001", "--as-of", "2021-02-28")
	require.NoError(t, err)
	require.Contains(t, output, "NOT SETTLED")
	require.Contains(t, output, "not been confirmed")
	_, err = run("service", "statement", "SA-0001", "--as-of", "2021-02-28", "--issue")
	require.ErrorContains(t, err, "cannot issue")
	output, err = run("service", "adjust", "SA-0001", "--date", "2021-02-28", "--through", "2021-01-31",
		"--amount", "-50", "--account", "Income:460.100:Service", "--reference", "Reviewed final service credit")
	require.NoError(t, err)
	require.Contains(t, output, "Preview only")
	_, err = run("service", "adjust", "SA-0001", "--date", "2021-02-28", "--through", "2021-01-31",
		"--amount", "-50", "--account", "Income:460.100:Service", "--reference", "Reviewed final service credit", "--save")
	require.NoError(t, err)
	_, err = run("service", "confirm-billing", "SA-0001", "--through", "2021-01-31",
		"--date", "2021-02-28", "--review", "Reviewed all service and final credit; no further unbilled service")
	require.NoError(t, err)
	output, err = run("service", "settle", "SA-0001", "--as-of", "2021-02-28")
	require.NoError(t, err)
	require.Contains(t, output, "SA-0001 as of 2021-02-28: SETTLED")
	output, err = run("service", "statement", "SA-0001", "--as-of", "2021-02-28", "--issue")
	require.NoError(t, err)
	require.Contains(t, output, "statement-0001.txt")
}
