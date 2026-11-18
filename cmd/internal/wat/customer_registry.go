package wat

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type CustomerAddress struct {
	Lines []string `yaml:"lines"`
}

type Customer struct {
	ID             string          `yaml:"id"`
	Payee          string          `yaml:"payee"`
	BillingAddress CustomerAddress `yaml:"billing_address"`
}

type Connection struct {
	ID             string          `yaml:"id"`
	Name           string          `yaml:"name"`
	ServiceAddress CustomerAddress `yaml:"service_address"`
}

type ServiceAccount struct {
	ID                string               `yaml:"id"`
	Customer          string               `yaml:"customer"`
	Connection        string               `yaml:"connection"`
	ServiceFrom       string               `yaml:"service_from"`
	ServiceThrough    *string              `yaml:"service_through"`
	ReceivableAccount string               `yaml:"receivable_account"`
	CreditAccount     string               `yaml:"credit_account,omitempty"`
	DepositAccount    string               `yaml:"deposit_account,omitempty"`
	FinalBilling      *BillingConfirmation `yaml:"final_billing,omitempty"`
}

type CustomerRegistry struct {
	Version         int              `yaml:"version"`
	Customers       []Customer       `yaml:"customers"`
	Connections     []Connection     `yaml:"connections"`
	ServiceAccounts []ServiceAccount `yaml:"service_accounts"`
}

type RegistryDraft struct {
	Registry CustomerRegistry
	books    *Books
	draft    *Draft
	saved    bool
}

func InitCustomerRegistry(cfg Config) (err error) {
	data, err := encodeRegistry(CustomerRegistry{
		Version: 1, Customers: []Customer{}, Connections: []Connection{}, ServiceAccounts: []ServiceAccount{},
	})
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(cfg.root, ".wat-registry-")
	if err != nil {
		return err
	}
	path := file.Name()
	defer func() { err = errors.Join(err, os.Remove(path)) }()
	if err := file.Close(); err != nil {
		return err
	}
	if err := writeSynced(path, data, 0600); err != nil {
		return err
	}
	// Link publishes the complete initial file without replacing an existing registry.
	if err := os.Link(path, cfg.Registry); err != nil {
		return fmt.Errorf("create registry (will not overwrite an existing file): %w", err)
	}
	if err := syncDirectory(cfg.root); err != nil {
		return fmt.Errorf("registry created, but directory sync failed: %w", err)
	}
	return nil
}

func encodeRegistry(registry CustomerRegistry) ([]byte, error) {
	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	if err := encoder.Encode(registry); err != nil {
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func loadCustomerRegistry(path string, chart accountChart) (*CustomerRegistry, fileState, error) {
	state, err := readNonsymlinkFileState(path, "customer registry")
	if err != nil {
		return nil, state, err
	}
	registry, err := decodeRegistry(state.data, chart)
	return registry, state, err
}

func decodeRegistry(data []byte, chart accountChart) (*CustomerRegistry, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var registry CustomerRegistry
	if err := decoder.Decode(&registry); err != nil {
		return nil, fmt.Errorf("decode customer registry: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("customer registry must contain exactly one YAML document")
	}
	if err := registry.validate(chart); err != nil {
		return nil, err
	}
	return &registry, nil
}

func validRegistryText(value string) bool {
	return value != "" && strings.TrimSpace(value) == value && singleLine(value)
}

func validPayeeName(value string) bool {
	return validRegistryText(value) && !strings.ContainsAny(value, ";|")
}

func (address CustomerAddress) validate() error {
	if len(address.Lines) == 0 {
		return fmt.Errorf("address requires at least one line")
	}
	for _, line := range address.Lines {
		if !validRegistryText(line) {
			return fmt.Errorf("address lines must be nonempty, single-line text without surrounding whitespace")
		}
	}
	return nil
}

func registryIDNumber(id, prefix string) (uint64, error) {
	number, err := strconv.ParseUint(strings.TrimPrefix(id, prefix+"-"), 10, 32)
	if err != nil || number == 0 || id != fmt.Sprintf("%s-%04d", prefix, number) {
		return 0, fmt.Errorf("invalid %s ID %q", prefix, id)
	}
	return number, nil
}

func nextRegistryID(prefix string, ids []string) (string, error) {
	var largest uint64
	for _, id := range ids {
		number, err := registryIDNumber(id, prefix)
		if err != nil {
			return "", err
		}
		if number > largest {
			largest = number
		}
	}
	if largest == 1<<32-1 {
		return "", fmt.Errorf("%s IDs exhausted", prefix)
	}
	return fmt.Sprintf("%s-%04d", prefix, largest+1), nil
}

func (r CustomerRegistry) validate(chart accountChart) error {
	if r.Version != 1 || r.Customers == nil || r.Connections == nil || r.ServiceAccounts == nil {
		return fmt.Errorf("customer registry requires version 1 and customers, connections, and service_accounts arrays")
	}
	ids := make(map[string]bool)
	checkID := func(id, prefix string) error {
		if _, err := registryIDNumber(id, prefix); err != nil {
			return err
		}
		if ids[id] {
			return fmt.Errorf("duplicate registry ID %s", id)
		}
		ids[id] = true
		return nil
	}
	for _, customer := range r.Customers {
		if err := checkID(customer.ID, "C"); err != nil {
			return err
		}
		if !validPayeeName(customer.Payee) {
			return fmt.Errorf("%s: invalid payee name", customer.ID)
		}
		if err := customer.BillingAddress.validate(); err != nil {
			return fmt.Errorf("%s billing address: %w", customer.ID, err)
		}
	}
	for _, connection := range r.Connections {
		if err := checkID(connection.ID, "CN"); err != nil {
			return err
		}
		if !validRegistryText(connection.Name) {
			return fmt.Errorf("%s: connection name is required and must be single-line text", connection.ID)
		}
		if err := connection.ServiceAddress.validate(); err != nil {
			return fmt.Errorf("%s service address: %w", connection.ID, err)
		}
	}
	owners := make(map[string]string)
	for i, service := range r.ServiceAccounts {
		if err := checkID(service.ID, "SA"); err != nil {
			return err
		}
		if !strings.HasPrefix(service.Customer, "C-") || !ids[service.Customer] ||
			!strings.HasPrefix(service.Connection, "CN-") || !ids[service.Connection] {
			return fmt.Errorf("%s: unknown customer or connection", service.ID)
		}
		if _, err := time.Parse("2006-01-02", service.ServiceFrom); err != nil {
			return fmt.Errorf("%s: service_from must be a valid YYYY-MM-DD date", service.ID)
		}
		end := "9999-12-31"
		if service.ServiceThrough != nil {
			end = *service.ServiceThrough
			if _, err := time.Parse("2006-01-02", end); err != nil || end < service.ServiceFrom {
				return fmt.Errorf("%s: service_through must be a valid date on or after service_from", service.ID)
			}
		}
		if err := service.validateBillingConfirmation(); err != nil {
			return err
		}
		for _, previous := range r.ServiceAccounts[:i] {
			if previous.Connection == service.Connection &&
				previous.ServiceFrom <= end &&
				(previous.ServiceThrough == nil || *previous.ServiceThrough >= service.ServiceFrom) {
				return fmt.Errorf("%s overlaps %s at connection %s", service.ID, previous.ID, service.Connection)
			}
		}
		if service.ReceivableAccount == "" {
			return fmt.Errorf("%s requires a receivable_account", service.ID)
		}
		for _, role := range service.roles() {
			account, ok := parsePostingAccount(role.account)
			if !ok {
				return fmt.Errorf("%s: invalid %s account %q", service.ID, role.name, role.account)
			}
			resolved, err := chart.resolve(account)
			if err != nil {
				return err
			}
			if (role.name == "receivable" && (resolved.Category != "Asset" || resolved.FormalNumber != "141")) ||
				(role.name == "deposit" && (resolved.Category != "Liability" || resolved.FormalNumber != "233")) ||
				(role.name == "credit" && (resolved.Category != "Liability" || resolved.FormalNumber == "233")) {
				return fmt.Errorf("%s: %s account has an incompatible CPUC classification", service.ID, role.name)
			}
			for owned, owner := range owners {
				if owned == role.account || strings.HasPrefix(owned, role.account+":") || strings.HasPrefix(role.account, owned+":") {
					return fmt.Errorf("%s: account %s overlaps ownership by %s", service.ID, role.account, owner)
				}
			}
			owners[role.account] = service.ID
		}
	}
	return nil
}

type serviceAccountRole struct{ name, account string }

func (s ServiceAccount) roles() []serviceAccountRole {
	roles := []serviceAccountRole{{"receivable", s.ReceivableAccount}}
	if s.CreditAccount != "" {
		roles = append(roles, serviceAccountRole{"credit", s.CreditAccount})
	}
	if s.DepositAccount != "" {
		roles = append(roles, serviceAccountRole{"deposit", s.DepositAccount})
	}
	return roles
}

func (s ServiceAccount) accounts() []string {
	var accounts []string
	for _, role := range s.roles() {
		accounts = append(accounts, role.account)
	}
	return accounts
}

func (r CustomerRegistry) accounts() []string {
	var accounts []string
	for _, service := range r.ServiceAccounts {
		accounts = append(accounts, service.accounts()...)
	}
	return accounts
}

func (r CustomerRegistry) Customer(selector string) (Customer, error) {
	var matches []Customer
	for _, customer := range r.Customers {
		if customer.ID == selector {
			return customer, nil
		}
		if customer.Payee == selector {
			matches = append(matches, customer)
		}
	}
	if len(matches) != 1 {
		return Customer{}, fmt.Errorf("customer %q matches %d records; use a customer ID", selector, len(matches))
	}
	return matches[0], nil
}

func (r CustomerRegistry) Connection(selector string) (Connection, error) {
	var matches []Connection
	for _, connection := range r.Connections {
		if connection.ID == selector {
			return connection, nil
		}
		if connection.Name == selector {
			matches = append(matches, connection)
		}
	}
	if len(matches) != 1 {
		return Connection{}, fmt.Errorf("connection %q matches %d records; use a connection ID", selector, len(matches))
	}
	return matches[0], nil
}

func (r CustomerRegistry) Service(selector string) (ServiceAccount, error) {
	for _, service := range r.ServiceAccounts {
		if service.ID == selector {
			return service, nil
		}
	}
	return ServiceAccount{}, fmt.Errorf("unknown service account %q", selector)
}

func (r CustomerRegistry) serviceLabel(service ServiceAccount) string {
	parts := []string{service.ID, service.Customer, service.Connection}
	if service.ServiceThrough != nil {
		parts = append(parts, "closed through "+*service.ServiceThrough)
	}
	for _, customer := range r.Customers {
		if customer.ID == service.Customer {
			parts = append(parts, customer.Payee)
		}
	}
	for _, connection := range r.Connections {
		if connection.ID == service.Connection {
			parts = append(parts, connection.Name, strings.Join(connection.ServiceAddress.Lines, ", "))
		}
	}
	return strings.Join(parts, " | ")
}

func (b *Books) inputStates() map[string]fileState {
	states := make(map[string]fileState)
	for path, state := range b.chartStates {
		states[path] = state
	}
	if b.Config.configPath != "" {
		states[b.Config.configPath] = b.Config.configState
	}
	if b.registry != nil {
		states[b.Config.Registry] = b.registryState
	}
	return states
}

func unchangedInputs(states map[string]fileState, chartPaths []string) error {
	for path, original := range states {
		label := "workspace input"
		if slices.Contains(chartPaths, path) {
			label = "account chart definition"
		}
		current, err := readNonsymlinkFileState(path, label)
		if err != nil {
			return err
		}
		if current.mode != original.mode || !bytes.Equal(current.data, original.data) {
			return fmt.Errorf("%s changed: %s; cancel and reopen Wat", label, path)
		}
	}
	return nil
}

func (b *Books) checkInputs() error {
	if b.registryError != nil {
		return b.registryError
	}
	return unchangedInputs(b.inputStates(), b.Config.ChartDefs)
}

func (b *Books) Customers() (CustomerRegistry, error) {
	if err := b.checkInputs(); err != nil {
		return CustomerRegistry{}, err
	}
	if b.registry == nil {
		return CustomerRegistry{}, fmt.Errorf("customer registry is disabled; run customer init, then set customer_registry: true in wat.yaml and restart")
	}
	registry, err := decodeRegistry(b.registryState.data, b.chart)
	if err != nil {
		return CustomerRegistry{}, err
	}
	return *registry, nil
}

func (b *Books) checkCustomerPayees(r CustomerRegistry) error {
	payees, err := b.declaredPayees()
	if err != nil {
		return err
	}
	for _, customer := range r.Customers {
		if !slices.Contains(payees, customer.Payee) {
			return fmt.Errorf("%s: declare payee %q in books/payees.journal before saving the customer", customer.ID, customer.Payee)
		}
	}
	return nil
}

func (b *Books) NewRegistryDraft() (*RegistryDraft, error) {
	registry, err := b.Customers()
	if err != nil {
		return nil, err
	}
	draft, err := b.NewDraft()
	if err != nil {
		return nil, err
	}
	draft.target = b.Config.LocalAccounts
	return &RegistryDraft{Registry: registry, books: b, draft: draft}, nil
}

func (d *RegistryDraft) AddCustomer(payee string, address []string) (string, error) {
	var ids []string
	for _, customer := range d.Registry.Customers {
		ids = append(ids, customer.ID)
	}
	id, err := nextRegistryID("C", ids)
	if err != nil {
		return "", err
	}
	d.Registry.Customers = append(d.Registry.Customers, Customer{id, payee, CustomerAddress{address}})
	return id, nil
}

func (d *RegistryDraft) AddConnection(name string, address []string) (string, error) {
	var ids []string
	for _, connection := range d.Registry.Connections {
		ids = append(ids, connection.ID)
	}
	id, err := nextRegistryID("CN", ids)
	if err != nil {
		return "", err
	}
	d.Registry.Connections = append(d.Registry.Connections, Connection{id, name, CustomerAddress{address}})
	return id, nil
}

func (d *RegistryDraft) OpenService(service ServiceAccount) (string, error) {
	customer, err := d.Registry.Customer(service.Customer)
	if err != nil {
		return "", err
	}
	connection, err := d.Registry.Connection(service.Connection)
	if err != nil {
		return "", err
	}
	var ids []string
	for _, existing := range d.Registry.ServiceAccounts {
		ids = append(ids, existing.ID)
	}
	id, err := nextRegistryID("SA", ids)
	if err != nil {
		return "", err
	}
	service.ID, service.Customer, service.Connection = id, customer.ID, connection.ID
	if service.ReceivableAccount == "" {
		service.ReceivableAccount = "Asset:141.000:Customers:" + id
	}
	d.Registry.ServiceAccounts = append(d.Registry.ServiceAccounts, service)
	return id, nil
}

func (d *RegistryDraft) Save() (string, error) {
	if d.saved {
		return "", fmt.Errorf("registry draft already saved")
	}
	b := d.books
	if err := d.Registry.validate(b.chart); err != nil {
		return "", err
	}
	for _, prior := range b.registry.Customers {
		if current, err := d.Registry.Customer(prior.ID); err != nil || current.ID != prior.ID {
			return "", fmt.Errorf("cannot remove customer %s", prior.ID)
		}
	}
	for _, prior := range b.registry.Connections {
		if current, err := d.Registry.Connection(prior.ID); err != nil || current.ID != prior.ID {
			return "", fmt.Errorf("cannot remove connection %s", prior.ID)
		}
	}
	for _, prior := range b.registry.ServiceAccounts {
		current, err := d.Registry.Service(prior.ID)
		identity := current
		identity.ServiceThrough, identity.FinalBilling = prior.ServiceThrough, prior.FinalBilling
		if err != nil || !reflect.DeepEqual(identity, prior) {
			return "", fmt.Errorf("existing service account %s cannot be removed or reassigned", prior.ID)
		}
		if prior.ServiceThrough != nil && !reflect.DeepEqual(current.ServiceThrough, prior.ServiceThrough) {
			return "", fmt.Errorf("service account %s is already closed; cannot change its service end date", prior.ID)
		}
	}
	if err := b.checkCustomerPayees(d.Registry); err != nil {
		return "", err
	}
	data, err := encodeRegistry(d.Registry)
	if err != nil {
		return "", err
	}
	candidate, err := decodeRegistry(data, b.chart)
	if err != nil {
		return "", err
	}
	cfg := b.Config
	cfg.registryAccounts = d.Registry.accounts()
	accounts := cfg.postingAccounts()
	for _, bank := range cfg.BankAccounts {
		accounts = append(accounts, bank.Account)
	}
	validate := func() error {
		return d.draft.withProposedBooks(generatedAccounts(cfg), func(journal string) error {
			proposed := *b
			proposed.registry = candidate
			if err := proposed.checkJournal(journal, accounts); err != nil {
				return err
			}
			transactions, err := proposed.transactions(journal)
			if err != nil {
				return err
			}
			for _, service := range candidate.ServiceAccounts {
				prior, _ := b.registry.Service(service.ID)
				if service.FinalBilling != nil && !reflect.DeepEqual(prior.FinalBilling, service.FinalBilling) {
					evidence, err := serviceEvidence(service, transactions)
					if err != nil {
						return err
					}
					if !reflect.DeepEqual(evidence, service.FinalBilling.Evidence) {
						return fmt.Errorf("%s: billing evidence changed; review and confirm again", service.ID)
					}
					for _, tx := range transactions {
						if service.touches(tx) && tx.Date > service.FinalBilling.ConfirmedOn {
							return fmt.Errorf("%s: confirmation date precedes recorded activity", service.ID)
						}
					}
				}
			}
			return nil
		})
	}
	backup, saved, err := saveSingleFile(cfg.Registry, b.registryState, data, d.draft.unchanged, validate)
	d.saved = saved
	if !saved {
		return backup, err
	}
	b.registryState = fileState{data: data, mode: b.registryState.mode}
	b.registry = candidate
	b.Config.registryAccounts = cfg.registryAccounts
	if generationErr := writeGeneratedFile(cfg.LocalAccounts, generatedAccounts(cfg)); generationErr != nil {
		b.registryError = fmt.Errorf("registry saved, but account generation failed; restart Wat to regenerate before continuing: %w", generationErr)
		err = errors.Join(err, b.registryError)
	}
	return backup, err
}

func (b *Books) DeclarePayee(payee string) (backup string, saved bool, err error) {
	if !validPayeeName(payee) {
		return "", false, fmt.Errorf("payee must be nonempty single-line text without ';' or '|'")
	}
	draft, err := b.NewDraft()
	if err != nil {
		return "", false, err
	}
	payees, err := b.declaredPayees()
	if err != nil {
		return "", false, err
	}
	if slices.Contains(payees, payee) {
		return "", false, fmt.Errorf("payee %q is already declared", payee)
	}
	draft.target = b.Config.Payees
	state, ok := draft.files[draft.target]
	if !ok {
		return "", false, fmt.Errorf("payee journal is not included in the books")
	}
	data := append(append([]byte(nil), state.data...), []byte("\npayee "+payee+"\n")...)
	return saveSingleFile(draft.target, state, data, draft.unchanged, func() error {
		return draft.withProposedBooks(data, func(journal string) error {
			return b.checkJournal(journal, draft.accounts)
		})
	})
}
