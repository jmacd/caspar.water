package wat

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *terminal) journalEntries() {
	ui.journalEntryList("")
}

func (ui *terminal) journalEntryList(selectedID string) {
	entries, err := ui.books.JournalEntries()
	if err != nil {
		ui.message(" Journal review failed ", err.Error(), ui.menu)
		return
	}
	table := tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	headers := []struct {
		text      string
		align     int
		expansion int
	}{
		{"State", tview.AlignLeft, 0},
		{"Date", tview.AlignLeft, 0},
		{"Description", tview.AlignLeft, 1},
	}
	if ui.books.Config.BankAccount != "" {
		headers = append(headers,
			struct {
				text      string
				align     int
				expansion int
			}{"Bank change", tview.AlignRight, 0},
			struct {
				text      string
				align     int
				expansion int
			}{"Bank balance", tview.AlignRight, 0},
		)
	}
	for column, header := range headers {
		table.SetCell(0, column, tview.NewTableCell(header.text).
			SetAlign(header.align).SetExpansion(header.expansion).
			SetAttributes(tcell.AttrBold).SetSelectable(false))
	}
	selectedRow := 1
	for index, entry := range entries {
		state := "WORKING"
		if entry.ReadOnly != "" {
			state = "READ-ONLY"
		}
		row := index + 1
		table.SetCell(row, 0, tview.NewTableCell(state).SetReference(entry.ID))
		table.SetCell(row, 1, tview.NewTableCell(entry.Date))
		table.SetCell(row, 2, tview.NewTableCell(entry.Description).SetExpansion(1))
		if ui.books.Config.BankAccount != "" {
			table.SetCell(row, 3, tview.NewTableCell(formatCents(entry.BankChange)).SetAlign(tview.AlignRight))
			table.SetCell(row, 4, tview.NewTableCell(formatCents(entry.BankBalance)).SetAlign(tview.AlignRight))
		}
		if selectedID != "" && entry.ID == selectedID {
			selectedRow = row
		}
	}
	table.Select(selectedRow, 0)
	table.SetSelectedFunc(func(row, _ int) {
		if row < 1 || row > len(entries) {
			return
		}
		entry := entries[row-1]
		back := func() { ui.journalEntryList(entry.ID) }
		if entry.ReadOnly != "" {
			ui.message(" Read-only entry ", entry.ReadOnly, back)
			return
		}
		draft, value, err := ui.books.EditDraft(entry.ID)
		if err != nil {
			ui.message(" Cannot revise entry ", err.Error(), back)
			return
		}
		ui.entryForm(draft, value, back, 0)
	})
	table.SetBorder(true).SetTitle(" Journal | one row per transaction ")
	table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape || event.Rune() == 'q' {
			ui.menu()
			return nil
		}
		return event
	})
	footer := "Up/Down Select | Enter Revise | Esc/q Menu"
	if ui.books.Config.BankAccount != "" {
		footer += "\n Bank columns: " + ui.books.Config.BankAccount
	}
	ui.screen("Review / revise journal", table, footer)
}
