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
			case *tview.DropDown:
				frame.focus = focus.GetLabel()
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
	require.Equal(t, "Account 1", frame.focus)
	frame = key(tcell.KeyDown, 0)
	require.Equal(t, "USD amount 1 (+debit/-credit)", frame.focus)
	frame = key(tcell.KeyUp, 0)
	require.Equal(t, "Account 1", frame.focus)
	key(tcell.KeyEnter, 0)
	key(tcell.KeyDown, 0)
	key(tcell.KeyDown, 0)
	key(tcell.KeyEnter, 0)
	key(tcell.KeyTab, 0)
	typeText("12.34")
	key(tcell.KeyTab, 0)
	key(tcell.KeyTab, 0)
	typeText("-12.34")
	for i := 0; i < 4; i++ {
		frame = key(tcell.KeyTab, 0)
		if frame.focus == "Preview" {
			break
		}
	}
	require.Equal(t, "Preview", frame.focus)
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "NOT SAVED")
	require.Contains(t, frame.text, "Expenses:Power")
	require.Contains(t, frame.text, "Assets:Bank")
	require.Equal(t, "; original text\n", read(t, books.Config.WriteJournal))
	frame = key(tcell.KeyF4, 0)
	require.Contains(t, frame.text, "Edited power")
	require.Contains(t, frame.text, "$-15.00")
	require.Equal(t, "; original text\n", read(t, books.Config.WriteJournal))
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Transaction saved and validated")
	saved := read(t, books.Config.WriteJournal)
	require.Contains(t, saved, "2021-04-05 Electric company | Edited power")
	require.Contains(t, saved, "Expenses:Power  $15.00")
	require.Contains(t, saved, "Assets:Bank  $-15.00")
	key(tcell.KeyEnter, 0)
	frame = key(tcell.KeyRune, '2')
	require.Contains(t, frame.text, "Electric company")
	key(tcell.KeyEscape, 0)
	key(tcell.KeyRune, '1')
	frame = key(tcell.KeyEscape, 0)
	require.Contains(t, frame.text, "Discard this unsaved transaction")
	key(tcell.KeyRight, 0)
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Water Accounting Terminal")
	require.Equal(t, saved, read(t, books.Config.WriteJournal))
	app.QueueEvent(tcell.NewEventKey(tcell.KeyRune, 'q', tcell.ModNone))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("terminal did not exit")
	}
}
