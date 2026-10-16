package wat

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *terminal) statementList() {
	statuses, err := ui.books.StatementStatuses(ui.statement)
	if err != nil {
		ui.message(" Statement review failed ", err.Error(), ui.menu)
		return
	}
	list := tview.NewList().ShowSecondaryText(true)
	footer := "Enter Select | i Source/notices | Esc Menu | Only Save records an entry"
	recorded := 0
	for index, row := range ui.statement.Rows {
		status := statuses[index]
		label := "PENDING"
		if status.Recorded {
			label = "RECORDED"
			recorded++
		} else if status.PossibleMatch {
			label = "POSSIBLE MATCH"
		}
		hint := "Suggested counteraccount: " + status.Suggestion
		if status.Recorded {
			hint = "Enter to revise the existing entry (working journal only)"
		}
		list.AddItem(tview.Escape(fmt.Sprintf("%s | %s | %s | %s USD", label, row.Date, row.Description, formatCents(row.Cents))),
			tview.Escape(hint), 0, func() { ui.statementOpen(index) })
	}
	if ui.queue != nil && recorded == len(statuses) {
		label := "Next statement"
		if ui.queue.position == ui.queue.total {
			label = "Finish statement batch"
		}
		list.AddItem(label, "All rows recorded; this does not certify reconciliation", 'n', func() {
			ui.queue.advance = true
			ui.app.Stop()
		})
		list.SetCurrentItem(len(ui.statement.Rows))
		footer = "Enter/n " + label + " | i Source/notices | Esc Menu"
	}
	list.AddItem("Statement notices / source", "Extraction warnings and PDF location; balance checks are not proof of reconciliation", 'i', func() {
		message := "Source PDF: " + ui.statement.Source + "\n\n" + strings.Join(ui.statement.Warnings, "\n") +
			"\n\nCompare extracted rows with the PDF. Matching opening/closing balances alone does not establish completeness or reconciliation."
		ui.message(" Statement source ", message, ui.statementList)
	})
	list.AddItem("Back to menu", "Saved rows remain recorded; pending rows can be resumed", 'q', ui.menu)
	list.SetBorder(true).SetTitle(fmt.Sprintf(" Bank statement | %d/%d recorded | %d notices (i) ", recorded, len(statuses), len(ui.statement.Warnings)))
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.menu()
			return nil
		}
		return event
	})
	title := "Bank statement review"
	if ui.queue != nil {
		title = fmt.Sprintf("Statement %d/%d | %s", ui.queue.position, ui.queue.total, ui.queue.path)
	}
	ui.screen(title, list, footer)
}

func (ui *terminal) statementNext(previous int) {
	statuses, err := ui.books.StatementStatuses(ui.statement)
	if err != nil {
		ui.message(" Statement review failed ", err.Error(), ui.menu)
		return
	}
	for offset := 1; offset <= len(statuses); offset++ {
		index := (previous + offset) % len(statuses)
		if !statuses[index].Recorded {
			ui.statementOpen(index)
			return
		}
	}
	ui.statementList()
}

func (ui *terminal) statementOpen(index int) {
	statuses, err := ui.books.StatementStatuses(ui.statement)
	if err != nil {
		ui.message(" Statement review failed ", err.Error(), ui.statementList)
		return
	}
	if statuses[index].Recorded {
		draft, entry, err := ui.books.EditStatementDraft(ui.statement, index)
		if err != nil {
			ui.message(" Cannot revise recorded row ", err.Error(), ui.statementList)
			return
		}
		focus := 5
		for index, posting := range entry.Postings {
			if posting.Account != ui.books.Config.BankAccount {
				focus = 5 + 2*index
				break
			}
		}
		ui.entryForm(draft, entry, ui.statementList, focus)
		return
	}
	open := func() {
		draft, entry, err := ui.books.StatementDraft(ui.statement, index)
		if err != nil {
			ui.message(" Cannot start statement row ", err.Error(), ui.statementList)
			return
		}
		ui.entryForm(draft, entry, ui.statementList, 7)
	}
	if statuses[index].PossibleMatch {
		modal := tview.NewModal().SetText("The journal already has a bank posting with this date and amount. It may be this transaction. Check the existing entry before recording another. Back leaves this row pending.").
			AddButtons([]string{"Back", "Record separate transaction"})
		modal.SetDoneFunc(func(index int, _ string) {
			if index == 1 {
				open()
			} else {
				ui.statementList()
			}
		})
		ui.app.SetRoot(modal, true).SetFocus(modal)
		return
	}
	open()
}
