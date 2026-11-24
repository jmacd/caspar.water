package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBillingCLIRequiresPreparedWorkspace(t *testing.T) {
	if _, err := exec.LookPath("hledger"); err != nil {
		t.Skip("requires hledger")
	}
	dir := t.TempDir()
	config := filepath.Join(dir, "wat.yaml")
	require.NoError(t, os.WriteFile(config, []byte("version: 3\nname: Test\nchart_defs: [chart.yaml]\naccounts: [Equity:218.000:Owner]\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "chart.yaml"), []byte(testAccountChart), 0600))
	run := func(args ...string) (string, error) {
		c := command()
		var out bytes.Buffer
		c.SetOut(&out)
		c.SetArgs(append([]string{"--config", config}, args...))
		err := c.Execute()
		return out.String(), err
	}
	_, err := run("billing", "review", "2021-12")
	require.ErrorContains(t, err, "billing workspace")
	require.NoDirExists(t, filepath.Join(dir, "books"), "review must not initialize missing source journals")
	_, err = run("payee", "add", "Journal")
	require.NoError(t, err)
	out, err := run("billing", "review", "2021-12")
	require.NoError(t, err)
	require.Contains(t, out, "2021-12-01 through 2021-12-31")
	out, err = run("billing", "review", "--from", "2021-01-15", "--through", "2021-12-31")
	require.NoError(t, err)
	require.Contains(t, out, "No applied billing cutoff")
	require.Contains(t, out, "2021-01-15 through 2021-12-31")
	master := filepath.Join(dir, "books", "books.journal")
	require.NoError(t, os.WriteFile(master, []byte("; stale master\n"), 0600))
	_, err = run("billing", "close", "2021-12", "--policy", "rates.yaml", "--margin-percent", "5")
	require.ErrorContains(t, err, "is stale")
	data, err := os.ReadFile(master)
	require.NoError(t, err)
	require.Equal(t, "; stale master\n", string(data))
}
