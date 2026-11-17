package wat

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func (b *Books) CustomerReport(kind, selector, from, asOf string) (string, error) {
	if _, err := time.Parse("2006-01-02", asOf); err != nil || asOf == "9999-12-31" {
		return "", fmt.Errorf("as-of must be a valid YYYY-MM-DD date before 9999-12-31")
	}
	if from != "" {
		if _, err := time.Parse("2006-01-02", from); err != nil || from > asOf {
			return "", fmt.Errorf("from must be a valid YYYY-MM-DD date on or before as-of")
		}
	}
	registry, err := b.Customers()
	if err != nil {
		return "", err
	}
	if err := b.Check(); err != nil {
		return "", err
	}
	var record any
	var services []ServiceAccount
	switch kind {
	case "customer":
		customer, err := registry.Customer(selector)
		if err != nil {
			return "", err
		}
		record = customer
		for _, service := range registry.ServiceAccounts {
			if service.Customer == customer.ID {
				services = append(services, service)
			}
		}
	case "connection":
		connection, err := registry.Connection(selector)
		if err != nil {
			return "", err
		}
		record = connection
		for _, service := range registry.ServiceAccounts {
			if service.Connection == connection.ID {
				services = append(services, service)
			}
		}
	case "service":
		return b.ServiceStatement(selector, from, asOf)
	default:
		return "", fmt.Errorf("unknown customer report %q", kind)
	}
	data, err := yaml.Marshal(record)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.Write(data)
	for _, service := range services {
		text, err := b.ServiceStatement(service.ID, from, asOf)
		if err != nil {
			return "", err
		}
		out.WriteString("\n" + text)
	}
	if len(services) == 0 {
		out.WriteString("\nNo service accounts.\n")
	}
	return out.String(), nil
}

func (b *Books) ServiceStatement(id, from, asOf string) (string, error) {
	registry, err := b.Customers()
	if err != nil {
		return "", err
	}
	service, err := registry.Service(id)
	if err != nil {
		return "", err
	}
	end, err := time.Parse("2006-01-02", asOf)
	if err != nil || asOf == "9999-12-31" {
		return "", fmt.Errorf("as-of must be a valid YYYY-MM-DD date before 9999-12-31")
	}
	if from == "" {
		from = service.ServiceFrom
		if from > asOf {
			from = asOf
		}
	}
	if _, err := time.Parse("2006-01-02", from); err != nil || from > asOf {
		return "", fmt.Errorf("from must be a valid YYYY-MM-DD date on or before as-of")
	}
	if err := b.Check(); err != nil {
		return "", err
	}
	customer, err := registry.Customer(service.Customer)
	if err != nil {
		return "", err
	}
	connection, err := registry.Connection(service.Connection)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	fmt.Fprintf(&out, "%s\nService statement: %s\nCustomer: %s | %s\nBilling address: %s\nConnection: %s | %s\nService address: %s\n",
		b.Config.Name, service.ID, customer.ID, customer.Payee, strings.Join(customer.BillingAddress.Lines, ", "),
		connection.ID, connection.Name, strings.Join(connection.ServiceAddress.Lines, ", "))
	through := "open"
	if service.ServiceThrough != nil {
		through = *service.ServiceThrough
	}
	fmt.Fprintf(&out, "Service: %s through %s\nReport: %s through %s (inclusive)\n", service.ServiceFrom, through, from, asOf)
	for _, role := range service.roles() {
		query := "acct:^" + regexp.QuoteMeta(role.account) + "(:|$)"
		balance := func(before string) (string, error) {
			text, err := b.run(b.Config.Journal, "balance", query, "--end", before, "--depth=1", "--no-total", "--format=%(total)", "--strict")
			if err != nil {
				return "", err
			}
			if strings.TrimSpace(text) == "" {
				return "$0.00", nil
			}
			return strings.TrimSpace(text), nil
		}
		opening, err := balance(from)
		if err != nil {
			return "", err
		}
		exclusiveEnd := end.AddDate(0, 0, 1).Format("2006-01-02")
		closing, err := balance(exclusiveEnd)
		if err != nil {
			return "", err
		}
		activity, err := b.run(b.Config.Journal, "register", query,
			"--begin", from, "--end", exclusiveEnd, "--historical", "--width=140", "--strict")
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&out, "\n%s: %s\nOpening balance: %s\n", role.name, role.account, opening)
		if strings.TrimSpace(activity) == "" {
			out.WriteString("No activity in this report period.\n")
		} else {
			out.WriteString(activity)
		}
		fmt.Fprintf(&out, "Closing balance: %s\n", closing)
	}
	out.WriteString("\nLedger signs: positive receivable = owed to utility; negative liability = owed to customer.\n")
	out.WriteString("A receivable credit requires review; it is not automatically income, a security deposit, or settlement.\n")
	out.WriteString("Recorded activity only; this does not establish billing completeness or final settlement.\n")
	return out.String(), nil
}

// ServicePaymentDraft revises one existing receipt; it never creates another bank posting.
func (b *Books) ServicePaymentDraft(id string, allocations []string) (*Draft, string, error) {
	registry, err := b.Customers()
	if err != nil {
		return nil, "", err
	}
	if err := b.Check(); err != nil {
		return nil, "", err
	}
	draft, entry, err := b.EditDraft(id)
	if err != nil {
		return nil, "", err
	}
	var bank *Posting
	for i := range entry.Postings {
		posting := &entry.Postings[i]
		if strings.HasPrefix(posting.Account, "Bank:") {
			if bank != nil {
				return nil, "", fmt.Errorf("allocation requires exactly one bank posting")
			}
			bank = posting
		} else {
			cents, err := ParseCents(posting.Amount)
			if err != nil || cents >= 0 {
				return nil, "", fmt.Errorf("allocation requires a simple receipt with credit-only counterpostings; use the transaction editor for fees or other splits")
			}
		}
	}
	if bank == nil {
		return nil, "", fmt.Errorf("transaction is not a bank receipt")
	}
	received, err := ParseCents(bank.Amount)
	if err != nil || received <= 0 {
		return nil, "", fmt.Errorf("transaction must have a positive bank receipt")
	}
	postings := []Posting{*bank}
	var allocated int64
	seen := make(map[string]bool)
	for _, allocation := range allocations {
		selector, amount, ok := strings.Cut(allocation, "=")
		if !ok {
			return nil, "", fmt.Errorf("allocation must use SA-ID[:receivable|credit|deposit]=AMOUNT")
		}
		serviceID, role, hasRole := strings.Cut(selector, ":")
		if !hasRole {
			role = "receivable"
		}
		service, err := registry.Service(serviceID)
		if err != nil {
			return nil, "", err
		}
		account := ""
		for _, candidate := range service.roles() {
			if candidate.name == role {
				account = candidate.account
			}
		}
		if account == "" || seen[account] {
			return nil, "", fmt.Errorf("allocation %q has an unconfigured or repeated account role", selector)
		}
		seen[account] = true
		cents, err := ParseCents(amount)
		if err != nil || cents <= 0 || allocated > math.MaxInt64-cents {
			return nil, "", fmt.Errorf("allocation requires positive exact USD amounts within the supported range")
		}
		allocated += cents
		postings = append(postings, Posting{Account: account, Amount: formatCents(-cents)})
	}
	if allocated != received {
		return nil, "", fmt.Errorf("allocations must total the original receipt of %s", formatCents(received))
	}
	entry.Postings = postings
	text, err := draft.Preview(entry)
	if err != nil {
		return nil, "", err
	}
	return draft, text, nil
}
