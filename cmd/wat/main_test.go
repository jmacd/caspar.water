package main

import (
	"bytes"
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
