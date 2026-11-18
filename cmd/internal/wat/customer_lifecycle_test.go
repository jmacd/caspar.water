package wat

import (
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	serviceAR      = "Asset:141.000:Customers:SA-0001"
	serviceDeposit = "Liability:233.000:Customers:SA-0001"
	serviceIncome  = "Income:460.100:Service"
	serviceBank    = "Bank:131.200:Checking"
)

func closeTestService(t *testing.T, books *Books) {
	t.Helper()
	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	require.NoError(t, draft.CloseService("SA-0001", "2021-01-31"))
	_, err = draft.Save()
	require.NoError(t, err)
}

func confirmTestBilling(t *testing.T, books *Books, date string) {
	t.Helper()
	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	require.NoError(t, draft.ConfirmServiceBilling("SA-0001", "2021-01-31", date, "Reviewed full service coverage and approved policy evidence"))
	_, err = draft.Save()
	require.NoError(t, err)
}

func TestServiceClosureRetainsDebtAndDoesNotNetRoles(t *testing.T) {
	books := customerFixture(t)
	saveCustomerEntry(t, books, "2021-01-15", "Example Customer", "Service", []Posting{
		{serviceAR, "100"}, {serviceIncome, "-100"},
	})
	saveCustomerEntry(t, books, "2021-01-16", "Example Customer", "Security", []Posting{
		{serviceBank, "100"}, {serviceDeposit, "-100"},
	})
	journal := read(t, books.Config.ManualJournal)
	closeTestService(t, books)
	require.Equal(t, journal, read(t, books.Config.ManualJournal))
	status, err := books.ServiceSettlement("SA-0001", "2021-01-31")
	require.NoError(t, err)
	require.False(t, status.Settled)
	require.Contains(t, status.Reason, "not been confirmed")
	_, err = books.IssueClosingStatement("SA-0001", "2021-01-31")
	require.ErrorContains(t, err, "cannot issue")

	_, _, err = books.DeclarePayee("Next Occupant")
	require.NoError(t, err)
	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	customer, err := draft.AddCustomer("Next Occupant", []string{"100 Example Street"})
	require.NoError(t, err)
	_, err = draft.OpenService(ServiceAccount{Customer: customer, Connection: "CN-0001", ServiceFrom: "2021-02-01"})
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	next, err := books.ServiceSettlement("SA-0002", "2021-02-02")
	require.NoError(t, err)
	require.Zero(t, next.Balances[0].Cents)
	require.Equal(t, journal, read(t, books.Config.ManualJournal))

	confirmTestBilling(t, books, "2021-02-01")
	status, err = books.ServiceSettlement("SA-0001", "2021-02-01")
	require.NoError(t, err)
	require.True(t, status.BillingValid)
	require.False(t, status.Settled, "receivable and deposit cannot silently net to zero")
	require.Equal(t, int64(10000), status.Balances[0].Cents)
	require.Equal(t, int64(-10000), status.Balances[2].Cents)
	_, _, err = books.ServiceApplyDraft("SA-0001", "deposit", "101", "2021-02-02", "approved application")
	require.ErrorContains(t, err, "exceeds")
	application, text, err := books.ServiceApplyDraft("SA-0001", "deposit", "100", "2021-02-02", "approved application")
	require.NoError(t, err)
	require.NotContains(t, text, "Bank:")
	require.Equal(t, journal, read(t, books.Config.ManualJournal), "preview cannot change books")
	_, err = application.Save()
	require.NoError(t, err)
	status, err = books.ServiceSettlement("SA-0001", "2021-02-02")
	require.NoError(t, err)
	require.False(t, status.Settled, "saved account activity requires an explicit renewed billing review")
	require.Contains(t, status.Reason, "stale")
	confirmTestBilling(t, books, "2021-02-02")
	status, err = books.ServiceSettlement("SA-0001", "2021-02-02")
	require.NoError(t, err)
	require.True(t, status.Settled)
	_, _, err = books.ServiceApplyDraft("SA-0001", "deposit", "1", "2021-02-03", "approved")
	require.ErrorContains(t, err, "exceeds")

	draft, err = books.NewRegistryDraft()
	require.NoError(t, err)
	end := "2021-01-30"
	draft.Registry.ServiceAccounts[0].ServiceThrough = &end
	draft.Registry.ServiceAccounts[0].FinalBilling = nil
	_, err = draft.Save()
	require.ErrorContains(t, err, "already closed")

	saveCustomerEntry(t, books, "2021-02-04", "Example Customer", "Combined late receipt", []Posting{
		{serviceBank, "2"}, {serviceAR, "-1"}, {"Asset:141.000:Customers:SA-0002", "-1"},
	})
	require.NoError(t, books.Check(), "split cash receipts remain valid after one service closes")
	status, err = books.ServiceSettlement("SA-0001", "2021-02-04")
	require.NoError(t, err)
	require.False(t, status.Settled, "new activity immediately changes a previously settled result")
}

func TestServiceRefundAndImmutableIssuedStatements(t *testing.T) {
	books := customerFixture(t)
	saveCustomerEntry(t, books, "2021-01-15", "Example Customer", "Service", []Posting{
		{serviceAR, "100"}, {serviceIncome, "-100"},
	})
	saveCustomerEntry(t, books, "2021-01-16", "Example Customer", "Security", []Posting{
		{serviceBank, "150"}, {serviceDeposit, "-150"},
	})
	closeTestService(t, books)
	confirmTestBilling(t, books, "2021-01-31")
	before := read(t, books.Config.ManualJournal)
	issued, err := books.IssueClosingStatement("SA-0001", "2021-01-31")
	require.NoError(t, err)
	require.Equal(t, "statement-0001.txt", filepath.Base(issued))
	originalStatement := read(t, issued)
	require.Contains(t, originalStatement, "NOT SETTLED")
	require.Contains(t, originalStatement, "Billing address: 100 Example Street, Exampletown")
	require.Contains(t, originalStatement, "sha256:")
	require.Equal(t, before, read(t, books.Config.ManualJournal), "issuance cannot post a planned refund")
	info, err := os.Stat(issued)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0400), info.Mode().Perm())

	application, _, err := books.ServiceApplyDraft("SA-0001", "deposit", "100", "2021-02-01", "Approved deposit application")
	require.NoError(t, err)
	_, err = application.Save()
	require.NoError(t, err)
	// This is an actual disbursement already recorded from the bank, initially unallocated.
	disbursement := saveCustomerEntry(t, books, "2021-02-02", "Example Customer", "Actual refund", []Posting{
		{serviceBank, "-50"}, {"Equity:218.000:Owner", "50"},
	})
	before = read(t, books.Config.ManualJournal)
	refund, text, err := books.ServiceRefundDraft(disbursement, "SA-0001", "deposit")
	require.NoError(t, err)
	require.Contains(t, text, "id: "+disbursement)
	require.Equal(t, before, read(t, books.Config.ManualJournal))
	_, err = refund.Save()
	require.NoError(t, err)
	refund, _, err = books.ServiceRefundDraft(disbursement, "SA-0001", "deposit")
	require.NoError(t, err, "repeated allocation is a revision, not another cash posting")
	_, err = refund.Save()
	require.NoError(t, err)
	require.Equal(t, 1, strings.Count(read(t, books.Config.ManualJournal), "id: "+disbursement))
	cash, err := books.serviceBalance(serviceBank, "2021-02-02")
	require.NoError(t, err)
	require.Equal(t, int64(10000), cash)
	confirmTestBilling(t, books, "2021-02-02")
	status, err := books.ServiceSettlement("SA-0001", "2021-02-02")
	require.NoError(t, err)
	require.True(t, status.Settled)
	tooLarge := saveCustomerEntry(t, books, "2021-02-03", "Example Customer", "Not available for refund", []Posting{
		{serviceBank, "-1"}, {"Equity:218.000:Owner", "1"},
	})
	_, _, err = books.ServiceRefundDraft(tooLarge, "SA-0001", "deposit")
	require.ErrorContains(t, err, "exceeds the refundable balance")

	draft, err := books.NewRegistryDraft()
	require.NoError(t, err)
	draft.Registry.Customers[0].BillingAddress.Lines = []string{"Forwarding address"}
	_, err = draft.Save()
	require.NoError(t, err)
	corrected, err := books.IssueClosingStatement("SA-0001", "2021-02-02")
	require.NoError(t, err)
	require.Equal(t, "statement-0002.txt", filepath.Base(corrected))
	require.Contains(t, read(t, corrected), "Forwarding address")
	require.Contains(t, read(t, corrected), "SA-0001 as of 2021-02-02: SETTLED")
	require.Equal(t, originalStatement, read(t, issued), "later postings and address edits cannot alter an issued statement")

	reopened, err := Open(books.Config)
	require.NoError(t, err)
	status, err = reopened.ServiceSettlement("SA-0001", "2021-02-02")
	require.NoError(t, err)
	require.True(t, status.Settled, "settlement and evidence must survive reopening ordinary files")
	require.Equal(t, originalStatement, read(t, issued))
}

func TestServiceFinalChargeCoverageAndBillingEvidence(t *testing.T) {
	books := customerFixture(t)
	closeTestService(t, books)
	confirmTestBilling(t, books, "2021-01-31")
	journal := read(t, books.Config.ManualJournal)
	draft, err := books.NewDraft()
	require.NoError(t, err)
	_, err = draft.Preview(Entry{
		Date: "2021-02-01", Payee: "Example Customer", Description: "Unsupported postclosure charge",
		Postings: []Posting{{serviceAR, "10"}, {serviceIncome, "-10"}},
	})
	require.NoError(t, err)
	_, err = draft.Save()
	require.ErrorContains(t, err, "service-through")
	require.Equal(t, journal, read(t, books.Config.ManualJournal))
	_, _, err = books.ServiceAdjustmentDraft("SA-0001", "2021-02-01", "2021-02-01", "10", serviceIncome, "reviewed")
	require.ErrorContains(t, err, "within the service period")
	_, _, err = books.ServiceAdjustmentDraft("SA-0001", "2021-02-01", "2021-01-31", "10", serviceBank, "reviewed")
	require.ErrorContains(t, err, "bank movement")
	charge, text, err := books.ServiceAdjustmentDraft("SA-0001", "2021-02-01", "2021-01-31", "10", serviceIncome, "Approved final charge")
	require.NoError(t, err)
	require.Contains(t, text, "; service-through: 2021-01-31")
	_, err = charge.Save()
	require.NoError(t, err)
	status, err := books.ServiceSettlement("SA-0001", "2021-02-01")
	require.NoError(t, err)
	require.Contains(t, status.Reason, "stale")

	registry, err := books.NewRegistryDraft()
	require.NoError(t, err)
	require.NoError(t, registry.ConfirmServiceBilling("SA-0001", "2021-01-31", "2021-01-31", "Reviewed"))
	_, err = registry.Save()
	require.ErrorContains(t, err, "precedes recorded activity")
	confirmTestBilling(t, books, "2021-02-01")
	edited, entry, err := books.EditDraft(charge.id)
	require.NoError(t, err)
	entry.Postings[0].Amount, entry.Postings[1].Amount = "20", "-20"
	_, err = edited.Preview(entry)
	require.NoError(t, err)
	_, err = edited.Save()
	require.NoError(t, err)
	status, err = books.ServiceSettlement("SA-0001", "2021-02-01")
	require.NoError(t, err)
	require.Contains(t, status.Reason, "stale", "a changed referenced charge invalidates confirmation")
	require.Equal(t, int64(2000), status.Balances[0].Cents)
	registry, err = books.NewRegistryDraft()
	require.NoError(t, err)
	require.NoError(t, registry.ConfirmServiceBilling("SA-0001", "2021-01-31", "2021-02-02", "Reviewed"))
	saveCustomerEntry(t, books, "2021-02-02", "Example Customer", "Late receipt", []Posting{
		{serviceBank, "20"}, {serviceAR, "-20"},
	})
	_, err = registry.Save()
	require.ErrorContains(t, err, "journal changed")
	confirmTestBilling(t, books, "2021-02-02")
	status, err = books.ServiceSettlement("SA-0001", "2021-02-02")
	require.NoError(t, err)
	require.True(t, status.Settled)
}

func TestServiceEvidenceRequiresUniqueIDsAndExactUSD(t *testing.T) {
	books := customerFixture(t)
	registry, err := books.NewRegistryDraft()
	require.NoError(t, err)
	registry.Registry.ServiceAccounts[0].FinalBilling = &BillingConfirmation{
		Through: "2021-01-31", ConfirmedOn: "2021-01-31", Review: "Reviewed", Evidence: []BillingEvidence{},
	}
	_, err = registry.Save()
	require.ErrorContains(t, err, "final billing requires closure", "registry validation must reject confirmation on open service")
	closeTestService(t, books)
	journal := `2021-01-15 Example Customer | Service
    ; id: same
    Asset:141.000:Customers:SA-0001  $10
    Income:460.100:Service  $-10
`
	for _, invalid := range []string{
		strings.Replace(journal, "    ; id: same\n", "", 1),
		journal + "\n" + journal,
	} {
		require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(invalid), 0600))
		draft, err := books.NewRegistryDraft()
		require.NoError(t, err)
		err = draft.ConfirmServiceBilling("SA-0001", "2021-01-31", "2021-01-31", "Reviewed")
		require.ErrorContains(t, err, "unique nonempty id")
	}
	fractional := strings.ReplaceAll(strings.ReplaceAll(journal, "$10", "$10.001"), "$-10", "$-10.001")
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(fractional), 0600))
	require.ErrorContains(t, books.Check(), "fractional cents")
	require.Equal(t, "-92233720368547758.08", formatCents(math.MinInt64))
}

func TestServiceRefundAndWorkspaceRestorePreserveEvidenceAndLock(t *testing.T) {
	books, statement, _ := statementFixture(t)
	books = enableTestCustomers(t, books)
	registry, err := books.NewRegistryDraft()
	require.NoError(t, err)
	_, err = registry.AddCustomer("Electric company", []string{"Customer address"})
	require.NoError(t, err)
	_, err = registry.AddConnection("House", []string{"Service address"})
	require.NoError(t, err)
	_, err = registry.OpenService(ServiceAccount{
		Customer: "C-0001", Connection: "CN-0001", ServiceFrom: "2021-01-01", DepositAccount: serviceDeposit,
	})
	require.NoError(t, err)
	_, err = registry.Save()
	require.NoError(t, err)
	closeTestService(t, books)
	var refundID string
	for index := range statement.Rows {
		draft, entry, err := books.StatementDraft(statement, index)
		require.NoError(t, err)
		entry.Payee, entry.Description = "Journal", "Reviewed bank row"
		entry.Postings[1].Account = "Equity:218.000:Owner"
		if index == 0 {
			entry.Postings[1].Account = serviceDeposit
		}
		_, err = draft.Preview(entry)
		require.NoError(t, err)
		_, err = draft.Save()
		require.NoError(t, err)
		if index == 1 {
			refundID = draft.id
		}
	}
	before, err := books.transactions(books.Config.Journal)
	require.NoError(t, err)
	draft, _, err := books.ServiceRefundDraft(refundID, "SA-0001", "deposit")
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	after, err := books.transactions(books.Config.Journal)
	require.NoError(t, err)
	require.Len(t, after, len(before))
	for _, tag := range []string{"id", "bank-row", "bank-description", "source"} {
		require.Equal(t, transactionTag(before[1], tag), transactionTag(after[1], tag))
	}
	require.Equal(t, before[1].Date, after[1].Date)
	require.NoError(t, books.FinishReconciliation(statement))
	_, _, err = books.ServiceRefundDraft(refundID, "SA-0001", "deposit")
	require.ErrorContains(t, err, "locked by finished bank reconciliation")

	confirmTestBilling(t, books, "2021-05-31")
	status, err := books.ServiceSettlement("SA-0001", "2021-05-31")
	require.NoError(t, err)
	require.True(t, status.BillingValid)
	require.False(t, status.Settled, "unrefunded deposit must remain a customer obligation")
	require.Equal(t, int64(-8766), status.Balances[1].Cents)
	report, err := books.ServiceStatement("SA-0001", "", "2021-05-31")
	require.NoError(t, err)
	balance, err := books.run(books.Config.Journal, "balance", "--output-format=json")
	require.NoError(t, err)
	issued, err := books.IssueClosingStatement("SA-0001", "2021-05-31")
	require.NoError(t, err)
	issuedText := read(t, issued)
	issuedRelative, err := filepath.Rel(books.Config.root, issued)
	require.NoError(t, err)
	registryText := read(t, books.Config.Registry)

	backup, restoredRoot := t.TempDir(), t.TempDir()
	copyCustomerTestWorkspace(t, books.Config.root, backup)
	require.NoError(t, os.Rename(books.Config.root, filepath.Join(t.TempDir(), "offline-original")))
	require.NoDirExists(t, books.Config.root)
	copyCustomerTestWorkspace(t, backup, restoredRoot)
	config, err := LoadConfig(filepath.Join(restoredRoot, "wat.yaml"))
	require.NoError(t, err)
	require.NoError(t, os.Remove(config.LocalAccounts))
	require.NoError(t, os.Remove(config.Journal))
	restored, err := Open(config)
	require.NoError(t, err, "restoring must not depend on original absolute paths or saved generated declarations")
	require.NoError(t, restored.Check())
	require.Equal(t, registryText, read(t, restored.Config.Registry))
	restoredStatus, err := restored.ServiceSettlement("SA-0001", "2021-05-31")
	require.NoError(t, err)
	require.Equal(t, status, restoredStatus, "relocation must preserve billing evidence and outstanding obligations")
	restoredReport, err := restored.ServiceStatement("SA-0001", "", "2021-05-31")
	require.NoError(t, err)
	require.Equal(t, report, restoredReport)
	restoredBalance, err := restored.run(restored.Config.Journal, "balance", "--output-format=json")
	require.NoError(t, err)
	require.Equal(t, balance, restoredBalance, "ordinary hledger must read the restored books directly")
	restoredIssue := filepath.Join(restored.Config.root, issuedRelative)
	require.Equal(t, issuedText, read(t, restoredIssue))
	info, err := os.Stat(restoredIssue)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0400), info.Mode().Perm())
	statements, err := restored.CachedStatements()
	require.NoError(t, err)
	require.Len(t, statements, 1)
	require.Equal(t, statement.Hash, statements[0].Hash)
	reconciliation, err := restored.Reconciliation(statements[0])
	require.NoError(t, err)
	require.NotNil(t, reconciliation.Checkpoint)
	_, _, err = restored.ServiceRefundDraft(refundID, "SA-0001", "deposit")
	require.ErrorContains(t, err, "locked by finished bank reconciliation")
	nextIssue, err := restored.IssueClosingStatement("SA-0001", "2021-05-31")
	require.NoError(t, err)
	require.Equal(t, "statement-0002.txt", filepath.Base(nextIssue))
	require.Equal(t, issuedText, read(t, restoredIssue), "restored issuance cannot overwrite its predecessor")
}

func copyCustomerTestWorkspace(t *testing.T, source, destination string) {
	t.Helper()
	require.NoError(t, filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("nonregular synthetic workspace file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode().Perm())
	}))
}
