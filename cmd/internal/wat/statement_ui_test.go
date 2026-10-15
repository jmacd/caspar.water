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
	require.Contains(t, saved, "Assets:Bank  $-12.34")
	require.Contains(t, key(tcell.KeyEnter, 0), "Statement row 3/3")
	focused, ok = app.GetFocus().(*tview.InputField)
	require.True(t, ok)
	require.Equal(t, "Expenses:Power", focused.GetText(), "next statement row learns the just-saved category")
	key(tcell.KeyEscape, 0)
	key(tcell.KeyRight, 0)
	require.Contains(t, key(tcell.KeyEnter, 0), "1/3 recorded")
	require.Equal(t, saved, read(t, books.Config.WriteJournal), "deferring the next row must not post it")
	app.Stop()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("terminal did not exit")
	}
}
