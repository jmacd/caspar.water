package wat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

func TestBankStatementsMenuResumesRecordedRows(t *testing.T) {
	books, statement, pdf := statementFixture(t)
	draft, entry, err := books.StatementDraft(statement, 0)
	require.NoError(t, err)
	entry.Postings[1].Account = "Equity:Capital:Owner"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	saved := read(t, books.Config.WriteJournal)
	require.NoError(t, os.Remove(pdf), "the original import path is no longer needed")
	ui := &terminal{app: tview.NewApplication(), books: books}
	press := func(key tcell.Key, r rune) {
		t.Helper()
		list, ok := ui.app.GetFocus().(*tview.List)
		require.True(t, ok)
		list.InputHandler()(tcell.NewEventKey(key, r, tcell.ModNone), func(tview.Primitive) {})
	}
	ui.menu()
	press(tcell.KeyRune, '5')
	list := ui.app.GetFocus().(*tview.List)
	require.Equal(t, 2, list.GetItemCount())
	label, source := list.GetItemText(0)
	require.Contains(t, label, "2021-04-01 to 2021-05-31")
	require.NotContains(t, label, "2021-05-05", "last transaction date is not the statement end date")
	require.Contains(t, label, "1 recorded, 2 pending")
	require.Equal(t, statement.Source, source)
	press(tcell.KeyEnter, 0)
	require.Equal(t, statement, ui.statement)
	list = ui.app.GetFocus().(*tview.List)
	label, _ = list.GetItemText(0)
	require.Contains(t, label, "RECORDED")
	label, _ = list.GetItemText(1)
	require.Contains(t, label, "PENDING")
	press(tcell.KeyEnter, 0)
	account, ok := ui.app.GetFocus().(*tview.InputField)
	require.True(t, ok, "recorded rows must reopen in the existing revision form")
	require.Equal(t, "Equity:Capital:Owner", account.GetText())
	ui.entryBack()
	press(tcell.KeyEscape, 0)
	require.Nil(t, ui.statement, "return from the browser must restore the previous active statement")
	press(tcell.KeyEscape, 0)
	list = ui.app.GetFocus().(*tview.List)
	label, _ = list.GetItemText(4)
	require.Equal(t, "Bank statements", label)
	require.Equal(t, saved, read(t, books.Config.WriteJournal), "opening and navigating must not duplicate entries")
}

func TestBankStatementsDoesNotInventMissingPeriod(t *testing.T) {
	books, statement, _ := statementFixture(t)
	legacy := strings.Replace(statementJSON, ` "wat_statement_period":{"start":"2021-04-01","end":"2021-05-31"},`, "", 1)
	require.NoError(t, os.WriteFile(filepath.Join(books.Config.root, filepath.Dir(statement.Source), "extracted.json"), []byte(legacy), 0600))
	ui := &terminal{app: tview.NewApplication(), books: books}
	ui.bankStatements()
	label, _ := ui.app.GetFocus().(*tview.List).GetItemText(0)
	require.Contains(t, label, "Statement dates unavailable")
	require.NotContains(t, label, "2021-04-01")
	require.NotContains(t, label, "2021-05-05")
}

func TestBankStatementsBrowserPreservesActiveBatch(t *testing.T) {
	books, statement, _ := statementFixture(t)
	for index := range statement.Rows {
		draft, entry, err := books.StatementDraft(statement, index)
		require.NoError(t, err)
		entry.Postings[1].Account = "Equity:Capital:Owner"
		_, err = draft.Preview(entry)
		require.NoError(t, err)
		_, err = draft.Save()
		require.NoError(t, err)
	}
	queue := &statementQueue{position: 1, total: 2, path: "first.pdf"}
	ui := &terminal{app: tview.NewApplication(), books: books, statement: statement, queue: queue}
	ui.bankStatements()
	ui.app.GetFocus().(*tview.List).InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	require.Nil(t, ui.queue, "a cached review must not advance a command-line batch")
	list := ui.app.GetFocus().(*tview.List)
	require.Equal(t, len(statement.Rows)+3, list.GetItemCount())
	label, _ := list.GetItemText(len(statement.Rows) + 2)
	require.Equal(t, "Back to statements", label)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), func(tview.Primitive) {})
	require.Same(t, statement, ui.statement)
	require.Same(t, queue, ui.queue)
	require.Nil(t, ui.statementBack)
	require.False(t, queue.advance)
	ui.menu()
	ui.app.GetFocus().(*tview.List).InputHandler()(tcell.NewEventKey(tcell.KeyRune, '6', tcell.ModNone), func(tview.Primitive) {})
	list = ui.app.GetFocus().(*tview.List)
	label, _ = list.GetItemText(len(statement.Rows))
	require.Equal(t, "Reconcile statement", label)
}

func TestBankStatementsMenuEmptyAndInvalidCache(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		books, _ := fixture(t, "")
		ui := &terminal{app: tview.NewApplication(), books: books}
		ui.menu()
		ui.app.GetFocus().(*tview.List).InputHandler()(tcell.NewEventKey(tcell.KeyRune, '5', tcell.ModNone), func(tview.Primitive) {})
		button, ok := ui.app.GetFocus().(*tview.Button)
		require.True(t, ok)
		require.Equal(t, "OK", button.GetLabel())
	})
	t.Run("invalid", func(t *testing.T) {
		books, _, _ := statementFixture(t)
		books.Config.Sources = books.Config.WriteJournal
		ui := &terminal{app: tview.NewApplication(), books: books}
		ui.bankStatements()
		button, ok := ui.app.GetFocus().(*tview.Button)
		require.True(t, ok, "cache errors must be shown rather than silently skipped")
		require.Equal(t, "OK", button.GetLabel())
	})
}

func TestFinishedStatementIsReadOnlyInStatementMenus(t *testing.T) {
	books, statement, _ := statementFixture(t)
	recordStatement(t, books, statement)
	require.NoError(t, books.FinishReconciliation(statement))
	ui := &terminal{app: tview.NewApplication(), books: books, statement: statement}
	ui.statementList()
	list := ui.app.GetFocus().(*tview.List)
	require.Equal(t, len(statement.Rows)+2, list.GetItemCount(), "finished statements offer neither Reconcile nor edit actions")
	for index := range statement.Rows {
		label, hint := list.GetItemText(index)
		require.Contains(t, label, "RECONCILED")
		require.Contains(t, hint, "read-only")
	}
	list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	button, ok := ui.app.GetFocus().(*tview.Button)
	require.True(t, ok)
	require.Equal(t, "OK", button.GetLabel(), "opening a finished row explains its lock instead of editing")

	ui.bankStatements()
	list = ui.app.GetFocus().(*tview.List)
	label, hint := list.GetItemText(0)
	require.Contains(t, label, "RECONCILED")
	require.Contains(t, hint, "read-only")
}

func TestReconciliationScreenOffersFinishAtZeroDifference(t *testing.T) {
	books, statement, _ := statementFixture(t)
	recordStatement(t, books, statement)
	ui := &terminal{app: tview.NewApplication(), books: books, statement: statement}
	ui.reconciliation()
	list, ok := ui.app.GetFocus().(*tview.List)
	require.True(t, ok)
	require.Equal(t, 2, list.GetItemCount())
	label, hint := list.GetItemText(0)
	require.Equal(t, "Finish reconciliation", label)
	require.Contains(t, hint, "Persist checkpoint")
}

func TestReconciliationScreenDoesNotOpenTransactionEditor(t *testing.T) {
	books, statement, _ := statementFixture(t)
	saveBankEntry(t, books, "2021-04-10", "Unexplained check", "25.00")
	recordStatement(t, books, statement)
	ui := &terminal{app: tview.NewApplication(), books: books, statement: statement}
	ui.reconciliation()
	list, ok := ui.app.GetFocus().(*tview.List)
	require.True(t, ok)
	label, hint := list.GetItemText(0)
	require.Contains(t, label, "UNEXPLAINED")
	require.Contains(t, hint, "Review or edit it from the journal")
	list.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), func(tview.Primitive) {})
	require.Same(t, list, ui.app.GetFocus(), "Enter must not open a reconciliation transaction or editor")
}

func TestStatementPageShowsBlockedReconciliationForOpeningMismatch(t *testing.T) {
	books, statement, _ := statementFixture(t)
	saveBankEntry(t, books, "2021-03-31", "Preexisting mistake", "25.00")
	recordStatement(t, books, statement)
	ui := &terminal{app: tview.NewApplication(), books: books, statement: statement}
	ui.statementList()
	list := ui.app.GetFocus().(*tview.List)
	label, hint := list.GetItemText(len(statement.Rows))
	require.Equal(t, "Reconciliation blocked", label)
	require.Contains(t, hint, "book opening balance -25.00")
	require.NotContains(t, hint, "outstanding")
	list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'r', tcell.ModNone), func(tview.Primitive) {})
	require.Same(t, list, ui.app.GetFocus(), "blocked reconciliation has no r shortcut")
}

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
	require.Equal(t, "Reconcile statement", label)
	require.Equal(t, len(statement.Rows), list.GetCurrentItem(), "offer advancement directly after recording the final row")
	ui.menu()
	ui.app.GetFocus().(*tview.List).InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone), func(tview.Primitive) {})
	require.False(t, queue.advance, "Quit must end the batch, not load the next file")
	require.NoError(t, books.FinishReconciliation(statement))
	ui.statementList()
	list = ui.app.GetFocus().(*tview.List)
	label, _ = list.GetItemText(len(statement.Rows))
	require.Equal(t, "Next statement", label)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone), func(tview.Primitive) {})
	require.True(t, queue.advance)
	queue.advance = false
	queue.position = queue.total
	ui.statementList()
	list = ui.app.GetFocus().(*tview.List)
	label, _ = list.GetItemText(len(statement.Rows))
	require.Equal(t, "Exit statement batch", label)
	list.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'n', tcell.ModNone), func(tview.Primitive) {})
	require.True(t, queue.advance)
}
