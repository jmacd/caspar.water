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

func TestServiceLifecycleFormsCloseConfirmAndIssue(t *testing.T) {
	books := customerFixture(t)
	before := read(t, books.Config.Registry)
	_, key := customerTestTerminal(t, books, func(ui *terminal) { ui.customerDetails("service", "SA-0001") })
	typeText := func(text string) {
		for _, r := range text {
			key(tcell.KeyRune, r)
		}
	}
	key(tcell.KeyRune, 'c')
	typeText("2021-01-31")
	frame := key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "Saved SA-0001")
	require.NotEqual(t, before, read(t, books.Config.Registry))
	require.Empty(t, read(t, books.Config.ManualJournal))
	key(tcell.KeyEnter, 0)
	// Menu entries: three setup actions, customer, connection, then this service.
	for i := 0; i < 5; i++ {
		key(tcell.KeyDown, 0)
	}
	key(tcell.KeyEnter, 0)
	key(tcell.KeyRune, 'b')
	frame = key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "final billing requires", "empty review must not confirm billing")
	key(tcell.KeyEnter, 0)
	key(tcell.KeyTab, 0)
	key(tcell.KeyTab, 0)
	typeText("Reviewed closure; no unbilled service or obligations")
	frame = key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "Saved SA-0001", "failed saves must allow corrected retry")
	key(tcell.KeyEnter, 0)
	for i := 0; i < 5; i++ {
		key(tcell.KeyDown, 0)
	}
	key(tcell.KeyEnter, 0)
	key(tcell.KeyRune, 'r')
	key(tcell.KeyTab, 0)
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, ": SETTLED")
	key(tcell.KeyEscape, 0)
	key(tcell.KeyEscape, 0)
	key(tcell.KeyRune, 's')
	for i := 0; i < 3; i++ {
		key(tcell.KeyTab, 0)
	}
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Statement issued")
	require.FileExists(t, filepath.Join(books.Config.Sources, "customer-statements", "SA-0001", "statement-0001.txt"))
	require.Empty(t, read(t, books.Config.ManualJournal))
}

func TestServiceAdjustmentFormRequiresPreviewBeforeSave(t *testing.T) {
	books := customerFixture(t)
	closeTestService(t, books)
	_, key := customerTestTerminal(t, books, func(ui *terminal) { ui.serviceTransactionForm("SA-0001", "adjust") })
	typeText := func(text string) {
		for _, r := range text {
			key(tcell.KeyRune, r)
		}
	}
	key(tcell.KeyTab, 0)
	typeText("10")
	key(tcell.KeyTab, 0)
	typeText("Approved final service charge")
	key(tcell.KeyTab, 0)
	typeText("2021-01-31")
	key(tcell.KeyTab, 0)
	typeText(serviceIncome)
	key(tcell.KeyTab, 0)
	frame := key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Preview - NOT SAVED")
	require.Contains(t, frame.text, "service-through: 2021-01-31")
	require.Empty(t, read(t, books.Config.ManualJournal))
	frame = key(tcell.KeyEnter, 0)
	require.Contains(t, frame.text, "Transaction saved and validated")
	data, err := os.ReadFile(books.Config.ManualJournal)
	require.NoError(t, err)
	require.Contains(t, string(data), "Approved final service charge")
	require.Equal(t, 1, strings.Count(string(data), "; id:"))
}

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

func TestFirstConnectionFormSavesWithLegacyJournalDescription(t *testing.T) {
	books, _ := fixture(t, legacyCustomerJournal)
	books = enableTestCustomers(t, books)
	_, key := customerTestTerminal(t, books, func(ui *terminal) { ui.customers() })
	key(tcell.KeyRune, 'n')
	for _, r := range "First house" {
		key(tcell.KeyRune, r)
	}
	key(tcell.KeyTab, 0)
	for _, r := range "100 Example Street" {
		key(tcell.KeyRune, r)
	}
	frame := key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "Saved CN-0001")
	registry, err := books.Customers()
	require.NoError(t, err)
	require.Len(t, registry.Connections, 1)
	require.Equal(t, "First house", registry.Connections[0].Name)
	require.Equal(t, legacyCustomerJournal, read(t, books.Config.ManualJournal))
}

func TestCustomerFormCreatesNameWithoutManualPayee(t *testing.T) {
	books, _ := fixture(t, "")
	configurePayees(t, books)
	books = enableTestCustomers(t, books)
	_, key := customerTestTerminal(t, books, func(ui *terminal) { ui.customers() })
	key(tcell.KeyRune, 'a')
	for _, r := range "New Customer" {
		key(tcell.KeyRune, r)
	}
	key(tcell.KeyTab, 0)
	for _, r := range "100 Example Street" {
		key(tcell.KeyRune, r)
	}
	frame := key(tcell.KeyF2, 0)
	require.Contains(t, frame.text, "Saved C-0001")
	registry, err := books.Customers()
	require.NoError(t, err)
	require.Len(t, registry.Customers, 1)
	require.Equal(t, "New Customer", registry.Customers[0].Payee)
	require.Contains(t, read(t, books.Config.CustomerPayees), "payee New Customer\n")
	require.Empty(t, read(t, books.Config.Payees))
	require.Empty(t, read(t, books.Config.ManualJournal))
	require.NoError(t, books.Check())
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
