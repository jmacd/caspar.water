package wat

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type entryControl struct {
	primitive tview.Primitive
	row, col  int
	input     bool
}

type entryGrid struct {
	*tview.Flex
	controls []entryControl
	balance  *tview.TextView
}

type entryInputGrid struct {
	*tview.Grid
}

type entryInputScreen struct {
	tcell.Screen
	bottom int
}

func (screen entryInputScreen) Size() (int, int) {
	width, _ := screen.Screen.Size()
	return width, screen.bottom
}

func (grid *entryInputGrid) Draw(screen tcell.Screen) {
	_, y, _, height := grid.GetInnerRect()
	// Autocomplete must fit above the pinned balance and action bar.
	grid.Grid.Draw(entryInputScreen{Screen: screen, bottom: y + height})
}

func newEntryGrid(app *tview.Application, entry *Entry, draft *Draft, focus *int, add, remove, save, discard func()) *entryGrid {
	grid := &entryGrid{
		Flex:    tview.NewFlex().SetDirection(tview.FlexRow),
		balance: tview.NewTextView(),
	}
	inputs := &entryInputGrid{Grid: tview.NewGrid().SetColumns(4, -1, 24).SetGap(0, 1)}
	updateBalance := func() { grid.balance.SetText(entry.balanceStatus()) }
	updateBalance()
	grid.SetBorder(true).SetTitle(" New transaction - UNSAVED ")
	rows := make([]int, 7+len(entry.Postings))
	for i := range rows {
		rows[i] = 1
	}
	inputs.SetRows(rows...)
	register := func(primitive tview.Primitive, box *tview.Box, row, col int, input bool) {
		index := len(grid.controls)
		grid.controls = append(grid.controls, entryControl{primitive, row, col, input})
		box.SetFocusFunc(func() {
			*focus = index
			if field, ok := primitive.(*tview.InputField); ok {
				field.Autocomplete()
			}
		})
	}
	metadata := []struct {
		label string
		value *string
	}{
		{"Date (YYYY-MM-DD)", &entry.Date},
		{"Payee", &entry.Payee},
		{"Description", &entry.Description},
		{"Reference", &entry.Reference},
		{"Source reference", &entry.Source},
	}
	for row, item := range metadata {
		field := tview.NewInputField().SetLabel(item.label).SetLabelWidth(20).
			SetText(*item.value).SetChangedFunc(func(text string) { *item.value = text })
		switch row {
		case 1:
			completeInput(field, func(query string) []string { return matchingText(draft.history.payees, query) })
		case 2:
			completeInput(field, func(query string) []string { return matchingText(draft.history.descriptions, query) })
		}
		register(field, field.Box, row, 0, true)
		inputs.AddItem(field, row, 0, 1, 3, 0, 0, row == 0)
	}
	inputs.AddItem(tview.NewTextView().SetText("#"), 6, 0, 1, 1, 0, 0, false)
	inputs.AddItem(tview.NewTextView().SetText("Account"), 6, 1, 1, 1, 0, 0, false)
	inputs.AddItem(tview.NewTextView().SetText("USD (+ debit / - credit)"), 6, 2, 1, 1, 0, 0, false)
	for i := range entry.Postings {
		posting := &entry.Postings[i]
		row := 7 + i
		account := tview.NewInputField().SetText(posting.Account).
			SetPlaceholder("Account code or name").
			SetChangedFunc(func(text string) { posting.Account = text })
		completeInput(account, func(query string) []string { return matchingAccounts(draft.accounts, query) })
		amount := tview.NewInputField().SetText(posting.Amount).
			SetPlaceholder("0.00 or -0.00").
			SetChangedFunc(func(text string) {
				posting.Amount = text
				updateBalance()
			})
		inputs.AddItem(tview.NewTextView().SetText(fmt.Sprint(i+1)), row, 0, 1, 1, 0, 0, false)
		inputs.AddItem(account, row, 1, 1, 1, 0, 0, false)
		inputs.AddItem(amount, row, 2, 1, 1, 0, 0, false)
		register(account, account.Box, row, 0, true)
		register(amount, amount.Box, row, 1, true)
	}
	buttons := tview.NewFlex()
	for _, action := range []struct {
		label string
		run   func()
	}{
		{"Save (F2)", save},
		{"Add posting", add},
		{"Remove last posting", remove},
		{"Back (Esc)", discard},
	} {
		button := tview.NewButton(action.label).SetSelectedFunc(action.run)
		button.SetDisabled(action.label == "Remove last posting" && len(entry.Postings) <= 2)
		buttons.AddItem(button, 0, 1, false)
		register(button, button.Box, 8+len(entry.Postings), 0, false)
	}
	grid.AddItem(inputs, 0, 1, true).
		AddItem(grid.balance, 2, 0, false).
		AddItem(buttons, 1, 0, false)
	step := func(delta int) {
		for index := (*focus + delta + len(grid.controls)) % len(grid.controls); index != *focus; index = (index + delta + len(grid.controls)) % len(grid.controls) {
			control := grid.controls[index]
			if button, ok := control.primitive.(*tview.Button); ok && button.IsDisabled() {
				continue
			}
			app.SetFocus(control.primitive)
			return
		}
	}
	for _, control := range grid.controls {
		if field, ok := control.primitive.(*tview.InputField); ok {
			field.SetDoneFunc(func(key tcell.Key) {
				if key == tcell.KeyEnter {
					step(1)
				}
			})
		}
	}
	grid.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyTab:
			step(1)
		case tcell.KeyBacktab:
			step(-1)
		case tcell.KeyUp, tcell.KeyDown:
			current := grid.controls[*focus]
			target := -1
			for index, candidate := range grid.controls {
				if !candidate.input ||
					(event.Key() == tcell.KeyUp && candidate.row >= current.row) ||
					(event.Key() == tcell.KeyDown && candidate.row <= current.row) {
					continue
				}
				if target < 0 ||
					(event.Key() == tcell.KeyUp && candidate.row > grid.controls[target].row) ||
					(event.Key() == tcell.KeyDown && candidate.row < grid.controls[target].row) ||
					(candidate.row == grid.controls[target].row && candidate.col == current.col) {
					target = index
				}
			}
			if target >= 0 {
				app.SetFocus(grid.controls[target].primitive)
			}
		default:
			return event
		}
		return nil
	})
	return grid
}
