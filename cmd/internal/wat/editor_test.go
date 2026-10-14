package wat

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func helperEditor(t *testing.T, books *Books) string {
	t.Helper()
	t.Setenv("WAT_EDITOR_HELPER", "1")
	t.Setenv("WAT_EDITOR_DIRECTORY", books.Config.root)
	t.Setenv("WAT_EDITOR_MODE", "edit")
	return fmt.Sprintf("%q -test.run=^TestEditorHelperProcess$ -- 'argument with spaces'", os.Args[0])
}

func TestEditorHelperProcess(t *testing.T) {
	if os.Getenv("WAT_EDITOR_HELPER") != "1" {
		return
	}
	fail := func(message string) {
		fmt.Fprintln(os.Stderr, message)
		os.Exit(2)
	}
	if len(os.Args) < 2 || os.Args[len(os.Args)-2] != "argument with spaces" {
		fail("quoted editor argument was not preserved")
	}
	directory, err := os.Getwd()
	if err != nil || directory != os.Getenv("WAT_EDITOR_DIRECTORY") {
		fail("editor did not run in the books workspace")
	}
	path := os.Args[len(os.Args)-1]
	data, err := os.ReadFile(path)
	if err != nil {
		fail(err.Error())
	}
	if !strings.Contains(string(data), editorHelpPrefix+"    Assets:Bank\n") ||
		!strings.Contains(string(data), editorHelpPrefix+"    Expenses:Power\n") ||
		!strings.Contains(string(data), "Use a YYYY-MM-DD date within 2021") ||
		strings.Count(string(data), "EDITOR CHEATSHEET") != 1 {
		fail("editor account cheatsheet is missing")
	}
	text := strings.ReplaceAll(string(data), "Power bill", "Edited power")
	text = strings.ReplaceAll(text, "12.34", "15.00")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		fail(err.Error())
	}
	if os.Getenv("WAT_EDITOR_MODE") == "fail" {
		os.Exit(9)
	}
	os.Exit(0)
}

func TestExternalEditor(t *testing.T) {
	books, _ := fixture(t, "; original\n")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	initial, err := draft.Preview(testEntry())
	require.NoError(t, err)
	editor := helperEditor(t, books)
	t.Setenv("VISUAL", editor)
	t.Setenv("EDITOR", "this-editor-must-not-be-launched")
	text, err := books.editText(initial)
	require.NoError(t, err)
	require.Contains(t, text, "Edited power")
	require.Contains(t, text, "$-15.00")
	require.NotContains(t, text, editorHelpPrefix)
	require.Equal(t, "; original\n", read(t, books.Config.WriteJournal))
	require.NoError(t, draft.PreviewText(text))
	_, err = draft.Save()
	require.NoError(t, err)
	require.Contains(t, read(t, books.Config.WriteJournal), text)
	require.NotContains(t, read(t, books.Config.WriteJournal), editorHelpPrefix)
}

func TestEditorErrorsKeepEditedTextWithoutSaving(t *testing.T) {
	books, _ := fixture(t, "")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	initial, err := draft.Preview(testEntry())
	require.NoError(t, err)
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", helperEditor(t, books))
	t.Setenv("WAT_EDITOR_MODE", "fail")
	text, err := books.editText(initial)
	require.ErrorContains(t, err, "editor exited unsuccessfully")
	require.Contains(t, text, "Edited power")
	require.NotContains(t, text, editorHelpPrefix)
	require.Empty(t, read(t, books.Config.WriteJournal))
	t.Setenv("EDITOR", "unterminated 'quote")
	text, err = books.editText(initial)
	require.ErrorContains(t, err, "invalid editor command")
	require.Equal(t, initial, text)
	t.Setenv("EDITOR", "nonexistent-wat-test-editor")
	text, err = books.editText(initial)
	require.ErrorContains(t, err, "editor exited unsuccessfully")
	require.Equal(t, initial, text)
}

func TestEditorCheatsheetStrippingPreservesUserText(t *testing.T) {
	userText := "; evidence note\n2021-04-05 Payee\n    ; id: preserved\n    Assets:Bank  $1.00\n    Equity:Capital  $-1.00\n"
	help := editorHelp(2021, []string{"Assets:Bank", "Equity:Capital"})
	require.Equal(t, userText, stripEditorHelp(userText+help))
	require.Equal(t, userText+"; my own closing note\n", stripEditorHelp(userText+help+"; my own closing note\n"))
	require.Equal(t, userText, stripEditorHelp(userText+strings.TrimSuffix(help, "\n")))
	require.Equal(t, userText, stripEditorHelp(userText+strings.ReplaceAll(help, "\n", "\r\n")))
	require.Equal(t, userText, stripEditorHelp(userText+strings.ReplaceAll(help, editorHelpPrefix, "    "+editorHelpPrefix)))
	require.Equal(t, "; not wat-help: retain this", stripEditorHelp("; not wat-help: retain this"))
}

func TestEditorCheatsheetUsesCurrentAccountsOnReopen(t *testing.T) {
	books, _ := fixture(t, "")
	t.Setenv("VISUAL", helperEditor(t, books))
	initial := testEntry().journalText("test-id")
	text, err := books.editText(initial + editorHelp(2021, []string{"Assets:Old-Account"}))
	require.NoError(t, err)
	require.NotContains(t, text, "Assets:Old-Account")
	require.NotContains(t, text, editorHelpPrefix)
	require.Contains(t, text, "; id: test-id")
}

func TestEditedTextUsesHledgerWithoutReformatting(t *testing.T) {
	books, _ := fixture(t, "; existing\n")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	text, err := draft.Preview(testEntry())
	require.NoError(t, err)
	text = strings.Replace(text, "Assets:Bank  $-12.34", "Assets:Bank", 1)
	text = "; manually written evidence note\n" + text + "    ; custom-note: preserve this\n"
	require.NoError(t, draft.PreviewText(text))
	require.Equal(t, text, draft.text)
	require.Equal(t, "; existing\n", read(t, books.Config.WriteJournal))
	_, err = draft.Save()
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(read(t, books.Config.WriteJournal), text))
	require.ErrorContains(t, draft.PreviewText(text), "already been saved")
}

func TestEditedAssertionUsesCompleteBooks(t *testing.T) {
	books, _ := fixture(t, "2021-01-01 Opening\n    Assets:Bank  $100\n    Equity:Capital  $-100\n")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	text, err := draft.Preview(testEntry())
	require.NoError(t, err)
	text = strings.Replace(text, "$-12.34", "$-12.34 = $87.66", 1)
	require.NoError(t, draft.PreviewText(text))
	_, err = draft.Save()
	require.NoError(t, err)
	require.NoError(t, books.Check())
}

func TestInvalidEditedTextCannotBeSaved(t *testing.T) {
	books, _ := fixture(t, "")
	for _, tc := range []struct {
		name   string
		change func(string, string) string
	}{
		{"multiple", func(s, _ string) string { return s + "\n" + s }},
		{"directive", func(s, _ string) string { return "include accounts.journal\n" + s }},
		{"alias", func(s, _ string) string { return "alias Assets=Expenses\n" + s }},
		{"unbalanced", func(s, _ string) string { return strings.Replace(s, "$12.34", "$12.35", 1) }},
		{"wrong-year", func(s, _ string) string { return strings.Replace(s, "2021", "2022", 1) }},
		{"unknown-account", func(s, _ string) string { return strings.Replace(s, "Expenses:Power", "Expenses:Typo", 1) }},
		{"wrong-id", func(s, id string) string { return strings.Replace(s, id, "different", 1) }},
		{"missing-id", func(s, id string) string { return strings.Replace(s, "    ; id: "+id+"\n", "", 1) }},
		{"virtual", func(s, _ string) string {
			s = strings.Replace(s, "Expenses:Power", "[Expenses:Power]", 1)
			return strings.Replace(s, "Assets:Bank", "[Assets:Bank]", 1)
		}},
		{"commodity", func(s, _ string) string { return strings.ReplaceAll(s, "$", "EUR ") }},
		{"control", func(s, _ string) string { return "; note \x1b\n" + s }},
		{"empty", func(_, _ string) string { return "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			draft, err := books.NewDraft()
			require.NoError(t, err)
			text, err := draft.Preview(testEntry())
			require.NoError(t, err)
			require.Error(t, draft.PreviewText(tc.change(text, draft.id)))
			_, err = draft.Save()
			require.ErrorContains(t, err, "preview")
			require.Empty(t, read(t, books.Config.WriteJournal))
		})
	}
}

func TestIncompleteFormCanSeedEditor(t *testing.T) {
	books, _ := fixture(t, "")
	draft, err := books.NewDraft()
	require.NoError(t, err)
	entry := Entry{Postings: []Posting{{Account: "Expenses:Power"}, {Account: "Assets:Bank"}}}
	text := entry.journalText(draft.id)
	require.Contains(t, text, "YYYY-MM-DD")
	require.Contains(t, text, "; id: "+draft.id)
	require.Error(t, draft.PreviewText(text))
	text = strings.Replace(text, "YYYY-MM-DD ", "2021-04-05 Electric company", 1)
	text = strings.Replace(text, "Expenses:Power\n", "Expenses:Power  $12.34\n", 1)
	text = strings.Replace(text, "Assets:Bank\n", "Assets:Bank  $-12.34\n", 1)
	require.NoError(t, draft.PreviewText(text))
	require.Empty(t, read(t, books.Config.WriteJournal))
}
