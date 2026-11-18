package wat

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"
)

type BillingEvidence struct {
	ID     string `yaml:"id"`
	Digest string `yaml:"sha256"`
}

type BillingConfirmation struct {
	Through     string            `yaml:"through"`
	ConfirmedOn string            `yaml:"confirmed_on"`
	Review      string            `yaml:"review"`
	Evidence    []BillingEvidence `yaml:"evidence"`
}

func validServiceDate(date string) bool {
	_, err := time.Parse("2006-01-02", date)
	return err == nil && date != "9999-12-31"
}

func (s ServiceAccount) validateBillingConfirmation() error {
	c := s.FinalBilling
	if c == nil {
		return nil
	}
	if s.ServiceThrough == nil || c.Through != *s.ServiceThrough ||
		!validServiceDate(c.ConfirmedOn) || c.ConfirmedOn < c.Through ||
		!validRegistryText(c.Review) || c.Evidence == nil {
		return fmt.Errorf("%s: final billing requires closure coverage, a confirmation date on/after closure, review evidence, and an evidence array", s.ID)
	}
	last := ""
	for _, evidence := range c.Evidence {
		digest, err := hex.DecodeString(evidence.Digest)
		if !validRegistryText(evidence.ID) || evidence.ID <= last || err != nil || len(digest) != 32 {
			return fmt.Errorf("%s: invalid or unsorted billing evidence", s.ID)
		}
		last = evidence.ID
	}
	return nil
}

func accountWithin(account, parent string) bool {
	return account == parent || strings.HasPrefix(account, parent+":")
}

func (s ServiceAccount) owns(account string) bool {
	for _, role := range s.roles() {
		if accountWithin(account, role.account) {
			return true
		}
	}
	return false
}

func (s ServiceAccount) touches(tx journalTransaction) bool {
	for _, posting := range tx.Postings {
		if s.owns(posting.Account) {
			return true
		}
	}
	return false
}

func checkServiceTransactions(registry *CustomerRegistry, transactions []journalTransaction) error {
	if registry == nil {
		return nil
	}
	for _, service := range registry.ServiceAccounts {
		for _, tx := range transactions {
			if !service.touches(tx) {
				continue
			}
			noncash := false
			hasBank, onlyCustomerAccounts := false, true
			for _, posting := range tx.Postings {
				if service.owns(posting.Account) {
					if posting.Type != "RegularPosting" || posting.Date != nil || posting.Date2 != nil {
						return fmt.Errorf("%s: customer postings require regular postings without separate posting dates", service.ID)
					}
					if _, err := usdAmountsCents(posting.Amounts); err != nil {
						return fmt.Errorf("%s: customer posting %w", service.ID, err)
					}
					for _, amount := range posting.Amounts {
						if presentJSON(amount.Cost) || presentJSON(amount.CostBasis) {
							return fmt.Errorf("%s: customer postings cannot have commodity costs", service.ID)
						}
					}
				} else if strings.HasPrefix(posting.Account, "Bank:") {
					hasBank = true
				} else {
					noncash = true
					customerAccount := false
					for _, other := range registry.ServiceAccounts {
						customerAccount = customerAccount || other.owns(posting.Account)
					}
					onlyCustomerAccounts = onlyCustomerAccounts && customerAccount
				}
			}
			if hasBank && onlyCustomerAccounts {
				noncash = false
			}
			through := ""
			for _, tag := range tx.Tags {
				if tag[0] == "service-through" {
					if through != "" {
						return fmt.Errorf("%s: repeated service-through tag", service.ID)
					}
					through = tag[1]
					if !validServiceDate(through) || through < service.ServiceFrom {
						return fmt.Errorf("%s: service-through must be a valid date during service", service.ID)
					}
				}
			}
			if through != "" && service.ServiceThrough != nil && through > *service.ServiceThrough {
				return fmt.Errorf("%s: service-through exceeds the closed service period", service.ID)
			}
			// Cash movements and transfers between this account's roles are not new service.
			if noncash && service.ServiceThrough != nil && tx.Date > *service.ServiceThrough && through == "" {
				return fmt.Errorf("%s: later-posted charge/adjustment requires service-through evidence on or before %s", service.ID, *service.ServiceThrough)
			}
		}
	}
	return nil
}

func serviceEvidence(service ServiceAccount, transactions []journalTransaction) ([]BillingEvidence, error) {
	evidence := []BillingEvidence{}
	ids := make(map[string]int)
	for _, tx := range transactions {
		for _, tag := range tx.Tags {
			if tag[0] == "id" {
				ids[tag[1]]++
			}
		}
	}
	for _, tx := range transactions {
		if !service.touches(tx) {
			continue
		}
		id, err := transactionID(tx)
		if err != nil || !validRegistryText(id) || ids[id] != 1 {
			return nil, fmt.Errorf("%s: every related transaction requires one unique nonempty id before confirming billing", service.ID)
		}
		digest, err := transactionDigest(tx)
		if err != nil {
			return nil, err
		}
		evidence = append(evidence, BillingEvidence{id, digest})
	}
	slices.SortFunc(evidence, func(a, b BillingEvidence) int { return strings.Compare(a.ID, b.ID) })
	return evidence, nil
}

func (d *RegistryDraft) CloseService(id, through string) error {
	service, err := d.Registry.Service(id)
	if err != nil {
		return err
	}
	if service.ServiceThrough != nil {
		return fmt.Errorf("%s is already closed through %s", id, *service.ServiceThrough)
	}
	if !validServiceDate(through) || through < service.ServiceFrom {
		return fmt.Errorf("through must be a valid date on or after service_from")
	}
	for i := range d.Registry.ServiceAccounts {
		if d.Registry.ServiceAccounts[i].ID == id {
			d.Registry.ServiceAccounts[i].ServiceThrough = &through
		}
	}
	return nil
}

func (d *RegistryDraft) ConfirmServiceBilling(id, through, date, review string) error {
	service, err := d.Registry.Service(id)
	if err != nil {
		return err
	}
	if err := d.draft.unchanged(); err != nil {
		return err
	}
	transactions, err := d.books.transactions(d.books.Config.Journal)
	if err != nil {
		return err
	}
	evidence, err := serviceEvidence(service, transactions)
	if err != nil {
		return err
	}
	service.FinalBilling = &BillingConfirmation{through, date, review, evidence}
	if err := service.validateBillingConfirmation(); err != nil {
		return err
	}
	for i := range d.Registry.ServiceAccounts {
		if d.Registry.ServiceAccounts[i].ID == id {
			d.Registry.ServiceAccounts[i].FinalBilling = service.FinalBilling
		}
	}
	return nil
}

type ServiceBalance struct {
	Role, Account string
	Cents         int64
}

type ServiceSettlement struct {
	Service      ServiceAccount
	AsOf         string
	Balances     []ServiceBalance
	BillingValid bool
	Settled      bool
	Reason       string
}

func (b *Books) serviceBalance(account, asOf string) (int64, error) {
	date, err := time.Parse("2006-01-02", asOf)
	if err != nil || asOf == "9999-12-31" {
		return 0, fmt.Errorf("as-of must be a valid YYYY-MM-DD date before 9999-12-31")
	}
	text, err := b.run(b.Config.Journal, "balance", "acct:^"+regexp.QuoteMeta(account)+"(:|$)",
		"--end", date.AddDate(0, 0, 1).Format("2006-01-02"), "--output-format=json", "--strict")
	if err != nil {
		return 0, err
	}
	var result []json.RawMessage
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return 0, err
	}
	if len(result) != 2 {
		return 0, fmt.Errorf("unexpected hledger balance report")
	}
	var totals []ledgerAmount
	if err := json.Unmarshal(result[1], &totals); err != nil {
		return 0, err
	}
	return usdAmountsCents(totals)
}

func (b *Books) ServiceSettlement(id, asOf string) (ServiceSettlement, error) {
	var status ServiceSettlement
	registry, err := b.Customers()
	if err != nil {
		return status, err
	}
	service, err := registry.Service(id)
	if err != nil {
		return status, err
	}
	if err := b.Check(); err != nil {
		return status, err
	}
	status.Service, status.AsOf = service, asOf
	zero := true
	for _, role := range service.roles() {
		cents, err := b.serviceBalance(role.account, asOf)
		if err != nil {
			return status, err
		}
		status.Balances = append(status.Balances, ServiceBalance{role.name, role.account, cents})
		zero = zero && cents == 0
	}
	status.Reason = "service is not closed as of this date"
	if service.ServiceThrough == nil || *service.ServiceThrough > asOf {
		return status, nil
	}
	status.Reason = "billing completion has not been confirmed through closure"
	if service.FinalBilling == nil || service.FinalBilling.ConfirmedOn > asOf {
		return status, nil
	}
	transactions, err := b.transactions(b.Config.Journal)
	if err != nil {
		return status, err
	}
	evidence, err := serviceEvidence(service, transactions)
	if err != nil {
		return status, err
	}
	status.Reason = "billing confirmation is stale; review recorded activity and confirm again"
	if !reflect.DeepEqual(evidence, service.FinalBilling.Evidence) {
		return status, nil
	}
	status.BillingValid = true
	status.Reason = "one or more obligations remain; balances are not netted across roles"
	if zero {
		status.Settled = true
		status.Reason = "service closed, billing confirmed, and every account role has zero balance"
	}
	return status, nil
}

func (s ServiceSettlement) String() string {
	var out strings.Builder
	state := "NOT SETTLED"
	if s.Settled {
		state = "SETTLED"
	}
	fmt.Fprintf(&out, "%s as of %s: %s\n%s\n", s.Service.ID, s.AsOf, state, s.Reason)
	for _, balance := range s.Balances {
		fmt.Fprintf(&out, "%s: $%s [%s]\n", balance.Role, formatCents(balance.Cents), balance.Account)
	}
	out.WriteString("Positive receivable is owed to the utility; negative liability is owed to the customer.\n")
	out.WriteString("This review makes no financial posting and stores no financial status.\n")
	return out.String()
}

func (b *Books) serviceDraft(id, date, reference string) (*Draft, ServiceAccount, Customer, error) {
	var service ServiceAccount
	var customer Customer
	registry, err := b.Customers()
	if err != nil {
		return nil, service, customer, err
	}
	service, err = registry.Service(id)
	if err != nil {
		return nil, service, customer, err
	}
	customer, err = registry.Customer(service.Customer)
	if err != nil {
		return nil, service, customer, err
	}
	if !validServiceDate(date) || !validRegistryText(reference) {
		return nil, service, customer, fmt.Errorf("a valid posting date and explicit review/approval reference are required")
	}
	draft, err := b.NewDraft()
	return draft, service, customer, err
}

func previewServiceDraft(draft *Draft, entry Entry) (*Draft, string, error) {
	text, err := draft.Preview(entry)
	if err == nil {
		err = draft.PreviewText(text)
	}
	if err != nil {
		return nil, "", err
	}
	return draft, text, nil
}

func (b *Books) ServiceAdjustmentDraft(id, date, through, amount, counteraccount, reference string) (*Draft, string, error) {
	draft, service, customer, err := b.serviceDraft(id, date, reference)
	if err != nil {
		return nil, "", err
	}
	cents, err := ParseCents(amount)
	if err != nil || cents == 0 || !validServiceDate(through) || through < service.ServiceFrom ||
		(service.ServiceThrough != nil && through > *service.ServiceThrough) {
		return nil, "", fmt.Errorf("provide a nonzero signed USD charge/credit and service coverage within the service period")
	}
	if strings.HasPrefix(counteraccount, "Bank:") {
		return nil, "", fmt.Errorf("a charge/credit cannot create a bank movement")
	}
	for _, other := range b.registry.ServiceAccounts {
		if other.owns(counteraccount) {
			return nil, "", fmt.Errorf("a charge/credit cannot transfer another customer obligation")
		}
	}
	return previewServiceDraft(draft, Entry{
		Date: date, Payee: customer.Payee, Description: "Reviewed service charge/credit " + id, Reference: reference,
		comments: []string{"service-through: " + through},
		Postings: []Posting{{service.ReceivableAccount, formatCents(cents)}, {counteraccount, formatCents(-cents)}},
	})
}

func (b *Books) ServiceApplyDraft(id, role, amount, date, reference string) (*Draft, string, error) {
	draft, service, customer, err := b.serviceDraft(id, date, reference)
	if err != nil {
		return nil, "", err
	}
	if service.ServiceThrough == nil || date < *service.ServiceThrough {
		return nil, "", fmt.Errorf("application requires service closed on or before the posting date")
	}
	account := ""
	if role == "deposit" {
		account = service.DepositAccount
	} else if role == "credit" {
		account = service.CreditAccount
	}
	cents, err := ParseCents(amount)
	if err != nil || cents <= 0 || account == "" {
		return nil, "", fmt.Errorf("select a configured deposit or credit role and a positive USD amount")
	}
	if err := b.Check(); err != nil {
		return nil, "", err
	}
	liability, err := b.serviceBalance(account, date)
	if err != nil {
		return nil, "", err
	}
	receivable, err := b.serviceBalance(service.ReceivableAccount, date)
	if err != nil {
		return nil, "", err
	}
	if liability > -cents || receivable < cents {
		return nil, "", fmt.Errorf("application exceeds the available liability or outstanding receivable")
	}
	return previewServiceDraft(draft, Entry{
		Date: date, Payee: customer.Payee, Description: "Approved " + role + " application " + id, Reference: reference,
		Postings: []Posting{{account, formatCents(cents)}, {service.ReceivableAccount, formatCents(-cents)}},
	})
}

// ServiceRefundDraft allocates an actual recorded disbursement, never a planned refund.
func (b *Books) ServiceRefundDraft(transaction, id, role string) (*Draft, string, error) {
	registry, err := b.Customers()
	if err != nil {
		return nil, "", err
	}
	service, err := registry.Service(id)
	if err != nil {
		return nil, "", err
	}
	account := ""
	for _, candidate := range service.roles() {
		if candidate.name == role {
			account = candidate.account
		}
	}
	if account == "" {
		return nil, "", fmt.Errorf("select a configured receivable, credit, or deposit role")
	}
	if err := b.Check(); err != nil {
		return nil, "", err
	}
	draft, entry, err := b.EditDraft(transaction)
	if err != nil {
		return nil, "", err
	}
	if service.ServiceThrough == nil || entry.Date < *service.ServiceThrough {
		return nil, "", fmt.Errorf("refund allocation requires a disbursement on or after service closure")
	}
	var bank *Posting
	var paid, alreadyAssigned int64
	for i, posting := range entry.Postings {
		cents, err := ParseCents(posting.Amount)
		if err != nil {
			return nil, "", err
		}
		if strings.HasPrefix(posting.Account, "Bank:") {
			if bank != nil || cents >= 0 {
				return nil, "", fmt.Errorf("refund requires exactly one negative bank posting")
			}
			bank, paid = &entry.Postings[i], -cents
		} else {
			if cents <= 0 {
				return nil, "", fmt.Errorf("refund requires debit-only counterpostings; use the transaction editor for complex splits")
			}
			if accountWithin(posting.Account, account) {
				alreadyAssigned += cents
			}
		}
	}
	if bank == nil {
		return nil, "", fmt.Errorf("refund requires an existing bank disbursement")
	}
	balance, err := b.serviceBalance(account, entry.Date)
	if err != nil {
		return nil, "", err
	}
	if balance > -(paid - alreadyAssigned) {
		return nil, "", fmt.Errorf("disbursement exceeds the refundable balance for %s", role)
	}
	entry.Postings = []Posting{*bank, {account, formatCents(paid)}}
	return previewServiceDraft(draft, entry)
}

func (b *Books) IssueClosingStatement(id, asOf string) (path string, err error) {
	snapshot, err := b.NewDraft()
	if err != nil {
		return "", err
	}
	status, err := b.ServiceSettlement(id, asOf)
	if err != nil {
		return "", err
	}
	if !status.BillingValid {
		return "", fmt.Errorf("cannot issue closing statement: %s", status.Reason)
	}
	report, err := b.ServiceStatement(id, "", asOf)
	if err != nil {
		return "", err
	}
	transactions, err := b.transactions(b.Config.Journal)
	if err != nil {
		return "", err
	}
	var references strings.Builder
	for _, tx := range transactions {
		if status.Service.touches(tx) && tx.Date <= asOf {
			digest, err := transactionDigest(tx)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&references, "%s | %s | sha256:%s\n  reference: %s\n  source: %s | bank-row: %s\n",
				tx.Date, transactionTag(tx, "id"), digest, transactionTag(tx, "reference"),
				transactionTag(tx, "source"), transactionTag(tx, "bank-row"))
		}
	}
	directory := filepath.Join(b.Config.Sources, "customer-statements", id)
	for _, dir := range []string{b.Config.Sources, filepath.Dir(directory), directory} {
		if err := ensureDirectory(dir); err != nil {
			return "", err
		}
		if err := syncDirectory(filepath.Dir(dir)); err != nil {
			return "", err
		}
	}
	file, err := os.CreateTemp(directory, ".wat-statement-")
	if err != nil {
		return "", err
	}
	temp := file.Name()
	defer func() { err = errors.Join(err, os.Remove(temp)) }()
	if err := file.Close(); err != nil {
		return "", err
	}
	for revision := 1; revision <= 999999; revision++ {
		name := fmt.Sprintf("statement-%04d.txt", revision)
		path = filepath.Join(directory, name)
		if _, err := os.Lstat(path); err == nil {
			continue
		} else if !os.IsNotExist(err) {
			return "", err
		}
		confirmation := status.Service.FinalBilling
		text := fmt.Sprintf("CLOSING STATEMENT %s/%s\nIssued: %s\n\n%s\n%s\nBilling reviewed through %s on %s\nReview: %s\n\nTransaction/source references:\n%s",
			id, name, time.Now().UTC().Format(time.RFC3339), report, status.String(),
			confirmation.Through, confirmation.ConfirmedOn, confirmation.Review, references.String())
		if err := writeSynced(temp, []byte(text), 0400); err != nil {
			return "", err
		}
		if err := snapshot.unchanged(); err != nil {
			return "", err
		}
		if err := os.Link(temp, path); err != nil {
			return "", fmt.Errorf("publish statement without overwriting: %w", err)
		}
		if err := syncDirectory(directory); err != nil {
			return path, fmt.Errorf("statement issued at %s, but directory sync failed: %w", path, err)
		}
		return path, nil
	}
	return "", fmt.Errorf("statement revision numbers exhausted")
}
