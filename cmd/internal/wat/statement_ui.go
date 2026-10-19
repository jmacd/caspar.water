package wat

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func (ui *terminal) bankStatements() {
	statements, err := ui.books.CachedStatements()
	if err != nil {
		ui.message(" Cannot list bank statements ", err.Error(), ui.menu)
		return
	}
	pending, err := ui.books.PendingStatements()
	if err != nil {
		ui.message(" Cannot list pending bank statements ", err.Error(), ui.menu)
		return
	}
	if len(statements) == 0 && len(pending) == 0 {
		ui.message(" Bank statements ", "No bank statements are available. Add PDF files to the configured statement inbox, then start wat again.", ui.menu)
		return
	}
	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(fmt.Sprintf(
		" Bank statements | %d extracted | %d ready - recorded does not mean reconciled ",
		len(statements), len(pending),
	))
	for _, statement := range statements {
		statuses, err := ui.books.StatementStatuses(statement)
		if err != nil {
			ui.message(" Cannot review bank statement ", statement.Source+"\n\n"+err.Error(), ui.menu)
			return
		}
		recorded := 0
		for _, status := range statuses {
			if status.Recorded {
				recorded++
			}
		}
		checkpoint, err := ui.books.loadCheckpoint(statement)
		if err != nil {
			ui.message(" Cannot review bank statement ", statement.Source+"\n\n"+err.Error(), ui.menu)
			return
		}
		period := "Statement dates unavailable"
		if statement.Period != nil {
			period = statement.Period.Start + " to " + statement.Period.End
		}
		state := fmt.Sprintf("%d recorded, %d pending", recorded, len(statuses)-recorded)
		hint := statement.Source
		if checkpoint != nil {
			state = "RECONCILED"
			hint = "Finished " + checkpoint.FinishedAt + " | read-only | " + statement.Source
		}
		label := fmt.Sprintf("EXTRACTED | %s | %s | %s | %s", statement.BankAccount, period, state, statement.Hash[:12])
		list.AddItem(tview.Escape(label), tview.Escape(hint), 0, func() {
			current, err := ui.books.cachedStatement(statement.Hash)
			if err != nil {
				ui.message(" Cannot open bank statement ", err.Error(), ui.bankStatements)
				return
			}
			ui.openStatementFromBrowser(current)
		})
	}
	for _, statement := range pending {
		label := fmt.Sprintf("READY TO EXTRACT | %s | %s", statement.Name, statement.Hash[:12])
		hint := "Enter to extract this complete PDF"
		list.AddItem(tview.Escape(label), tview.Escape(hint), 0, func() {
			ui.extractPendingStatement(statement)
		})
	}
	list.AddItem("Back to menu", "Saved entries and statement caches are unchanged", 'q', ui.menu)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.menu()
			return nil
		}
		return event
	})
	ui.screen("Bank statements", list, "Enter Open / extract one statement | Esc Menu")
}

func (ui *terminal) openStatementFromBrowser(statement *Statement) {
	previous, queue, back := ui.statement, ui.queue, ui.statementBack
	ui.statement, ui.queue = statement, nil
	ui.statementBack = func() {
		ui.statement, ui.queue, ui.statementBack = previous, queue, back
		ui.bankStatements()
	}
	ui.statementList()
}

func (ui *terminal) extractPendingStatement(pending *PendingStatement) {
	frames := []string{"|", "/", "-", "\\"}
	started := time.Now()
	status := tview.NewTextView().SetText(statementExtractionStatus(
		pending.Name, ui.books.Config.Statement.Model, frames[0], 0,
	))
	ui.screen("Statement extraction", status, "Extraction in progress")
	var progress bytes.Buffer
	result := make(chan statementExtractionResult, 1)
	go func() {
		statement, err := ui.books.ExtractPendingStatement(pending.Hash, StatementOptions{}, &progress)
		result <- statementExtractionResult{statement: statement, err: err}
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	go func() {
		defer ticker.Stop()
		frame := 0
		for {
			select {
			case outcome := <-result:
				ui.app.QueueUpdateDraw(func() {
					if outcome.err != nil {
						detail := strings.TrimSpace(progress.String())
						if detail != "" {
							detail += "\n\n"
						}
						detail += outcome.err.Error()
						ui.message(" Statement extraction failed ", detail, ui.bankStatements)
						return
					}
					ui.openStatementFromBrowser(outcome.statement)
				})
				return
			case now := <-ticker.C:
				frame = (frame + 1) % len(frames)
				text := statementExtractionStatus(
					pending.Name, ui.books.Config.Statement.Model, frames[frame], now.Sub(started),
				)
				ui.app.QueueUpdateDraw(func() {
					status.SetText(text)
				})
			}
		}
	}()
}

type statementExtractionResult struct {
	statement *Statement
	err       error
}

func statementExtractionStatus(name, model, frame string, elapsed time.Duration) string {
	totalSeconds := int(elapsed / time.Second)
	return fmt.Sprintf(
		"Extracting %s\n\n[%s] Reading embedded PDF text and waiting for model %s\n    Elapsed %02d:%02d\n\nNo journal entries will be written.",
		name, frame, model, totalSeconds/60, totalSeconds%60,
	)
}

func (ui *terminal) statementList() {
	back := ui.menu
	backLabel := "Back to menu"
	destination := "Menu"
	if ui.statementBack != nil {
		back = ui.statementBack
		backLabel = "Back to statements"
		destination = "Statements"
	}
	statuses, err := ui.books.StatementStatuses(ui.statement)
	if err != nil {
		ui.message(" Statement review failed ", err.Error(), back)
		return
	}
	checkpoint, err := ui.books.loadCheckpoint(ui.statement)
	if err != nil {
		ui.message(" Statement review failed ", err.Error(), back)
		return
	}
	list := tview.NewList().ShowSecondaryText(true)
	footer := "Enter Select | i Source/notices | Esc " + destination + " | Only Save records an entry"
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
		if checkpoint != nil {
			label = "RECONCILED"
			hint = "Finished reconciliation; transaction is read-only"
		}
		list.AddItem(tview.Escape(fmt.Sprintf("%s | %s | %s | %s USD", label, row.Date, row.Description, formatCents(row.Cents))),
			tview.Escape(hint), 0, func() { ui.statementOpen(index) })
	}
	if checkpoint == nil && recorded == len(statuses) {
		if _, err := ui.books.Reconciliation(ui.statement); err != nil {
			list.AddItem("Reconciliation blocked", err.Error(), 0, func() {
				ui.message(" Cannot begin reconciliation ", err.Error(), ui.statementList)
			})
		} else {
			list.AddItem("Reconcile statement", "Compare the complete bank register; Finish locks cleared transactions", 'r', ui.reconciliation)
			footer = "Enter/r Reconcile | i Source/notices | Esc " + destination
		}
		list.SetCurrentItem(len(ui.statement.Rows))
	}
	if ui.queue != nil && checkpoint != nil {
		label := "Next statement"
		if ui.queue.position == ui.queue.total {
			label = "Exit statement batch"
		}
		list.AddItem(label, "This statement is reconciled and read-only", 'n', func() {
			ui.queue.advance = true
			ui.app.Stop()
		})
		list.SetCurrentItem(len(ui.statement.Rows))
		footer = "Enter/n " + label + " | i Source/notices | Esc Menu"
	}
	list.AddItem("Statement notices / source", "Extraction warnings and PDF location; balance checks are not proof of reconciliation", 'i', func() {
		period := "Statement dates unavailable"
		if ui.statement.Period != nil {
			period = ui.statement.Period.Start + " to " + ui.statement.Period.End
		}
		message := "Statement period: " + period + "\nSource PDF: " + ui.statement.Source + "\n\n" + strings.Join(ui.statement.Warnings, "\n") +
			"\n\nCompare extracted rows with the PDF. Matching opening/closing balances alone does not establish completeness or reconciliation."
		ui.message(" Statement source ", message, ui.statementList)
	})
	list.AddItem(backLabel, "Saved rows remain recorded; pending rows can be resumed", 'q', back)
	state := fmt.Sprintf("%d/%d recorded", recorded, len(statuses))
	if checkpoint != nil {
		state = "RECONCILED - read-only"
	}
	list.SetBorder(true).SetTitle(fmt.Sprintf(" Bank statement | %s | %d notices (i) ", state, len(ui.statement.Warnings)))
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			back()
			return nil
		}
		return event
	})
	title := "Bank statement review"
	if ui.statement.Period != nil {
		title += " | " + ui.statement.Period.Start + " to " + ui.statement.Period.End
	}
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
	checkpoint, err := ui.books.loadCheckpoint(ui.statement)
	if err != nil {
		ui.message(" Cannot open statement row ", err.Error(), ui.statementList)
		return
	}
	if checkpoint != nil {
		ui.message(" Reconciled transaction ", fmt.Sprintf(
			"This transaction is read-only because statement %s to %s was finished at %s.",
			checkpoint.Period.Start, checkpoint.Period.End, checkpoint.FinishedAt), ui.statementList)
		return
	}
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
			if posting.Account != ui.statement.BankAccount {
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
