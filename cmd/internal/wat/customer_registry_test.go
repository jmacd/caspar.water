package wat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func customerTestChart() string {
	return strings.Replace(testChart, "  Equity:", `    - number: "141"
      name: Customer-Receivables
  Equity:`, 1) + `    - number: "460.1"
      name: Unmetered-Units
  Liability:
    - number: "233"
      name: Customer-Deposits
    - number: "241"
      name: Other-Current-Liabilities
`
}

func enableTestCustomers(t *testing.T, books *Books) *Books {
	t.Helper()
	configPath := filepath.Join(books.Config.root, "wat.yaml")
	require.NoError(t, os.WriteFile(books.Config.ChartDefs[0], []byte(customerTestChart()), 0600))
	config := read(t, configPath) + "\ncustomer_registry: true\n"
	config = strings.Replace(config, "accounts:\n", "accounts:\n  - Income:460.100:Service\n", 1)
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0600))
	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.NoError(t, InitCustomerRegistry(cfg))
	books, err = Open(cfg)
	require.NoError(t, err)
	return books
}

func customerFixture(t *testing.T) *Books {
	t.Helper()
	books, _ := fixture(t, "")
	books = enableTestCustomers(t, books)
	_, saved, err := books.DeclarePayee("Example Customer")
	require.NoError(t, err)
	require.True(t, saved)
	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	_, err = draft.AddCustomer("Example Customer", []string{"100 Example Street", "Exampletown"})
	require.NoError(t, err)
	_, err = draft.AddConnection("House", []string{"100 Example Street"})
	require.NoError(t, err)
	_, err = draft.OpenService(ServiceAccount{
		Customer: "C-0001", Connection: "CN-0001", ServiceFrom: "2021-01-01",
		CreditAccount:  "Liability:241.000:Customers:SA-0001",
		DepositAccount: "Liability:233.000:Customers:SA-0001",
	})
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	return books
}

func TestCustomerRegistryStrictFormat(t *testing.T) {
	chart, err := parseAccountChart([]byte(customerTestChart()))
	require.NoError(t, err)
	const valid = `version: 1
customers:
  - id: C-0001
    payee: Example Customer
    billing_address:
      lines: [Mailing address]
connections:
  - id: CN-0001
    name: House
    service_address:
      lines: [Service address]
service_accounts:
  - id: SA-0001
    customer: C-0001
    connection: CN-0001
    service_from: "2021-01-01"
    service_through: "2021-06-30"
    receivable_account: Asset:141.000:Customers:SA-0001
`
	for _, tc := range []struct{ name, text, message string }{
		{"unknown field", valid + "balance: 100\n", "field balance"},
		{"duplicate key", valid + "version: 1\n", "already defined"},
		{"second document", valid + "---\nversion: 1\n", "exactly one"},
		{"missing list", strings.Replace(valid, "connections:", "connections: null\nunused:", 1), "field unused"},
		{"missing version", strings.Replace(valid, "version: 1\n", "", 1), "version 1"},
		{"bad date", strings.Replace(valid, "2021-01-01", "2021-02-30", 1), "valid YYYY-MM-DD"},
		{"reverse dates", strings.Replace(valid, "2021-06-30", "2020-06-30", 1), "on or after"},
		{"unknown customer", strings.Replace(valid, "customer: C-0001", "customer: C-9999", 1), "unknown customer"},
		{"wrong role", strings.Replace(valid, "Asset:141.000", "Liability:233.000", 1), "incompatible CPUC"},
		{"empty address", strings.Replace(valid, "[Mailing address]", "[]", 1), "at least one"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decodeRegistry([]byte(tc.text), chart)
			require.ErrorContains(t, err, tc.message)
		})
	}
	registry, err := decodeRegistry([]byte(valid), chart)
	require.NoError(t, err)
	next := registry.ServiceAccounts[0]
	next.ID = "SA-0002"
	next.ReceivableAccount = "Asset:141.000:Customers:SA-0002"
	next.ServiceFrom = "2021-06-30"
	next.ServiceThrough = nil
	registry.ServiceAccounts = append(registry.ServiceAccounts, next)
	require.ErrorContains(t, registry.validate(chart), "overlaps")
	registry.ServiceAccounts[1].ServiceFrom = "2021-07-01"
	require.NoError(t, registry.validate(chart), "a new account may start the day after the old one closes")
	registry.ServiceAccounts[1].ReceivableAccount = registry.ServiceAccounts[0].ReceivableAccount + ":Child"
	require.ErrorContains(t, registry.validate(chart), "overlaps ownership")
	registry.ServiceAccounts = registry.ServiceAccounts[:1]
	registry.Customers = append(registry.Customers, registry.Customers[0])
	require.ErrorContains(t, registry.validate(chart), "duplicate registry ID")
	registry.Customers[1].ID = "C-0002"
	_, err = registry.Customer("Example Customer")
	require.ErrorContains(t, err, "matches 2")
}

func TestCustomerSetupSavesOnlyRegistryAndDerivedAccounts(t *testing.T) {
	books := customerFixture(t)
	registryBytes := read(t, books.Config.Registry)
	require.Empty(t, read(t, books.Config.ManualJournal))
	require.Empty(t, read(t, books.Config.BankAccounts[0].Journal))
	require.Contains(t, read(t, books.Config.LocalAccounts), "account Asset:141.000:Customers:SA-0001")
	require.NoError(t, books.Check())
	require.ErrorContains(t, InitCustomerRegistry(books.Config), "will not overwrite")
	require.Equal(t, registryBytes, read(t, books.Config.Registry))

	stale, err := books.NewDraft()
	require.NoError(t, err)
	_, err = stale.Preview(testEntry())
	require.NoError(t, err)
	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	draft.Registry.Customers[0].BillingAddress.Lines = []string{"Updated mailing address"}
	backup, err := draft.Save()
	require.NoError(t, err)
	require.Equal(t, registryBytes, read(t, backup))
	_, err = stale.Save()
	require.ErrorContains(t, err, "input changed", "journal drafts must include registry content, not only generated declarations")
	_, err = draft.Save()
	require.ErrorContains(t, err, "already saved")
	draft.Registry.Customers[0].Payee = "mutating a saved draft"
	registry, err := books.Customers()
	require.NoError(t, err)
	require.Equal(t, "Example Customer", registry.Customers[0].Payee)

	// Rebuildable declarations may lag an authoritative registry save after interruption.
	require.NoError(t, os.WriteFile(books.Config.LocalAccounts, []byte("commodity $\n"), 0600))
	cfg, err := LoadConfig(books.Config.configPath)
	require.NoError(t, err)
	reopened, err := Open(cfg)
	require.NoError(t, err)
	require.Contains(t, read(t, reopened.Config.LocalAccounts), "Asset:141.000:Customers:SA-0001")
	registry, err = reopened.Customers()
	require.NoError(t, err)
	require.Equal(t, []string{"Updated mailing address"}, registry.Customers[0].BillingAddress.Lines)
	require.Empty(t, read(t, reopened.Config.ManualJournal))
}

func TestCustomerRegistryRejectsInvalidAndStaleSaves(t *testing.T) {
	books := customerFixture(t)
	original := read(t, books.Config.Registry)
	for _, tc := range []struct {
		name, message string
		change        func(*RegistryDraft)
	}{
		{"undeclared payee", "declare payee", func(d *RegistryDraft) { d.Registry.Customers[0].Payee = "Unknown customer" }},
		{"removed customer", "unknown customer", func(d *RegistryDraft) { d.Registry.Customers = []Customer{} }},
		{"reassigned service", "cannot be removed or reassigned", func(d *RegistryDraft) { d.Registry.ServiceAccounts[0].ServiceFrom = "2020-01-01" }},
		{"duplicate ownership", "overlaps ownership", func(d *RegistryDraft) {
			d.Registry.Connections = append(d.Registry.Connections, Connection{"CN-0002", "Second house", CustomerAddress{[]string{"Second address"}}})
			service := d.Registry.ServiceAccounts[0]
			service.ID, service.Connection = "SA-0002", "CN-0002"
			d.Registry.ServiceAccounts = append(d.Registry.ServiceAccounts, service)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			draft, err := books.NewRegistryDraft()
			require.NoError(t, err)
			tc.change(draft)
			_, err = draft.Save()
			require.ErrorContains(t, err, tc.message)
			require.Equal(t, original, read(t, books.Config.Registry))
		})
	}
	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	lock := books.Config.Registry + ".wat-lock"
	require.NoError(t, os.WriteFile(lock, nil, 0600))
	_, err = draft.Save()
	require.ErrorContains(t, err, "lock")
	require.NoError(t, os.Remove(lock))
	require.NoError(t, os.WriteFile(books.Config.Registry, []byte(original+"# concurrent edit\n"), 0600))
	_, err = draft.Save()
	require.ErrorContains(t, err, "input changed")
	require.Contains(t, read(t, books.Config.Registry), "# concurrent edit")
}

func TestCustomerRegistryMissingOrSymlinkedNeverDisablesFeature(t *testing.T) {
	books := customerFixture(t)
	accounts := read(t, books.Config.LocalAccounts)
	original := books.Config.Registry + ".original"
	require.NoError(t, os.Rename(books.Config.Registry, original))
	_, err := Open(books.Config)
	require.ErrorContains(t, err, "customer registry")
	require.Equal(t, accounts, read(t, books.Config.LocalAccounts))
	require.NoError(t, os.Symlink(original, books.Config.Registry))
	_, err = Open(books.Config)
	require.ErrorContains(t, err, "nonsymlink")
	require.Equal(t, accounts, read(t, books.Config.LocalAccounts))
}

func TestPayeeDeclarationIsSeparateFromCustomerSave(t *testing.T) {
	books := customerFixture(t)
	original := read(t, books.Config.Registry)
	backup, saved, err := books.DeclarePayee("Another Customer")
	require.NoError(t, err)
	require.True(t, saved)
	require.NotContains(t, read(t, backup), "Another Customer")
	require.Equal(t, original, read(t, books.Config.Registry))
	require.Empty(t, read(t, books.Config.ManualJournal))
	_, _, err = books.DeclarePayee("Another Customer")
	require.ErrorContains(t, err, "already declared")
	_, _, err = books.DeclarePayee("Invalid\ninclude other.journal")
	require.Error(t, err)
}

func saveCustomerEntry(t *testing.T, books *Books, date, payee, memo string, postings []Posting) string {
	t.Helper()
	draft, err := books.NewDraft()
	require.NoError(t, err)
	_, err = draft.Preview(Entry{Date: date, Payee: payee, Description: memo, Postings: postings})
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	return draft.id
}

func TestServicePaymentAllocationAndDatedStatement(t *testing.T) {
	books := customerFixture(t)
	receivable := "Asset:141.000:Customers:SA-0001"
	saveCustomerEntry(t, books, "2021-01-15", "Example Customer", "Service charge", []Posting{
		{receivable, "150.00"}, {"Income:460.100:Service", "-150.00"},
	})
	id := saveCustomerEntry(t, books, "2021-02-01", "Deposit", "Customer payment", []Posting{
		{"Bank:131.200:Checking", "100.00"}, {"Income:421.000:Misc", "-100.00"},
	})
	original := read(t, books.Config.ManualJournal)
	for _, allocations := range [][]string{{"SA-0001=99"}, {"SA-0001=100", "SA-0001=1"}, {"SA-9999=100"}, {"SA-0001=-100"}, {"SA-0001:unknown=100"}} {
		_, _, err := books.ServicePaymentDraft(id, allocations)
		require.Error(t, err)
		require.Equal(t, original, read(t, books.Config.ManualJournal))
	}
	draft, preview, err := books.ServicePaymentDraft(id, []string{"SA-0001=100"})
	require.NoError(t, err)
	require.Contains(t, preview, receivable)
	require.Equal(t, original, read(t, books.Config.ManualJournal), "preview does not save")
	_, err = draft.Save()
	require.NoError(t, err)
	retry, _, err := books.ServicePaymentDraft(id, []string{"SA-0001=100"})
	require.NoError(t, err)
	_, err = retry.Save()
	require.NoError(t, err)
	transactions, err := books.transactions(books.Config.Journal)
	require.NoError(t, err)
	require.Len(t, transactions, 2)
	bank, err := bankAmount(transactions[1], "Bank:131.200:Checking")
	require.NoError(t, err)
	require.Equal(t, int64(10000), bank)
	require.Equal(t, id, transactionTag(transactions[1], "id"))
	saveCustomerEntry(t, books, "2021-03-01", "Example Customer", "Later charge", []Posting{
		{receivable, "20.00"}, {"Income:460.100:Service", "-20.00"},
	})
	report, err := books.ServiceStatement("SA-0001", "2021-02-01", "2021-02-28")
	require.NoError(t, err)
	require.Contains(t, report, "Opening balance: $150.00")
	require.Contains(t, report, "Closing balance: $50.00")
	require.Contains(t, report, "Customer payment")
	require.NotContains(t, report, "Later charge")
	require.Contains(t, report, "liability")
	require.Contains(t, report, "billing completeness")
	require.NoError(t, books.Check())
}

func TestServicePaymentSplitsAndRetainsReconciliationLock(t *testing.T) {
	books, statement, _ := statementFixture(t)
	books = enableTestCustomers(t, books)
	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	_, err = draft.AddCustomer("Electric company", []string{"Address"})
	require.NoError(t, err)
	for _, house := range []string{"House one", "House two"} {
		connection, err := draft.AddConnection(house, []string{house})
		require.NoError(t, err)
		_, err = draft.OpenService(ServiceAccount{Customer: "C-0001", Connection: connection, ServiceFrom: "2021-01-01"})
		require.NoError(t, err)
	}
	_, err = draft.Save()
	require.NoError(t, err)
	var receiptID string
	for index := range statement.Rows {
		draft, entry, err := books.StatementDraft(statement, index)
		require.NoError(t, err)
		entry.Payee, entry.Description = "Journal", "Reviewed bank row"
		entry.Postings[1].Account = "Equity:218.000:Owner"
		_, err = draft.Preview(entry)
		require.NoError(t, err)
		_, err = draft.Save()
		require.NoError(t, err)
		if index == 0 {
			receiptID = draft.id
		}
	}
	before, err := books.transactions(books.Config.Journal)
	require.NoError(t, err)
	allocated, _, err := books.ServicePaymentDraft(receiptID, []string{"SA-0001=60.00", "SA-0002=40.00"})
	require.NoError(t, err)
	_, err = allocated.Save()
	require.NoError(t, err)
	after, err := books.transactions(books.Config.Journal)
	require.NoError(t, err)
	require.Len(t, after, len(before))
	for _, tag := range []string{"id", "bank-row", "bank-description", "source"} {
		require.Equal(t, transactionTag(before[0], tag), transactionTag(after[0], tag))
	}
	require.Equal(t, before[0].Date, after[0].Date)
	require.Len(t, after[0].Postings, 3)
	require.NoError(t, books.FinishReconciliation(statement))
	_, _, err = books.ServicePaymentDraft(receiptID, []string{"SA-0001=100"})
	require.ErrorContains(t, err, "locked by finished bank reconciliation")
}
