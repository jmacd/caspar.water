package wat

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *terminal) reconciliation() {
	ui.reconciliationAt(0)
}

func (ui *terminal) reconciliationAt(selected int) {
	value, err := ui.books.Reconciliation(ui.statement)
	if err != nil {
		ui.message(" Cannot reconcile statement ", err.Error(), ui.statementList)
		return
	}
	if value.Checkpoint != nil {
		ui.message(" Reconciliation finished ", "This statement is already reconciled and read-only.", ui.statementList)
		return
	}
	summary := tview.NewTextView().SetDynamicColors(false)
	summary.SetText(fmt.Sprintf(
		" Statement opening:       %12s\n"+
			" Statement credits:       %12s\n"+
			" Statement debits:        %12s\n"+
			" Statement closing:       %12s\n"+
			"\n"+
			" Book opening:            %12s\n"+
			" Book credits:            %12s\n"+
			" Book debits:             %12s\n"+
			" Book closing:            %12s\n"+
			"\n"+
			" Outstanding deposits:    %12s\n"+
			" Outstanding payments:    %12s\n"+
			" Adjusted statement:      %12s\n"+
			" Unexplained difference:  %12s\n"+
			"\n Outstanding items are existing book entries not yet cleared by the bank; no transaction is created.",
		formatCents(value.Statement.OpeningCents), formatCents(value.StatementCredits),
		formatCents(value.StatementDebits), formatCents(value.Statement.ClosingCents),
		formatCents(value.BookOpening), formatCents(value.BookCredits),
		formatCents(value.BookDebits), formatCents(value.BookClosing),
		formatCents(value.OutstandingCredit), formatCents(value.OutstandingDebit),
		formatCents(value.AdjustedStatement), formatCents(value.Difference)))
	list := tview.NewList().ShowSecondaryText(true)
	for _, item := range value.Items {
		state := "UNEXPLAINED"
		hint := "Review or edit it from the journal; o marks an existing genuine outstanding item"
		if item.Outstanding {
			state = "OUTSTANDING"
			hint = "No transaction was created; o removes the outstanding classification"
		}
		list.AddItem(tview.Escape(fmt.Sprintf("%s | %s | %s | %s", state, item.Date, item.Description, formatCents(item.Cents))),
			hint, 0, nil)
	}
	finishIndex := list.GetItemCount()
	finishHint := "Enabled only when every row is recorded and unexplained difference is zero"
	if value.RowsRecorded == len(value.Statement.Rows) && value.Difference == 0 {
		finishHint = "Persist checkpoint and make reconciled transactions read-only"
	}
	list.AddItem("Finish reconciliation", finishHint, 'f', func() {
		if value.RowsRecorded != len(value.Statement.Rows) {
			ui.message(" Cannot finish ", "Record every statement row before Finish.", func() { ui.reconciliationAt(finishIndex) })
			return
		}
		if value.Difference != 0 {
			ui.message(" Cannot finish ", "The unexplained difference must be zero before Finish.", func() { ui.reconciliationAt(finishIndex) })
			return
		}
		modal := tview.NewModal().
			SetText("Finish this reconciliation?\n\nThe statement transactions will become read-only in wat.").
			AddButtons([]string{"Cancel", "Finish"})
		modal.SetDoneFunc(func(index int, _ string) {
			if index != 1 {
				ui.reconciliationAt(finishIndex)
				return
			}
			if err := ui.books.FinishReconciliation(ui.statement); err != nil {
				ui.message(" Cannot finish reconciliation ", err.Error(), ui.reconciliation)
				return
			}
			ui.message(" Reconciliation finished ",
				"The checkpoint was saved. Cleared statement transactions are now read-only.", ui.statementList)
		})
		ui.app.SetRoot(modal, true).SetFocus(modal)
	})
	list.AddItem("Back to statement", "Outstanding selections are saved; no checkpoint is created", 'q', ui.statementList)
	if selected >= list.GetItemCount() {
		selected = list.GetItemCount() - 1
	}
	list.SetCurrentItem(selected)
	list.SetBorder(true).SetTitle(" Unmatched bank entries ")
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.statementList()
			return nil
		}
		if event.Rune() == 'o' && list.GetCurrentItem() < len(value.Items) {
			index := list.GetCurrentItem()
			item := value.Items[index]
			if err := ui.books.SetOutstanding(ui.statement, item.ID, !item.Outstanding); err != nil {
				ui.message(" Cannot update reconciliation ", err.Error(), func() { ui.reconciliationAt(index) })
				return nil
			}
			ui.reconciliationAt(index)
			return nil
		}
		return event
	})
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(summary, 16, 0, false).
		AddItem(list, 0, 1, true)
	ui.screen("Reconcile | "+value.Statement.Period.Start+" to "+value.Statement.Period.End, body,
		"o Toggle existing outstanding item | f Finish | Esc Back")
	ui.app.SetFocus(list)
}
