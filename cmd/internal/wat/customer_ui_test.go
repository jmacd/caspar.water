package wat

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/stretchr/testify/require"
)

func TestCustomerFormsSaveAndCancelDirectly(t *testing.T) {
	books := customerFixture(t)
	_, key := customerTestTerminal(t, books, func(ui *terminal) { ui.customers() })
	typeText := func(text string) {
		for _, r := range text {
			key(tcell.KeyRune, r)
		}
	}
	before := read(t, books.Config.Registry)
	key(tcell.KeyRune, 'n')
	typeText("Discarded house")
	key(tcell.KeyEscape, 0)
	key(tcell.KeyTab, 0)
	key(tcell.KeyEnter, 0)
	require.Equal(t, before, read(t, books.Config.Registry))

	key(tcell.KeyRune, 'n')
	typeText("Second house")
	key(tcell.KeyTab, 0)
	typeText("200 Example Street; Exampletown")
	frame := key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "Saved CN-0002")
	registry, err := books.Customers()
	require.NoError(t, err)
	require.Len(t, registry.Connections, 2)
	require.Equal(t, []string{"200 Example Street", "Exampletown"}, registry.Connections[1].ServiceAddress.Lines)
	require.Empty(t, read(t, books.Config.ManualJournal))

	key(tcell.KeyEnter, 0)
	key(tcell.KeyRune, 'o')
	key(tcell.KeyEnter, 0)
	key(tcell.KeyDown, 0)
	key(tcell.KeyEnter, 0)
	key(tcell.KeyTab, 0)
	key(tcell.KeyEnter, 0)
	key(tcell.KeyDown, 0)
	key(tcell.KeyDown, 0)
	key(tcell.KeyEnter, 0)
	key(tcell.KeyTab, 0)
	typeText("2021-04-01")
	frame = key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "Saved SA-0002")
	registry, err = books.Customers()
	require.NoError(t, err)
	require.Len(t, registry.ServiceAccounts, 2)
	require.Equal(t, "CN-0002", registry.ServiceAccounts[1].Connection)
	require.Contains(t, read(t, books.Config.LocalAccounts), "Asset:141.000:Customers:SA-0002")
}

func TestServiceChooserRevisesOnlySelectedPosting(t *testing.T) {
	books := customerFixture(t)
	id := saveCustomerEntry(t, books, "2021-04-01", "Deposit", "Payment", []Posting{
		{"Bank:131.200:Checking", "100.00"}, {"Equity:218.000:Owner", "-100.00"},
	})
	original := read(t, books.Config.ManualJournal)
	draft, entry, err := books.EditDraft(id)
	require.NoError(t, err)
	require.Contains(t, draft.accountSearch["Asset:141.000:Customers:SA-0001"], "Example Customer")
	matches := matchingAccountsWithDesignations(draft.accounts, draft.accountSearch, "Example Customer")
	require.Contains(t, matches, "Asset:141.000:Customers:SA-0001")

	first, key := customerTestTerminal(t, books, func(ui *terminal) {
		ui.entryForm(draft, entry, ui.menu, 7)
	})
	require.Equal(t, "Equity:218.000:Owner", first.value)
	frame := key(tcell.KeyF3, 0)
	require.Contains(t, frame.text, "Select service account and accounting role")
	require.Contains(t, frame.text, "Example Customer")
	frame = key(tcell.KeyEnter, 0)
	require.Equal(t, "Asset:141.000:Customers:SA-0001", frame.value)
	require.Equal(t, original, read(t, books.Config.ManualJournal), "choosing a service account must not save")
	frame = key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "Transaction saved and validated")
	require.Equal(t, 1, strings.Count(read(t, books.Config.ManualJournal), "id: "+id))
	transactions, err := books.transactions(books.Config.Journal)
	require.NoError(t, err)
	require.Len(t, transactions, 1)
	bank, err := bankAmount(transactions[0], "Bank:131.200:Checking")
	require.NoError(t, err)
	require.Equal(t, int64(10000), bank)
	receivable, err := bankAmount(transactions[0], "Asset:141.000:Customers:SA-0001")
	require.NoError(t, err)
	require.Equal(t, int64(-10000), receivable)
}

func customerTestTerminal(t *testing.T, books *Books, start func(*terminal)) (terminalFrame, func(tcell.Key, rune) terminalFrame) {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	app := tview.NewApplication().SetScreen(screen)
	screen.SetSize(140, 40)
	ui := &terminal{app: app, books: books}
	frames := make(chan terminalFrame, 100)
	app.SetAfterDrawFunc(func(_ tcell.Screen) {
		var text strings.Builder
		width, height := screen.Size()
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, _, _, _ := screen.GetContent(x, y)
				text.WriteRune(r)
			}
			text.WriteByte('\n')
		}
		frames <- terminalFrame{text: text.String()}
	})
	start(ui)
	done := make(chan error, 1)
	go func() { done <- app.Run() }()
	t.Cleanup(func() {
		app.Stop()
		select {
		case err := <-done:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("terminal did not stop")
		}
	})
	next := func() terminalFrame {
		t.Helper()
		select {
		case frame := <-frames:
			if field, ok := app.GetFocus().(*tview.InputField); ok {
				frame.value = field.GetText()
			}
			return frame
		case <-time.After(5 * time.Second):
			t.Fatal("terminal did not draw")
			return terminalFrame{}
		}
	}
	key := func(key tcell.Key, r rune) terminalFrame {
		app.QueueEvent(tcell.NewEventKey(key, r, tcell.ModNone))
		return next()
	}
	return next(), key
}
