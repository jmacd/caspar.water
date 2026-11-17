package wat

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func RunCustomersTerminal(books *Books) error {
	if _, err := books.Customers(); err != nil {
		return err
	}
	applyTUITheme()
	ui := &terminal{app: tview.NewApplication(), books: books}
	ui.captureGlobalInput()
	ui.customers()
	return ui.app.Run()
}

func (ui *terminal) customers() {
	registry, err := ui.books.Customers()
	if err != nil {
		ui.message(" Customer registry ", err.Error(), ui.menu)
		return
	}
	list := themedList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(" Customers / service accounts ")
	list.AddItem("Declare a payee", "Separate save to the native payee journal", 'p', ui.customerPayeeForm).
		AddItem("Add customer", "Select an already declared payee", 'a', func() { ui.customerForm("") }).
		AddItem("Add connection", "Create a physical service point", 'n', func() { ui.connectionForm("") }).
		AddItem("Open service", "Link a customer to a connection without posting money", 'o', ui.serviceForm)
	for _, customer := range registry.Customers {
		list.AddItem(customer.ID+" | "+customer.Payee, strings.Join(customer.BillingAddress.Lines, ", "), 0,
			func() { ui.customerDetails("customer", customer.ID) })
	}
	for _, connection := range registry.Connections {
		list.AddItem(connection.ID+" | "+connection.Name, strings.Join(connection.ServiceAddress.Lines, ", "), 0,
			func() { ui.customerDetails("connection", connection.ID) })
	}
	for _, service := range registry.ServiceAccounts {
		list.AddItem(registry.serviceLabel(service), service.ReceivableAccount, 0,
			func() { ui.customerDetails("service", service.ID) })
	}
	list.AddItem("Back", "", 'q', ui.menu)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.menu()
			return nil
		}
		return event
	})
	ui.screen("Customers / service accounts", list, "Enter Select | p Payee | a Customer | n Connection | o Open service | Esc Back")
}

func (ui *terminal) customerDetails(kind, id string) {
	list := themedList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle(" " + id + " ")
	list.AddItem("Statement / history", "", 's', func() { ui.customerReportForm(kind, id) })
	if kind == "customer" {
		list.AddItem("Edit customer", "", 'e', func() { ui.customerForm(id) })
	}
	if kind == "connection" {
		list.AddItem("Edit connection", "", 'e', func() { ui.connectionForm(id) })
	}
	list.AddItem("Back", "", 'q', ui.customers)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.customers()
			return nil
		}
		return event
	})
	ui.screen("Account details", list, "Enter Select | Esc Back")
}

func addressLines(text string) []string {
	lines := strings.Split(text, ";")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return lines
}

func (ui *terminal) registryForm(title string, form *tview.Form, draft *RegistryDraft, prepare func() (string, error)) {
	original := draft.Registry
	ui.customerSaveForm(title, form, func() (string, bool, error) {
		draft.Registry = original
		id, err := prepare()
		if err != nil {
			return "", false, err
		}
		backup, err := draft.Save()
		return fmt.Sprintf("Saved %s.\nRecovery copy: %s", id, backup), draft.saved, err
	})
}

func (ui *terminal) customerSaveForm(title string, form *tview.Form, save func() (string, bool, error)) {
	var show func()
	show = func() {
		ui.screen(title, form, "F2 Save this step | Esc Discard draft | Earlier saves are retained")
		ui.app.SetFocus(form)
	}
	discard := func() {
		modal := themedModal().SetText("Discard this unsaved draft? Earlier saved steps remain.").
			AddButtons([]string{"Keep editing", "Discard"})
		modal.SetDoneFunc(func(index int, _ string) {
			if index == 1 {
				ui.customers()
			} else {
				show()
			}
		})
		ui.app.SetRoot(modal, true).SetFocus(modal)
	}
	commit := func() {
		text, saved, err := save()
		if err != nil {
			back := show
			if saved {
				back = ui.customers
			}
			ui.message(" Save result ", err.Error(), back)
			return
		}
		ui.message(" Saved ", text, ui.customers)
	}
	form.SetBorder(true).SetTitle(" " + title + " - UNSAVED ")
	form.AddButton("Save (F2)", commit).AddButton("Back (Esc)", discard)
	form.SetCancelFunc(discard)
	form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		for i := 0; i < form.GetFormItemCount(); i++ {
			if dropdown, ok := form.GetFormItem(i).(*tview.DropDown); ok && dropdown.IsOpen() {
				return event
			}
		}
		switch event.Key() {
		case tcell.KeyF2:
			commit()
			return nil
		case tcell.KeyEscape:
			discard()
			return nil
		}
		return formNavigation(event)
	})
	show()
}

func (ui *terminal) customerPayeeForm() {
	var payee string
	form := themedForm().AddInputField("Payee", "", 48, nil, func(text string) { payee = text })
	ui.customerSaveForm("Declare payee (separate step)", form, func() (string, bool, error) {
		backup, saved, err := ui.books.DeclarePayee(payee)
		return "Payee declared. Next add the customer.\nRecovery copy: " + backup, saved, err
	})
}

func (ui *terminal) customerForm(id string) {
	draft, err := ui.books.NewRegistryDraft()
	if err != nil {
		ui.message(" Customer ", err.Error(), ui.customers)
		return
	}
	payees, err := ui.books.declaredPayees()
	if err != nil {
		ui.message(" Customer ", err.Error(), ui.customers)
		return
	}
	if len(payees) == 0 {
		ui.message(" Customer ", "Declare a payee first. That is a separate save.", ui.customers)
		return
	}
	payee, address := "", ""
	selected := 0
	options := append([]string{"Select a declared payee"}, payees...)
	if id != "" {
		customer, err := draft.Registry.Customer(id)
		if err != nil {
			ui.message(" Customer ", err.Error(), ui.customers)
			return
		}
		payee, address = customer.Payee, strings.Join(customer.BillingAddress.Lines, "; ")
		for i, name := range options {
			if name == payee {
				selected = i
			}
		}
	}
	form := themedForm().
		AddDropDown("Declared payee", options, selected, func(_ string, index int) {
			payee = ""
			if index > 0 {
				payee = payees[index-1]
			}
		}).
		AddInputField("Billing address (; separates lines)", address, 60, nil, func(text string) { address = text })
	ui.registryForm("Customer", form, draft, func() (string, error) {
		if id == "" {
			return draft.AddCustomer(payee, addressLines(address))
		}
		for i := range draft.Registry.Customers {
			if draft.Registry.Customers[i].ID == id {
				draft.Registry.Customers[i].Payee = payee
				draft.Registry.Customers[i].BillingAddress = CustomerAddress{addressLines(address)}
			}
		}
		return id, nil
	})
}

func (ui *terminal) connectionForm(id string) {
	draft, err := ui.books.NewRegistryDraft()
	if err != nil {
		ui.message(" Connection ", err.Error(), ui.customers)
		return
	}
	name, address := "", ""
	if id != "" {
		connection, err := draft.Registry.Connection(id)
		if err != nil {
			ui.message(" Connection ", err.Error(), ui.customers)
			return
		}
		name, address = connection.Name, strings.Join(connection.ServiceAddress.Lines, "; ")
	}
	form := themedForm().
		AddInputField("Connection name", name, 48, nil, func(text string) { name = text }).
		AddInputField("Service address (; separates lines)", address, 60, nil, func(text string) { address = text })
	ui.registryForm("Connection", form, draft, func() (string, error) {
		if id == "" {
			return draft.AddConnection(name, addressLines(address))
		}
		for i := range draft.Registry.Connections {
			if draft.Registry.Connections[i].ID == id {
				draft.Registry.Connections[i].Name = name
				draft.Registry.Connections[i].ServiceAddress = CustomerAddress{addressLines(address)}
			}
		}
		return id, nil
	})
}

func (ui *terminal) serviceForm() {
	draft, err := ui.books.NewRegistryDraft()
	if err != nil {
		ui.message(" Open service ", err.Error(), ui.customers)
		return
	}
	customers, connections := []string{"Select customer"}, []string{"Select connection"}
	for _, customer := range draft.Registry.Customers {
		customers = append(customers, customer.ID+" | "+customer.Payee)
	}
	for _, connection := range draft.Registry.Connections {
		connections = append(connections, connection.ID+" | "+connection.Name)
	}
	var service ServiceAccount
	form := themedForm().
		AddDropDown("Customer", customers, 0, func(_ string, i int) {
			service.Customer = ""
			if i > 0 {
				service.Customer = draft.Registry.Customers[i-1].ID
			}
		}).
		AddDropDown("Connection", connections, 0, func(_ string, i int) {
			service.Connection = ""
			if i > 0 {
				service.Connection = draft.Registry.Connections[i-1].ID
			}
		}).
		AddInputField("Service from (YYYY-MM-DD)", "", 12, nil, func(text string) { service.ServiceFrom = text }).
		AddInputField("Receivable (blank = new 141 account)", "", 65, nil, func(text string) { service.ReceivableAccount = text }).
		AddInputField("Credit liability (optional, approved)", "", 65, nil, func(text string) { service.CreditAccount = text }).
		AddInputField("Security deposit (optional, 233)", "", 65, nil, func(text string) { service.DepositAccount = text })
	ui.registryForm("Open service (no financial posting)", form, draft, func() (string, error) {
		return draft.OpenService(service)
	})
}

func (ui *terminal) customerReportForm(kind, id string) {
	asOf, from := time.Now().Format("2006-01-02"), ""
	form := themedForm().
		AddInputField("As of (inclusive)", asOf, 12, nil, func(text string) { asOf = text }).
		AddInputField("From (blank = service start)", from, 12, nil, func(text string) { from = text })
	var show func()
	show = func() {
		ui.screen("Statement period", form, "Enter Select | Esc Back")
		ui.app.SetFocus(form)
	}
	back := func() { ui.customerDetails(kind, id) }
	form.AddButton("Show report", func() {
		text, err := ui.books.CustomerReport(kind, id, from, asOf)
		if err != nil {
			ui.message(" Report failed ", err.Error(), show)
			return
		}
		view := tview.NewTextView().SetWrap(false).SetText(text)
		view.SetBorder(true).SetTitle(" Recorded activity - not final settlement ")
		view.SetDoneFunc(func(_ tcell.Key) { show() })
		ui.screen("Customer statement", view, "Arrows/PgUp/PgDn Scroll | Esc Back")
	}).AddButton("Back", back)
	form.SetCancelFunc(back).SetInputCapture(formNavigation)
	show()
}

func (ui *terminal) chooseServicePosting(draft *Draft, choose func(string), back func()) {
	if err := draft.unchanged(); err != nil {
		ui.message(" Service accounts ", err.Error(), back)
		return
	}
	registry, err := ui.books.Customers()
	if err != nil {
		ui.message(" Service accounts ", err.Error(), back)
		return
	}
	list := themedList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(" Select service account and accounting role ")
	for _, service := range registry.ServiceAccounts {
		for _, role := range service.roles() {
			list.AddItem(registry.serviceLabel(service)+" | "+role.name, role.account, 0,
				func() { choose(role.account) })
		}
	}
	list.AddItem("Back (no change)", "", 'q', back)
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			back()
			return nil
		}
		return event
	})
	ui.screen("Allocate to service account", list, "Enter Select account only (amount unchanged) | Esc Back")
}
