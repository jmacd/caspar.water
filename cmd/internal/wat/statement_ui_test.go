package wat

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

func TestStatementTerminalSaveAndLearnNext(t *testing.T) {
	books, statement, _ := statementFixture(t)
	app := tview.NewApplication()
	screen := tcell.NewSimulationScreen("UTF-8")
	app.SetScreen(screen)
	screen.SetSize(120, 40)
	ui := &terminal{app: app, books: books, statement: statement}
	frames := make(chan string, 100)
	app.SetAfterDrawFunc(func(_ tcell.Screen) {
		var text strings.Builder
		for y := 0; y < 40; y++ {
			for x := 0; x < 120; x++ {
				r, _, _, _ := screen.GetContent(x, y)
				text.WriteRune(r)
			}
			text.WriteByte('\n')
		}
		frames <- text.String()
	})
	ui.statementOpen(1)
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	t.Cleanup(app.Stop)
	next := func() string {
		t.Helper()
		select {
		case frame := <-frames:
			return frame
		case err := <-done:
			t.Fatalf("terminal exited: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("terminal did not draw")
		}
		return ""
	}
	key := func(k tcell.Key, r rune) string {
		app.QueueEvent(tcell.NewEventKey(k, r, tcell.ModNone))
		return next()
	}
	require.Contains(t, next(), "Statement row 2/3")
	focused, ok := app.GetFocus().(*tview.InputField)
	require.True(t, ok)
	require.Empty(t, focused.GetText(), "start directly in the counteraccount")
	for _, r := range "power" {
		key(tcell.KeyRune, r)
	}
	key(tcell.KeyEnter, 0)
	require.Contains(t, key(tcell.KeyF2, 0), "Transaction saved and validated")
	saved := read(t, books.Config.WriteJournal)
	require.Contains(t, saved, "; bank-row: "+statement.key(1))
	require.Contains(t, saved, "Assets:Bank:Checking  $-12.34")
	require.Contains(t, key(tcell.KeyEnter, 0), "Statement row 3/3")
	focused, ok = app.GetFocus().(*tview.InputField)
	require.True(t, ok)
	require.Equal(t, "Expenses:Power:Utility", focused.GetText(), "next statement row learns the just-saved category")
	require.Contains(t, key(tcell.KeyEscape, 0), "Press Esc again to discard")
	require.Contains(t, key(tcell.KeyEscape, 0), "1/3 recorded")
	require.Equal(t, saved, read(t, books.Config.WriteJournal), "deferring the next row must not post it")
	key(tcell.KeyDown, 0)
	require.Contains(t, key(tcell.KeyEnter, 0), "Revise working transaction")
	focused, ok = app.GetFocus().(*tview.InputField)
	require.True(t, ok)
	require.Equal(t, "Expenses:Power:Utility", focused.GetText())
	key(tcell.KeyCtrlU, 0)
	for _, r := range "Income:Other:Misc" {
		key(tcell.KeyRune, r)
	}
	require.Contains(t, key(tcell.KeyF2, 0), "Transaction saved and validated")
	require.Contains(t, key(tcell.KeyEnter, 0), "1/3 recorded")
	require.Equal(t, 1, strings.Count(read(t, books.Config.WriteJournal), "; bank-row: "+statement.key(1)))
	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.Equal(t, "Income:Other:Misc", statuses[2].Suggestion)
	app.Stop()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("terminal did not exit")
	}
}

func TestStatementBatchAdvanceRequiresRecordedRows(t *testing.T) {
	books, statement, _ := statementFixture(t)
	queue := &statementQueue{position: 1, total: 2, path: "2021-01.pdf"}
	ui := &terminal{app: tview.NewApplication(), books: books, statement: statement, queue: queue}
	ui.statementList()
	list, ok := ui.app.GetFocus().(*tview.List)
	require.True(t, ok)
	require.Equal(t, len(statement.Rows)+2, list.GetItemCount(), "pending rows must not offer Next")
	list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone), func(tview.Primitive) {})
	require.False(t, queue.advance)
	for index := range statement.Rows {
		draft, entry, err := books.StatementDraft(statement, index)
		require.NoError(t, err)
		entry.Postings[1].Account = "Equity:Capital:Owner"
		_, err = draft.Preview(entry)
		require.NoError(t, err)
		_, err = draft.Save()
		require.NoError(t, err)
	}
	ui.statementList()
	list = ui.app.GetFocus().(*tview.List)
	label, _ := list.GetItemText(len(statement.Rows))
	require.Equal(t, "Next statement", label)
	require.Equal(t, len(statement.Rows), list.GetCurrentItem(), "offer advancement directly after recording the final row")
	ui.menu()
	ui.app.GetFocus().(*tview.List).InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone), func(tview.Primitive) {})
	require.False(t, queue.advance, "Quit must end the batch, not load the next file")
	ui.statementList()
	ui.app.GetFocus().(*tview.List).InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone), func(tview.Primitive) {})
	require.True(t, queue.advance)
	queue.advance = false
	queue.position = queue.total
	ui.statementList()
	list = ui.app.GetFocus().(*tview.List)
	label, _ = list.GetItemText(len(statement.Rows))
	require.Equal(t, "Finish statement batch", label)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone), func(tview.Primitive) {})
	require.True(t, queue.advance)
}
