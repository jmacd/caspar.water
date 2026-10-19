package wat

import (
	"fmt"
	"io"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type terminal struct {
	app           *tview.Application
	books         *Books
	statement     *Statement
	queue         *statementQueue
	statementBack func()
	entryBack     func()
}

type statementQueue struct {
	position, total int
	path            string
	advance         bool
}

func RunTerminal(books *Books) error {
	return runTerminal(books, nil, nil)
}

func RunStatementTerminal(books *Books, statement *Statement) error {
	return runTerminal(books, statement, nil)
}

func RunStatementsTerminal(books *Books, paths []string, progress io.Writer) error {
	for index, path := range paths {
		statement, err := books.LoadStatement(path, StatementOptions{}, progress)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		queue := &statementQueue{position: index + 1, total: len(paths), path: path}
		if err := runTerminal(books, statement, queue); err != nil {
			return err
		}
		if !queue.advance {
			return nil
		}
	}
	return nil
}

func runTerminal(books *Books, statement *Statement, queue *statementQueue) error {
	tview.Styles.PrimitiveBackgroundColor = tcell.NewHexColor(0x101b2b)
	tview.Styles.ContrastBackgroundColor = tcell.NewHexColor(0x25466b)
	tview.Styles.BorderColor = tcell.NewHexColor(0x629ac2)
	tview.Styles.PrimaryTextColor = tcell.NewHexColor(0xe6edf3)
	ui := &terminal{app: tview.NewApplication(), books: books, statement: statement, queue: queue}
	ui.captureGlobalInput()
	if statement == nil {
		ui.menu()
	} else {
		ui.statementList()
	}
	return ui.app.Run()
}

func (ui *terminal) captureGlobalInput() {
	ui.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC {
			ui.app.Stop()
			return nil
		}
		return event
	})
}

func (ui *terminal) screen(title string, body tview.Primitive, footer string) {
	header := tview.NewTextView().SetDynamicColors(true)
	header.SetText(tview.Escape(fmt.Sprintf(" %s | %d | %s\n %s",
		ui.books.Config.Name, ui.books.Config.ReportYear, title, ui.books.Config.Journal)))
	hint := tview.NewTextView().SetText(" " + footer)
	frame := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 3, 0, false).
		AddItem(body, 0, 1, true).
		AddItem(hint, 2, 0, false)
	ui.app.SetRoot(frame, true).SetFocus(body)
}

func (ui *terminal) message(title, text string, back func()) {
	modal := tview.NewModal().SetText(tview.Escape(text)).AddButtons([]string{"OK"})
	modal.SetTitle(title).SetBorder(true)
	modal.SetDoneFunc(func(_ int, _ string) { back() })
	ui.app.SetRoot(modal, true).SetFocus(modal)
}

func (ui *terminal) menu() {
	list := tview.NewList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(" Water Accounting Terminal ")
	list.AddItem("Record a transaction", "Balanced postings with a live balance", '1', ui.entry).
		AddItem("Review / revise journal", "Browse entries; revise entries in the working journal", '2', ui.journalEntries).
		AddItem("Reports", "Trial balance, balance sheet, income statement", '3', ui.reports).
		AddItem("Check books", "Run hledger strict validation and CPUC account-mapping checks", '4', func() {
			if err := ui.books.Check(); err != nil {
				ui.message(" Check failed ", err.Error(), ui.menu)
				return
			}
			ui.message(" Check books ", "The books pass hledger strict checks and CPUC account-mapping checks.\nThis does not establish bank reconciliation or completeness.", ui.menu)
		}).
		AddItem("Bank statements", "Extract one pending inbox PDF or reopen a statement", '5', ui.bankStatements).
		AddItem("Quit", "Leave all saved journal files in place", 'q', ui.app.Stop)
	footer := "Enter Select | 1-5 Shortcut | q Quit"
	if ui.statement != nil {
		list.AddItem("Continue bank statement", "Return to the current statement review", '6', ui.statementList)
		footer = "Enter Select | 1-6 Shortcut | q Quit"
	}
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.app.Stop()
			return nil
		}
		return event
	})
	ui.screen("Menu", list, footer)
}

func (ui *terminal) reports() {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Reports ")
	for _, name := range []string{"journal", "trial-balance", "balance-sheet", "income-statement"} {
		name := name
		list.AddItem(name, "", 0, func() { ui.report(name) })
	}
	list.AddItem("Back", "", 'q', ui.menu)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.menu()
			return nil
		}
		return event
	})
	ui.screen("Reports", list, "Enter Select | Esc Back")
}

func (ui *terminal) report(name string) {
	text, err := ui.books.Report(name)
	if err != nil {
		ui.message(" Report failed ", err.Error(), ui.menu)
		return
	}
	view := tview.NewTextView().SetDynamicColors(false).SetWrap(false).SetText(text)
	view.SetBorder(true).SetTitle(" " + name + " ")
	view.SetDoneFunc(func(_ tcell.Key) { ui.menu() })
	view.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Rune() == 'q' {
			ui.menu()
			return nil
		}
		return event
	})
	ui.screen(name, view, "Arrows/PgUp/PgDn Scroll | Esc Back | q Back")
}

func (ui *terminal) entry() {
	draft, err := ui.books.NewDraft()
	if err != nil {
		ui.message(" Cannot start transaction ", err.Error(), ui.menu)
		return
	}
	entry := Entry{Postings: make([]Posting, 2)}
	ui.entryForm(draft, entry, ui.menu, 0)
}

func (ui *terminal) entryForm(draft *Draft, entry Entry, back func(), focus int) {
	ui.entryBack = back
	var showForm func()
	showForm = func() {
		discard := func() {
			modal := tview.NewModal().SetText("Discard this unsaved transaction?\nPress Esc again to discard and go back.").
				AddButtons([]string{"Keep editing", "Discard"})
			modal.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
				if event.Key() == tcell.KeyEscape {
					back()
					return nil
				}
				return event
			})
			modal.SetDoneFunc(func(index int, _ string) {
				if index == 1 {
					back()
				} else {
					showForm()
				}
			})
			ui.app.SetRoot(modal, true).SetFocus(modal)
		}
		add := func() {
			focus = 5 + 2*len(entry.Postings)
			entry.Postings = append(entry.Postings, Posting{})
			showForm()
		}
		remove := func() {
			if len(entry.Postings) > 2 {
				entry.Postings = entry.Postings[:len(entry.Postings)-1]
				focus = 5 + 2*(len(entry.Postings)-1)
				showForm()
			}
		}
		save := func() {
			_, err := draft.Preview(entry)
			if err != nil {
				ui.message(" Cannot save ", err.Error(), showForm)
				return
			}
			ui.save(draft, showForm)
		}
		form := newEntryGrid(ui.app, &entry, draft, &focus, add, remove, save, discard)
		if draft.statement != nil {
			form.SetTitle(fmt.Sprintf(" Statement row %d/%d - UNSAVED ", draft.statement.index+1, len(draft.statement.statement.Rows)))
		}
		title := "Record transaction"
		if draft.revision != nil {
			title = "Revise working transaction"
			form.SetTitle(" Revise existing transaction - UNSAVED ")
		}
		navigate := form.GetInputCapture()
		form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyF4 {
				ui.edit(draft, entry.journalText(draft.id), showForm)
				return nil
			}
			if event.Key() == tcell.KeyF2 {
				save()
				return nil
			}
			if event.Key() == tcell.KeyEscape {
				discard()
				return nil
			}
			return navigate(event)
		})
		target := focus
		ui.screen(title, form,
			"Esc Back (confirm discard) | F2 Save | F4 Ledger editor\n Up/Down Move | Tab/Shift-Tab Next/Previous | Ctrl-N/P Suggest | Enter Accept")
		ui.app.SetFocus(form.controls[target].primitive)
	}
	showForm()
}

func (ui *terminal) save(draft *Draft, back func()) {
	backup, err := draft.Save()
	if err != nil {
		if draft.saved {
			back = ui.menu
		}
		ui.message(" Save result ", err.Error(), back)
		return
	}
	next := ui.menu
	if draft.revision != nil {
		next = ui.entryBack
	} else if draft.statement != nil {
		next = func() { ui.statementNext(draft.statement.index) }
	}
	ui.message(" Saved ", "Transaction saved and validated.\nRecovery copy: "+backup, next)
}

func formNavigation(event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyUp && event.Key() != tcell.KeyDown {
		return event
	}
	key := tcell.KeyTab
	if event.Key() == tcell.KeyUp {
		key = tcell.KeyBacktab
	}
	return tcell.NewEventKey(key, 0, event.Modifiers())
}

func (ui *terminal) preview(draft *Draft, text string, back, edit func()) {
	view := tview.NewTextView().SetWrap(false).SetText(text)
	view.SetBorder(true).SetTitle(" Preview - NOT SAVED ")
	buttons := tview.NewForm().
		AddButton("Save", func() { ui.save(draft, back) }).
		AddButton("Edit ledger text (F4)", edit).
		AddButton("Back to edit", back)
	buttons.SetCancelFunc(back)
	buttons.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return formNavigation(event)
	})
	body := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(view, 0, 1, false).
		AddItem(buttons, 3, 0, true)
	body.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF4 {
			edit()
			return nil
		}
		if event.Key() == tcell.KeyEscape {
			back()
			return nil
		}
		if event.Key() == tcell.KeyPgUp || event.Key() == tcell.KeyPgDn {
			view.InputHandler()(event, func(tview.Primitive) {})
			return nil
		}
		return event
	})
	ui.screen("Preview transaction", body, "Up/Down or Tab Choose | F4 Ledger editor | PgUp/PgDn Scroll | Esc Back")
	ui.app.SetFocus(buttons)
}

func (ui *terminal) edit(draft *Draft, initial string, back func()) {
	text := initial
	var err error
	if !ui.app.Suspend(func() { text, err = ui.books.editText(initial) }) {
		err = fmt.Errorf("could not suspend the terminal to open the editor")
	}
	if err == nil {
		err = draft.PreviewText(text)
	}
	if err != nil {
		modal := tview.NewModal().
			SetText(tview.Escape(err.Error() + "\n\nThe journals have not changed. Reopen the editor to keep working on this text.")).
			AddButtons([]string{"Edit again", "Discard text edits"})
		modal.SetTitle(" Pending transaction ").SetBorder(true)
		modal.SetDoneFunc(func(index int, _ string) {
			if index == 1 {
				back()
			} else {
				ui.edit(draft, text, back)
			}
		})
		ui.app.SetRoot(modal, true).SetFocus(modal)
		return
	}
	var showPreview func()
	showPreview = func() {
		ui.preview(draft, draft.text, func() {
			modal := tview.NewModal().
				SetText("Discard ledger-text edits and return to the entry form?").
				AddButtons([]string{"Keep text", "Discard text edits"})
			modal.SetDoneFunc(func(index int, _ string) {
				if index == 1 {
					draft.text = ""
					back()
				} else {
					showPreview()
				}
			})
			ui.app.SetRoot(modal, true).SetFocus(modal)
		}, func() { ui.edit(draft, draft.text, back) })
	}
	showPreview()
}

func ReportNames() string {
	return strings.Join([]string{"journal", "trial-balance", "balance-sheet", "income-statement"}, ", ")
}
