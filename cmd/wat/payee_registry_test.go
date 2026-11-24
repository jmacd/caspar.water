package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPayeeRegistryCommandsAndCanonicalHistory(t *testing.T) {
	if _, err := exec.LookPath("hledger"); err != nil {
		t.Skip("requires hledger")
	}
	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "books"), 0700))
	config := "version: 3\nname: Test\nchart_defs: [chart.yaml]\naccounts: [Equity:218.000:Owner]\nbank_accounts: [Bank:131.200:Checking]\n"
	journal := "2021-01-01 POWER ONLINE | Original payment\n    Bank:131.200:Checking  $-10\n    Equity:218.000:Owner  $10\n"
	for name, text := range map[string]string{
		"wat.yaml": config, "chart.yaml": testAccountChart,
		"books/manual.journal": journal, "books/payees.journal": "payee Power company\n",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(text), 0600))
	}
	run := func(args ...string) (string, error) {
		t.Helper()
		cmd := command()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(append([]string{"--config", filepath.Join(root, "wat.yaml")}, args...))
		err := cmd.Execute()
		return output.String(), err
	}
	_, err := run("payee", "init")
	require.NoError(t, err)
	data, err := os.ReadFile(filepath.Join(root, "books/payees.journal.pre-registry"))
	require.NoError(t, err)
	require.Equal(t, "payee Power company\n", string(data))
	_, err = run("payee", "init")
	require.ErrorIs(t, err, os.ErrExist)
	require.NoError(t, os.WriteFile(filepath.Join(root, "wat.yaml"), []byte(config+"payee_registry: true\n"), 0600))
	_, err = run("payee", "alias", "Power company", "POWER ONLINE")
	require.NoError(t, err)
	_, err = run("payee", "add", "New supplier")
	require.NoError(t, err)
	output, err := run("payee", "list")
	require.NoError(t, err)
	require.Contains(t, output, "name: Power company")
	require.Contains(t, output, "POWER ONLINE")
	require.Contains(t, output, "name: New supplier")
	output, err = run("history", "--search", "POWER ONLINE")
	require.NoError(t, err)
	require.Contains(t, output, "Power company | Original payment")
	require.NotContains(t, output, "POWER ONLINE")
	_, err = run("check")
	require.NoError(t, err)
	data, err = os.ReadFile(filepath.Join(root, "books/manual.journal"))
	require.NoError(t, err)
	require.Equal(t, journal, string(data))
}
