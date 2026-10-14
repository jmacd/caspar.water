package wat

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type terminal struct {
	app   *tview.Application
	books *Books
}

func RunTerminal(books *Books) error {
	tview.Styles.PrimitiveBackgroundColor = tcell.NewHexColor(0x101b2b)
	tview.Styles.ContrastBackgroundColor = tcell.NewHexColor(0x25466b)
	tview.Styles.BorderColor = tcell.NewHexColor(0x629ac2)
	tview.Styles.PrimaryTextColor = tcell.NewHexColor(0xe6edf3)
	ui := &terminal{app: tview.NewApplication(), books: books}
	ui.menu()
	return ui.app.Run()
}

func (ui *terminal) screen(title string, body tview.Primitive, footer string) {
	header := tview.NewTextView().SetDynamicColors(true)
	header.SetText(tview.Escape(fmt.Sprintf(" %s | %d | %s\n %s",
		ui.books.Config.Name, ui.books.Config.Year, title, ui.books.Config.Journal)))
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
	list.AddItem("Record a transaction", "Balanced postings with a journal preview", '1', ui.entry).
		AddItem("Review journal", "Transactions in the selected year", '2', func() { ui.report("journal") }).
		AddItem("Reports", "Trial balance, balance sheet, income statement", '3', ui.reports).
		AddItem("Check books", "Run hledger strict validation", '4', func() {
			if err := ui.books.Check(); err != nil {
				ui.message(" Check failed ", err.Error(), ui.menu)
				return
			}
			ui.message(" Check books ", "The books pass hledger strict checks.\nThis does not establish bank reconciliation or completeness.", ui.menu)
		}).
		AddItem("Quit", "Leave all saved journal files in place", 'q', ui.app.Stop)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.app.Stop()
			return nil
		}
		return event
	})
	ui.screen("Menu", list, "Enter Select | 1-4 Shortcut | q Quit")
}

func (ui *terminal) reports() {
	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" Reports ")
	for _, name := range []string{"trial-balance", "balance-sheet", "income-statement"} {
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
	entry := Entry{Postings: []Posting{{Account: draft.accounts[0]}, {Account: draft.accounts[0]}}}
	var showForm func()
	showForm = func() {
		form := tview.NewForm()
		form.SetBorder(true).SetTitle(" New transaction - UNSAVED ")
		form.AddInputField("Date (YYYY-MM-DD)", entry.Date, 12, nil, func(s string) { entry.Date = s }).
			AddInputField("Payee", entry.Payee, 48, nil, func(s string) { entry.Payee = s }).
			AddInputField("Description", entry.Description, 48, nil, func(s string) { entry.Description = s }).
			AddInputField("Reference", entry.Reference, 48, nil, func(s string) { entry.Reference = s }).
			AddInputField("Source reference", entry.Source, 48, nil, func(s string) { entry.Source = s })
		options := make([]string, len(draft.accounts))
		for i, account := range draft.accounts {
			options[i] = tview.Escape(account)
		}
		for i := range entry.Postings {
			i := i
			selected := 0
			for j, account := range draft.accounts {
				if account == entry.Postings[i].Account {
					selected = j
				}
			}
			form.AddDropDown(fmt.Sprintf("Account %d", i+1), options, selected, func(_ string, index int) {
				if index >= 0 {
					entry.Postings[i].Account = draft.accounts[index]
				}
			})
			form.AddInputField(fmt.Sprintf("USD amount %d (+debit/-credit)", i+1),
				entry.Postings[i].Amount, 24, nil, func(s string) { entry.Postings[i].Amount = s })
		}
		discard := func() {
			modal := tview.NewModal().SetText("Discard this unsaved transaction?").
				AddButtons([]string{"Keep editing", "Discard"})
			modal.SetDoneFunc(func(index int, _ string) {
				if index == 1 {
					ui.menu()
				} else {
					showForm()
				}
			})
			ui.app.SetRoot(modal, true).SetFocus(modal)
		}
		form.AddButton("Add posting", func() {
			entry.Postings = append(entry.Postings, Posting{Account: draft.accounts[0]})
			showForm()
		}).AddButton("Remove last posting", func() {
			if len(entry.Postings) > 2 {
				entry.Postings = entry.Postings[:len(entry.Postings)-1]
				showForm()
			}
		}).AddButton("Preview", func() {
			text, err := draft.Preview(entry)
			if err != nil {
				ui.message(" Cannot preview ", err.Error(), showForm)
				return
			}
			ui.preview(draft, text, showForm, func() { ui.edit(draft, draft.text, showForm) })
		}).AddButton("Cancel", discard)
		form.GetButton(1).SetDisabled(len(entry.Postings) <= 2)
		form.SetCancelFunc(discard)
		form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
			if event.Key() == tcell.KeyF4 {
				ui.edit(draft, entry.journalText(draft.id), showForm)
				return nil
			}
			return formNavigation(form, event)
		})
		ui.screen("Record transaction", form,
			"Up/Down or Tab Move | Enter Select | F4 Ledger editor | Esc Cancel")
	}
	showForm()
}

func formNavigation(form *tview.Form, event *tcell.EventKey) *tcell.EventKey {
	if event.Key() != tcell.KeyUp && event.Key() != tcell.KeyDown {
		return event
	}
	index, _ := form.GetFocusedItemIndex()
	if index >= 0 {
		if dropdown, ok := form.GetFormItem(index).(*tview.DropDown); ok && dropdown.IsOpen() {
			return event
		}
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
		AddButton("Save", func() {
			backup, err := draft.Save()
			if err != nil {
				next := back
				if draft.saved {
					next = ui.menu
				}
				ui.message(" Save result ", err.Error(), next)
				return
			}
			ui.message(" Saved ", "Transaction saved and validated.\nRecovery copy: "+backup, ui.menu)
		}).
		AddButton("Edit ledger text (F4)", edit).
		AddButton("Back to edit", back)
	buttons.SetCancelFunc(back)
	buttons.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		return formNavigation(buttons, event)
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
