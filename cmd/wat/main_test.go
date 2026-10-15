package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHelpDoesNotRequireBooks(t *testing.T) {
	root := command()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetArgs([]string{"--config", "missing.yaml", "--help"})
	require.NoError(t, root.Execute())
	require.Contains(t, output.String(), "Water Accounting Terminal")
}

func TestStatementFlagAndCache(t *testing.T) {
	if _, err := exec.LookPath("hledger"); err != nil {
		t.Skip("requires hledger")
	}
	root := t.TempDir()
	config := filepath.Join(root, "wat.yaml")
	for name, text := range map[string]string{
		"wat.yaml":      "version: 1\nname: Test\nyear: 2021\njournal: books.journal\nwrite_journal: 2021.journal\nsources: sources\nbank_account: Assets:Bank\n",
		"books.journal": "commodity $\naccount Assets:Bank\naccount Equity:Capital\ninclude 2021.journal\n",
		"2021.journal":  "",
		"statement.pdf": "%PDF-1.7\nsynthetic\n",
		"extract.json":  `{"transactions":[{"booking_date":"2021-11-10","description":"DEPOSIT","amount":"100.00","currency":"USD"}],"verification":{"opening_balance":"0.00","closing_balance":"100.00"}}`,
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0600))
	}
	for i := 0; i < 2; i++ {
		cmd := command()
		args := []string{"--config", config, "--statement", filepath.Join(root, "statement.pdf"), "--statement-check"}
		if i == 0 {
			args = append(args, "--statement-extraction", filepath.Join(root, "extract.json"))
		}
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs(args)
		require.NoError(t, cmd.Execute())
		require.Contains(t, output.String(), "0 recorded, 1 pending")
	}
	content, err := os.ReadFile(filepath.Join(root, "2021.journal"))
	require.NoError(t, err)
	require.Empty(t, content, "ingesting/checking a statement must not create journal postings")
}

func TestCheckAndReports(t *testing.T) {
	if _, err := exec.LookPath("hledger"); err != nil {
		t.Skip("hledger integration tests require hledger on PATH")
	}
	for _, args := range [][]string{
		{"check"}, {"report", "trial-balance"}, {"report", "balance-sheet"}, {"report", "income-statement"}, {"report", "journal"},
	} {
		t.Run(args[len(args)-1], func(t *testing.T) {
			root := command()
			var output bytes.Buffer
			root.SetOut(&output)
			root.SetArgs(append([]string{"--config", filepath.Join("example", "wat.yaml")}, args...))
			require.NoError(t, root.Execute())
			require.NotEmpty(t, output.String())
		})
	}
}
