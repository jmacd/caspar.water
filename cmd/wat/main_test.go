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
		"wat.yaml":      "version: 2\nname: Test\nreport_year: 2021\njournal: books.journal\naccount_chart: chart.journal\nmanual_journal: 2021.journal\nsources: sources\nbank_accounts:\n  - account: Bank:131.2:Checking\n    journal: bank.journal\n",
		"books.journal": "include chart.journal\naccount Bank ; type: A\naccount Bank:131.2:Checking\naccount Equity:218:Owner\ninclude 2021.journal\ninclude bank.journal\n",
		"chart.journal": "commodity $\naccount Asset:131.2-Cash-in-Bank\naccount Equity:218-Proprietary-Capital\n",
		"2021.journal":  "",
		"bank.journal":  "",
		"statement.pdf": "%PDF-1.7\nsynthetic\n",
		"extract.json":  `{"transactions":[{"booking_date":"2021-11-10","description":"DEPOSIT","amount":"100.00","currency":"USD","account_id":"Checking"}],"verification":{"opening_balance":"0.00","closing_balance":"100.00"}}`,
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
	accounts := command()
	var accountsOutput bytes.Buffer
	accounts.SetOut(&accountsOutput)
	accounts.SetArgs([]string{"--config", config, "accounts"})
	require.NoError(t, accounts.Execute())
	require.Contains(t, accountsOutput.String(), "Bank:131.2:Checking\tAsset:131.2-Cash-in-Bank")
	override := command()
	override.SetOut(&bytes.Buffer{})
	override.SetArgs([]string{
		"--config", config,
		"--statement-inbox", filepath.Join(root, "missing-inbox"),
		"--statement", filepath.Join(root, "statement.pdf"),
		"--statement-check",
	})
	require.NoError(t, override.Execute(), "an explicit statement must override automatic inbox scanning")
	check := command()
	check.SetOut(&bytes.Buffer{})
	check.SetArgs([]string{
		"--config", config,
		"--statement-inbox", filepath.Join(root, "missing-inbox"),
		"check",
	})
	require.NoError(t, check.Execute(), "noninteractive subcommands must not mutate or scan the statement inbox")
	second := filepath.Join(root, "z-statement.pdf")
	require.NoError(t, os.WriteFile(second, []byte("%PDF-1.7\nsecond synthetic statement\n"), 0600))
	seed := command()
	seed.SetOut(&bytes.Buffer{})
	seed.SetArgs([]string{"--config", config, "--statement", second, "--statement-extraction", filepath.Join(root, "extract.json"), "--statement-check"})
	require.NoError(t, seed.Execute())
	batch := command()
	var output bytes.Buffer
	batch.SetOut(&output)
	batch.SetArgs([]string{"--config", config, "--statements", second, filepath.Join(root, "statement.pdf"), "--statement-check"})
	require.NoError(t, batch.Execute())
	require.Equal(t, 2, strings.Count(output.String(), "0 recorded, 1 pending"))
	require.Contains(t, output.String(), "Statement file: "+second)
	require.Contains(t, output.String(), "Statement file: "+filepath.Join(root, "statement.pdf"))
	require.Less(t, strings.Index(output.String(), "Statement file: "+second), strings.Index(output.String(), "Statement file: "+filepath.Join(root, "statement.pdf")), "preserve supplied order instead of sorting")
	for _, args := range [][]string{
		{"--statement", second, "--statements", second},
		{"--statements", second, "--statement-extraction", filepath.Join(root, "extract.json")},
		{"--statements"},
		{"--statements", ""},
		{"--statements", filepath.Join(root, "*.pdf"), "--statement-check"},
		{"--statement-inbox", root, "--statement-check"},
		{second},
	} {
		cmd := command()
		cmd.SetArgs(append([]string{"--config", config}, args...))
		require.Error(t, cmd.Execute())
	}
	content, err := os.ReadFile(filepath.Join(root, "2021.journal"))
	require.NoError(t, err)
	require.Empty(t, content, "ingesting/checking a statement must not create journal postings")
}

func TestStatementFiles(t *testing.T) {
	root := t.TempDir()
	names := []string{"2022-03.pdf", "2022-01.pdf", "2022 02.pdf"}
	for _, name := range names {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("%PDF-synthetic"), 0600))
	}
	t.Chdir(root)
	require.NoError(t, checkStatementFiles(names))
	require.Equal(t, []string{"2022-03.pdf", "2022-01.pdf", "2022 02.pdf"}, names)
	require.ErrorContains(t, checkStatementFiles([]string{"*.pdf"}), "statement file")
	require.ErrorContains(t, checkStatementFiles([]string{"missing.pdf"}), "statement file")
	require.NoError(t, os.Mkdir(filepath.Join(root, "directory.pdf"), 0700))
	require.ErrorContains(t, checkStatementFiles([]string{"directory.pdf"}), "not a regular PDF")
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), nil, 0600))
	require.ErrorContains(t, checkStatementFiles([]string{"notes.txt"}), "not a regular PDF")
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
