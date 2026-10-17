package wat

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

type terminalFrame struct {
	text  string
	focus string
	value string
	x, y  int
}

func TestTerminalRecordReviewAndCancel(t *testing.T) {
	books, _ := fixture(t, "; original text\n")
	t.Setenv("VISUAL", helperEditor(t, books))
	screen := tcell.NewSimulationScreen("UTF-8")
	app := tview.NewApplication().SetScreen(screen)
	screen.SetSize(120, 40)
	ui := &terminal{app: app, books: books}
	frames := make(chan terminalFrame, 100)
	app.SetAfterDrawFunc(func(_ tcell.Screen) {
		width, height := screen.Size()
		var text strings.Builder
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, _, _, _ := screen.GetContent(x, y)
				text.WriteRune(r)
			}

			text.WriteByte('\n')
		}
		frames <- terminalFrame{text: text.String()}
	})
	ui.menu()
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	t.Cleanup(app.Stop)
	next := func() terminalFrame {
		t.Helper()
		select {
		case frame := <-frames:
			switch focus := app.GetFocus().(type) {
			case *tview.Button:
				frame.focus = focus.GetLabel()
			case *tview.InputField:
				frame.focus = focus.GetLabel()
				frame.value = focus.GetText()
				frame.x, frame.y, _, _ = focus.GetRect()
			}
			return frame
		case err := <-done:
			t.Fatalf("terminal exited unexpectedly: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("terminal did not draw")
		}
		return terminalFrame{}
	}
	key := func(key tcell.Key, r rune) terminalFrame {
		t.Helper()
		app.QueueEvent(tcell.NewEventKey(key, r, tcell.ModNone))
		return next()
	}
	typeText := func(text string) {
		t.Helper()
		for _, r := range text {
			key(tcell.KeyRune, r)
		}
	}
	require.Contains(t, next().text, "Water Accounting Terminal")
	frame := key(tcell.KeyRune, '1')
	require.Contains(t, frame.text, "UNSAVED")
	require.Equal(t, "Date (YYYY-MM-DD)", frame.focus)
	frame = key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "date must be")
	require.Equal(t, "; original text\n", read(t, books.Config.WriteJournal))
	key(tcell.KeyEnter, 0)
	frame = key(tcell.KeyBacktab, 0)
	require.Equal(t, "Back (Esc)", frame.focus)
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Discard this unsaved transaction")
	frame = key(tcell.KeyEnter, 0)
	require.Equal(t, "Back (Esc)", frame.focus)
	frame = key(tcell.KeyTab, 0)
	require.Equal(t, "Date (YYYY-MM-DD)", frame.focus)
	frame = key(tcell.KeyDown, 0)
	require.Equal(t, "Payee", frame.focus)
	frame = key(tcell.KeyUp, 0)
	require.Equal(t, "Date (YYYY-MM-DD)", frame.focus)
	typeText("2021-04-05")
	key(tcell.KeyTab, 0)
	typeText("Electric company")
	key(tcell.KeyTab, 0)
	typeText("Power bill")
	key(tcell.KeyTab, 0)
	typeText("check-7")
	key(tcell.KeyTab, 0)
	typeText("sources/power.pdf")
	frame = key(tcell.KeyDown, 0)
	accountX, firstY := frame.x, frame.y
	typeText("pow")
	frame = key(tcell.KeyTab, 0)
	require.Equal(t, firstY, frame.y)
	require.Greater(t, frame.x, accountX)
	key(tcell.KeyBacktab, 0)
	key(tcell.KeyEnter, 0)
	frame = key(tcell.KeyDown, 0)
	require.Equal(t, accountX, frame.x)
	require.Equal(t, firstY+1, frame.y)
	frame = key(tcell.KeyUp, 0)
	require.Equal(t, accountX, frame.x)
	require.Equal(t, firstY, frame.y)
	key(tcell.KeyTab, 0)
	typeText("12.34")
	key(tcell.KeyTab, 0)
	typeText("bank")
	key(tcell.KeyEnter, 0)
	key(tcell.KeyTab, 0)
	typeText("-12.34")
	frame = key(tcell.KeyTab, 0)
	require.Equal(t, "Save (F2)", frame.focus)
	frame = key(tcell.KeyTab, 0)
	require.Equal(t, "Add posting", frame.focus)
	frame = key(tcell.KeyEnter, 0)
	require.Equal(t, accountX, frame.x)
	require.Equal(t, firstY+2, frame.y)
	key(tcell.KeyTab, 0)
	key(tcell.KeyTab, 0)
	key(tcell.KeyTab, 0)
	frame = key(tcell.KeyTab, 0)
	require.Equal(t, "Remove last posting", frame.focus)
	frame = key(tcell.KeyEnter, 0)
	require.Equal(t, accountX, frame.x)
	require.Equal(t, firstY+1, frame.y)
	frame = key(tcell.KeyTab, 0)
	require.Equal(t, "-12.34", frame.value)
	frame = key(tcell.KeyTab, 0)
	require.Equal(t, "Save (F2)", frame.focus)
	require.Contains(t, frame.text, "Balance: $0.00 | Balanced")
	require.Contains(t, frame.text, "Back (Esc)")
	require.Equal(t, "; original text\n", read(t, books.Config.WriteJournal))
	frame = key(tcell.KeyF4, 0)
	require.Contains(t, frame.text, "Edited power")
	require.Contains(t, frame.text, "$-15.00")
	require.Equal(t, "; original text\n", read(t, books.Config.WriteJournal))
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Transaction saved and validated")
	saved := read(t, books.Config.WriteJournal)
	require.Contains(t, saved, "2021-04-05 Electric company | Edited power")
	require.Contains(t, saved, "Expenses:Power:Utility  $15.00")
	require.Contains(t, saved, "Assets:Bank:Checking  $-15.00")
	key(tcell.KeyEnter, 0)
	frame = key(tcell.KeyRune, '2')
	require.Contains(t, frame.text, "Electric company")
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Revise working transaction")
	key(tcell.KeyTab, 0)
	key(tcell.KeyTab, 0)
	frame = key(tcell.KeyTab, 0)
	require.Equal(t, "Reference", frame.focus)
	require.Equal(t, "check-7", frame.value)
	for range "check-7" {
		key(tcell.KeyBackspace2, 0)
	}
	typeText("check-8")
	for i := 0; i < 6; i++ {
		frame = key(tcell.KeyTab, 0)
	}
	require.Equal(t, "Save (F2)", frame.focus, "Save must immediately follow the final field when revising")
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Transaction saved and validated")
	saved = read(t, books.Config.WriteJournal)
	require.Contains(t, saved, "; reference: check-8")
	require.Equal(t, 1, strings.Count(saved, "; id:"))
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Review / revise journal")
	key(tcell.KeyEnter, 0)
	key(tcell.KeyTab, 0)
	typeText("UNSAVED")
	frame = key(tcell.KeyEscape, 0)
	require.Contains(t, frame.text, "Press Esc again to discard")
	require.Equal(t, saved, read(t, books.Config.WriteJournal))
	frame = key(tcell.KeyEscape, 0)
	require.Contains(t, frame.text, "Review / revise journal")
	require.Equal(t, saved, read(t, books.Config.WriteJournal), "cancelling a revision must preserve the saved entry")
	key(tcell.KeyEscape, 0)
	key(tcell.KeyRune, '1')
	key(tcell.KeyTab, 0)
	typeText("elect")
	frame = key(tcell.KeyEnter, 0)
	require.Equal(t, "Electric company", frame.value)
	key(tcell.KeyTab, 0)
	typeText("edited")
	frame = key(tcell.KeyEnter, 0)
	require.Equal(t, "Edited power", frame.value)
	frame = key(tcell.KeyEscape, 0)
	require.Contains(t, frame.text, "Discard this unsaved transaction")
	frame = key(tcell.KeyEscape, 0)
	require.Contains(t, frame.text, "Water Accounting Terminal")
	require.Equal(t, saved, read(t, books.Config.WriteJournal))
	key(tcell.KeyRune, '1')
	for _, value := range []string{
		"2021-04-06", "Electric company", "Direct save", "", "",
		"Expenses:Power:Utility", "12.34", "Assets:Bank:Checking", "-12.34",
	} {
		typeText(value)
		key(tcell.KeyTab, 0)
	}
	frame = key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "Transaction saved and validated")
	require.Contains(t, read(t, books.Config.WriteJournal), "2021-04-06 Electric company | Direct save")
	key(tcell.KeyEnter, 0)
	app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("terminal did not exit")
	}
}

func TestJournalBrowserUsesRightAlignedBankColumns(t *testing.T) {
	books, _ := fixture(t, testEntry().journalText("entry"))
	books.Config.BankAccount = "Assets:Bank:Checking"
	ui := &terminal{app: tview.NewApplication(), books: books}
	ui.journalEntries()
	table, ok := ui.app.GetFocus().(*tview.Table)
	require.True(t, ok)
	require.Equal(t, 5, table.GetColumnCount())
	require.Equal(t, "Bank change", table.GetCell(0, 3).Text)
	require.Equal(t, tview.AlignRight, table.GetCell(0, 3).Align)
	require.Equal(t, "Bank balance", table.GetCell(0, 4).Text)
	require.Equal(t, tview.AlignRight, table.GetCell(0, 4).Align)
	require.Equal(t, "-12.34", table.GetCell(1, 3).Text)
	require.Equal(t, tview.AlignRight, table.GetCell(1, 3).Align)
	require.Equal(t, "-12.34", table.GetCell(1, 4).Text)
	require.Equal(t, tview.AlignRight, table.GetCell(1, 4).Align)
	for row := 0; row < table.GetRowCount(); row++ {
		for column := 0; column < table.GetColumnCount(); column++ {
			require.NotContains(t, table.GetCell(row, column).Text, "Enter to revise")
		}
	}
}

func TestEntryGridCompletionAndNavigation(t *testing.T) {
	app := tview.NewApplication()
	screen := tcell.NewSimulationScreen("UTF-8")
	require.NoError(t, screen.Init())
	t.Cleanup(screen.Fini)
	screen.SetSize(100, 24)
	bank := "Assets:131.2-Cash-in-Bank"
	savings := "Assets:131.3-Savings"
	payee := "City [red] Water"
	draft := &Draft{
		accounts: []string{bank, savings, "Expenses:Power:Utility"},
		history:  completionHistory{payees: []string{payee}, descriptions: []string{"Power bill"}},
	}
	entry := Entry{Postings: make([]Posting, 2)}
	focus := 0
	added, saved := false, false
	grid := newEntryGrid(app, &entry, draft, &focus,
		func() { added = true }, func() {}, func() { saved = true }, func() {})
	grid.SetRect(0, 0, 100, 24)
	app.SetFocus(grid.controls[0].primitive)
	draw := func() { grid.Draw(screen) }
	draw()
	key := func(key tcell.Key, r rune) {
		t.Helper()
		grid.InputHandler()(tcell.NewEventKey(key, r, tcell.ModNone), func(p tview.Primitive) { app.SetFocus(p) })
		draw()
	}
	typeText := func(text string) {
		for _, r := range text {
			key(tcell.KeyRune, r)
		}
	}
	assertFocus := func(index int) {
		t.Helper()
		require.Equal(t, index, focus)
		require.Same(t, grid.controls[index].primitive, app.GetFocus())
	}
	key(tcell.KeyBacktab, 0)
	assertFocus(12)
	key(tcell.KeyTab, 0)
	assertFocus(0)
	key(tcell.KeyUp, 0)
	assertFocus(0)
	key(tcell.KeyDown, 0)
	typeText("city")
	key(tcell.KeyDown, 0)
	assertFocus(2)
	require.Equal(t, "city", entry.Payee, "arrows must not accept completion")
	key(tcell.KeyUp, 0)
	key(tcell.KeyEnter, 0)
	require.Equal(t, payee, entry.Payee, "completion must preserve literal color-tag text")
	key(tcell.KeyEnter, 0)
	assertFocus(2)
	typeText("BILL")
	key(tcell.KeyEnter, 0)
	require.Equal(t, "Power bill", entry.Description)
	key(tcell.KeyTab, 0)
	key(tcell.KeyDown, 0)
	key(tcell.KeyDown, 0)
	assertFocus(5)
	typeText("13")
	key(tcell.KeyTab, 0)
	assertFocus(6)
	require.Equal(t, "13", entry.Postings[0].Account)
	x1, y1, _, _ := grid.controls[5].primitive.GetRect()
	x2, y2, _, _ := grid.controls[6].primitive.GetRect()
	require.Equal(t, y1, y2)
	require.Greater(t, x2, x1, "account and amount must occupy separate columns")
	key(tcell.KeyDown, 0)
	assertFocus(8)
	key(tcell.KeyDown, 0)
	assertFocus(8)
	key(tcell.KeyUp, 0)
	assertFocus(6)
	key(tcell.KeyBacktab, 0)
	assertFocus(5)
	key(tcell.KeyCtrlN, 0)
	require.Equal(t, "13", entry.Postings[0].Account, "cycling must preserve the query")
	key(tcell.KeyCtrlP, 0)
	key(tcell.KeyCtrlN, 0)
	key(tcell.KeyEnter, 0)
	require.Equal(t, savings, entry.Postings[0].Account)
	key(tcell.KeyDown, 0)
	assertFocus(7)
	typeText("131.2")
	key(tcell.KeyEnter, 0)
	require.Equal(t, bank, entry.Postings[1].Account)
	key(tcell.KeyTab, 0)
	assertFocus(8)
	key(tcell.KeyTab, 0)
	assertFocus(9)
	key(tcell.KeyEnter, 0)
	require.True(t, saved)
	require.False(t, added)
	key(tcell.KeyTab, 0)
	assertFocus(10)
	key(tcell.KeyEnter, 0)
	require.True(t, added)
	key(tcell.KeyTab, 0)
	assertFocus(12) // The disabled remove button is skipped.
	key(tcell.KeyBacktab, 0)
	assertFocus(10)
	key(tcell.KeyBacktab, 0)
	assertFocus(9)
	key(tcell.KeyUp, 0)
	assertFocus(7)

	entry.Postings[0].Account = bank
	focus = 5
	grid = newEntryGrid(app, &entry, draft, &focus,
		func() {}, func() {}, func() {}, func() {})
	grid.SetRect(0, 0, 100, 24)
	app.SetFocus(grid.controls[5].primitive)
	draw()
	key(tcell.KeyEnter, 0)
	assertFocus(5)
	require.Equal(t, bank, entry.Postings[0].Account, "opening the chooser must not clear the current account")
	key(tcell.KeyDown, 0)
	assertFocus(5)
	key(tcell.KeyEnter, 0)
	require.Equal(t, savings, entry.Postings[0].Account)
	assertFocus(5)
	key(tcell.KeyTab, 0)
	assertFocus(6)

	entry.Postings = make([]Posting, 30)
	grid = newEntryGrid(app, &entry, draft, &focus,
		func() {}, func() {}, func() {}, func() {})
	grid.SetRect(0, 0, 60, 10)
	app.SetFocus(grid.controls[64].primitive)
	grid.Draw(screen)
	_, y, _, height := app.GetFocus().GetRect()
	require.GreaterOrEqual(t, y, 1)
	require.LessOrEqual(t, y+height, 9, "focused posting must scroll into the viewport")
	for _, index := range []int{65, 67, 68} {
		_, buttonY, _, buttonHeight := grid.controls[index].primitive.GetRect()
		require.GreaterOrEqual(t, buttonY, 1)
		require.LessOrEqual(t, buttonY+buttonHeight, 9, "actions must stay visible while posting rows scroll")
	}
	field, ok := grid.controls[64].primitive.(*tview.InputField)
	require.True(t, ok)
	field.SetText("-100")
	require.Contains(t, grid.balance.GetText(false), "To balance: +100.00 USD (debit)")
	other, ok := grid.controls[62].primitive.(*tview.InputField)
	require.True(t, ok)
	other.SetText("40")
	require.Contains(t, grid.balance.GetText(false), "To balance: +60.00 USD (debit)")
	other.SetText("100")
	require.Equal(t, "Balance: $0.00 | Balanced", grid.balance.GetText(false))
	other.SetText("-")
	require.Contains(t, grid.balance.GetText(false), "Balance unavailable: posting 29")
	other.SetText("")
	account, ok := grid.controls[63].primitive.(*tview.InputField)
	require.True(t, ok)
	app.SetFocus(account)
	account.SetText("13")
	account.Autocomplete()
	draw()
	_, balanceY, _, _ := grid.balance.GetRect()
	var balanceRow strings.Builder
	for x := 1; x < 59; x++ {
		r, _, _, _ := screen.GetContent(x, balanceY)
		balanceRow.WriteRune(r)
	}
	require.Contains(t, balanceRow.String(), "To balance: +100.00")
	_, backY, _, _ := grid.controls[68].primitive.GetRect()
	var actionRow strings.Builder
	for x := 1; x < 59; x++ {
		r, _, _, _ := screen.GetContent(x, backY)
		actionRow.WriteRune(r)
	}
	require.Contains(t, actionRow.String(), "Back (Esc)")
	app.SetFocus(grid.controls[64].primitive)
	key(tcell.KeyUp, 0)
	assertFocus(62)
	key(tcell.KeyBacktab, 0)
	assertFocus(61)
}
