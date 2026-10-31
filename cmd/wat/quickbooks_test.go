package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuickBooksFilesScansAndSortsInbox(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"b.xlsx", "a.xls", "c.csv", "statement.pdf"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("test"), 0600))
	}
	paths, err := quickBooksFiles(nil, root)
	require.NoError(t, err)
	require.Equal(t, []string{
		filepath.Join(root, "a.xls"),
		filepath.Join(root, "b.xlsx"),
		filepath.Join(root, "c.csv"),
	}, paths)
}

func TestQuickBooksFilesResolveRelativeNamesInInbox(t *testing.T) {
	paths, err := quickBooksFiles([]string{"report.xlsx", "/tmp/register.xls"}, "/inbox")
	require.NoError(t, err)
	require.Equal(t, []string{"/inbox/report.xlsx", "/tmp/register.xls"}, paths)
}

func TestQuickBooksFilesRejectUnsupportedInput(t *testing.T) {
	_, err := quickBooksFiles([]string{"statement.pdf"}, "")
	require.ErrorContains(t, err, "must be .xls, .xlsx, or .csv")
}
