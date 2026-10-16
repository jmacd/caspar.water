package wat

import (
	"fmt"

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
	list := tview.NewList().ShowSecondaryText(true)
	for index, entry := range entries {
		label := "WORKING"
		hint := "Enter to revise this existing transaction"
		if entry.ReadOnly != "" {
			label, hint = "READ-ONLY", entry.ReadOnly
		}
		list.AddItem(tview.Escape(fmt.Sprintf("%s | %s | %s", label, entry.Date, entry.Description)),
			tview.Escape(hint), 0, func() {
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
		if selectedID != "" && entry.ID == selectedID {
			list.SetCurrentItem(index)
		}
	}
	list.AddItem("Back to menu", "Saved revisions replace the existing transaction; they do not append another", 'q', ui.menu)
	list.SetBorder(true).SetTitle(" Journal | working entries editable; other included journals read-only ")
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.menu()
			return nil
		}
		return event
	})
	ui.screen("Review / revise journal", list, "Enter Revise entry | Esc/q Menu | Only working journal entries can be revised")
}
