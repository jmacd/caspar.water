package wat

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const sampleRatePolicy = `version: 1
bill_from: 2021-12
billable_expenses: [Expense:615.000:Utility]
revenue_account: Income:460.100:Reconstruction
`

func TestBillingCloseFromAccrualsNotPayments(t *testing.T) {
	b := customerFixture(t)
	const liability = "Liability:241.000:Accrued"
	path := b.Config.configPath
	cfgText := strings.Replace(read(t, path), "accounts:\n", "accounts:\n  - "+liability+"\n  - Expense:615.001:Excluded\n", 1)
	require.NoError(t, os.WriteFile(path, []byte(cfgText), 0600))
	cfg, err := LoadConfig(path)
	require.NoError(t, err)
	b, err = Open(cfg)
	require.NoError(t, err)
	const journal = `2021-11-30 Journal | Prior services
    Expense:615.000:Utility  $5
    Liability:241.000:Accrued  $-5

2021-12-31 Journal | Service now, pay next month
    Expense:615.000:Utility  $30
    Liability:241.000:Accrued  $-30

2021-12-31 Journal | Credit on included expense and excluded cost
    Expense:615.000:Utility  $-10
    Expense:615.001:Excluded  $20
    Bank:131.200:Checking  $-10

2021-12-31 Deposit | Receipt is not a bill
    Bank:131.200:Checking  $50
    Equity:218.000:Owner  $-50

2022-01-03 Journal | Settle liability
    Liability:241.000:Accrued  $35
    Bank:131.200:Checking  $-35
`
	require.NoError(t, os.WriteFile(b.Config.ManualJournal, []byte(journal), 0600))
	policyPath := filepath.Join(b.Config.root, "policy.yaml")
	require.NoError(t, os.WriteFile(policyPath, []byte(sampleRatePolicy), 0600))
	opt := BillingOptions{Policy: policyPath, MarginPercent: "5", DefaultWeight: -1, Weights: []string{"CN-0001=2"}, UnbilledWeight: 1}
	before, err := b.BillingClose("2021-11", opt)
	require.NoError(t, err)
	require.Contains(t, before, "Unassigned cost: 5.00; no markup or carry-forward")
	require.Contains(t, before, "billing-count: 0")
	preview, err := b.BillingClose("2021-12", opt)
	require.NoError(t, err)
	for _, line := range []string{
		liability + "  -5.00 / -30.00 / -35.00", "Expense:615.001:Excluded  20.00 excluded",
		"Billable cost: 20.00; markup at 5.00%: 1.00; pool: 21.00", "Assessed: 14.00; unassigned: 7.00",
		"Asset:141.000:Customers:SA-0001  $14.00", "Income:460.100:Reconstruction  $-14.00",
	} {
		require.Contains(t, preview, line)
	}
	require.NotContains(t, preview, "Bank:")
	jan, err := b.BillingClose("2022-01", opt)
	require.NoError(t, err)
	require.Contains(t, jan, liability+"  -35.00 / 35.00 / 0.00")
	require.Contains(t, jan, "Billable cost: 0.00")
	require.Contains(t, jan, "billing-count: 0")
	opt.Output = filepath.Join(t.TempDir(), "assessment.journal")
	out, err := b.BillingClose("2021-12", opt)
	require.NoError(t, err)
	require.Equal(t, preview, out)
	require.Equal(t, preview, read(t, opt.Output))
	// Verify hledger can read the supplemental journal with the live declarations.
	list, err := b.run(b.Config.Journal, "-f", opt.Output, "check", "--strict", "payees")
	require.NoError(t, err, list)
	_, err = b.BillingClose("2021-12", opt)
	require.ErrorIs(t, err, os.ErrExist)
	opt.Output = filepath.Join(b.Config.root, "review.journal")
	_, err = b.BillingClose("2021-12", opt)
	require.ErrorContains(t, err, "outside the live workspace")
	require.NoFileExists(t, opt.Output)
	require.Equal(t, journal, read(t, b.Config.ManualJournal))
	// Simulate an operator explicitly including a reviewed journal later.
	require.NoError(t, os.WriteFile(b.Config.ManualJournal, []byte(journal+"\n"+preview), 0600))
	_, err = b.BillingClose("2021-12", opt)
	require.ErrorContains(t, err, "already appear")
}

func TestBillingAllocationEligibilityAndCents(t *testing.T) {
	end := "2021-11-30"
	r := CustomerRegistry{Connections: []Connection{{ID: "CN-0002"}, {ID: "CN-0001"}, {ID: "CN-0003"}},
		ServiceAccounts: []ServiceAccount{
			{ID: "SA-0001", Connection: "CN-0001", ServiceFrom: "2021-01-01"},
			{ID: "SA-0002", Connection: "CN-0002", ServiceFrom: "2021-11-01"},
			{ID: "SA-0003", Connection: "CN-0003", ServiceFrom: "2021-01-01", ServiceThrough: &end},
		}}
	m := billingSpan{first: "2021-12-01", last: "2021-12-31"}
	opt := BillingOptions{DefaultWeight: 1, Weights: []string{"CN-0003=1"}}
	shares, err := billingShares(&r, m, opt)
	require.NoError(t, err)
	require.Empty(t, shares[2].service.ID, "a weight for a vacant connection stays unassigned")
	for _, tc := range []struct {
		total int64
		want  []int64
	}{
		{5, []int64{2, 2, 1}}, {-2, []int64{-1, -1, 0}}, {0, []int64{0, 0, 0}},
	} {
		require.NoError(t, distributeBilling(tc.total, shares))
		require.Equal(t, tc.want, []int64{shares[0].cents, shares[1].cents, shares[2].cents})
	}
	for _, specs := range [][]string{{"CN-Z=3"}, {"CN-0001=1", "CN-0001=2"}, {"CN-0001=-1"}, {"CN-0001=half"}} {
		opt.Weights = specs
		_, err := billingShares(&r, m, opt)
		require.Error(t, err)
	}
	opt.Weights, opt.DefaultWeight = nil, -1
	_, err = billingShares(&r, m, opt)
	require.ErrorContains(t, err, "missing weight")
	opt.DefaultWeight = 1
	r.ServiceAccounts[1].ServiceFrom = "2021-12-15"
	_, err = billingShares(&r, m, opt)
	require.ErrorContains(t, err, "--through 2021-12-14")
	for _, n := range []int64{-10, 10} {
		markup, err := billingMarkup(n, 500)
		require.NoError(t, err)
		require.Equal(t, n/10, markup, "half-cent markup rounds away from zero")
	}
	_, err = billingMarkup(math.MaxInt64, math.MaxInt64)
	require.Error(t, err)
}

func TestBillingPolicyFailsClosed(t *testing.T) {
	b := customerFixture(t)
	for _, data := range []string{
		sampleRatePolicy + "\ntypo: true\n",
		sampleRatePolicy + "\n---\n{}\n",
		strings.Replace(sampleRatePolicy, "Expense:615.000:Utility", "Expense:615.000:*", 1),
		strings.Replace(sampleRatePolicy, "Expense:615.000:Utility", "Bank:131.200:Checking", 1),
		strings.Replace(sampleRatePolicy, "Income:460.100", "Income:999.000", 1),
		sampleRatePolicy + "\nrevenue_by_connection: {CN-9999: Income:460.100:Service}\n",
	} {
		_, err := b.billingPolicy([]byte(data))
		require.Error(t, err, data)
	}
}

func TestBillingCutoffPreviewApplyAndContinue(t *testing.T) {
	b := customerFixture(t)
	const before = `2021-11-30 Journal | Before the rate policy
    Expense:615.000:Utility  $70
    Equity:218.000:Owner  $-70

2021-12-01 Journal | Accrued edge of billing period
    Expense:615.000:Utility  $11
    Equity:218.000:Owner  $-11

2021-12-15 Journal | Credit on cutoff
    Expense:615.000:Utility  $-1
    Equity:218.000:Owner  $1

2021-12-16 Journal | Next day
    Expense:615.000:Utility  $3
    Equity:218.000:Owner  $-3
`
	require.NoError(t, os.WriteFile(b.Config.ManualJournal, []byte(before), 0600))
	path := filepath.Join(b.Config.root, "policy.yaml")
	require.NoError(t, os.WriteFile(path, []byte(sampleRatePolicy), 0600))
	options := BillingOptions{Policy: path, MarginPercent: "0", DefaultWeight: 1}
	d := BillingDates{From: "2021-01-15", Through: "2021-12-15"}
	_, err := b.BillingReview(BillingDates{Through: d.Through})
	require.ErrorContains(t, err, "no billing cutoff")
	review, err := b.BillingReview(d)
	require.NoError(t, err)
	require.Contains(t, review, "80.00  (charges: 2, credits: 1)")
	tokenFrom := func(preview string) string {
		t.Helper()
		line, _, _ := strings.Cut(preview, "\n")
		require.True(t, strings.HasPrefix(line, "; Review token: "))
		return strings.TrimPrefix(line, "; Review token: ")
	}
	preview, err := b.BillingClosePeriod(d, options)
	require.NoError(t, err)
	require.Contains(t, preview, "Unassigned cost: 70.00")
	require.Contains(t, preview, "Assessed: 10.00; unassigned: 0.00")
	require.Contains(t, preview, "billing-count: 1")
	options.Apply = tokenFrom(preview)
	options.MarginPercent = "5"
	_, err = b.BillingClosePeriod(d, options)
	require.ErrorContains(t, err, "review token does not match")
	require.Equal(t, before, read(t, b.Config.ManualJournal))
	options.MarginPercent = "0"
	result, err := b.BillingClosePeriod(d, options)
	require.NoError(t, err)
	require.Contains(t, result, "Next unbilled day: 2021-12-16")
	require.Contains(t, read(t, b.Config.ManualJournal), preview)
	_, err = b.BillingClosePeriod(d, options)
	require.ErrorContains(t, err, "must start 2021-12-16")
	options.Apply = ""
	_, err = b.BillingClosePeriod(BillingDates{From: "2021-12-17", Through: "2021-12-31"}, options)
	require.ErrorContains(t, err, "must start 2021-12-16")
	d = BillingDates{Through: "2021-12-31"}
	review, err = b.BillingReview(d)
	require.NoError(t, err)
	require.Contains(t, review, "2021-12-16 through 2021-12-31")
	require.Contains(t, review, "Asset:141.000:Customers:SA-0001  10.00 / 0.00 / 10.00")
	require.NotContains(t, review, "WARNING")
	original := read(t, b.Config.ManualJournal)
	require.NoError(t, os.WriteFile(b.Config.ManualJournal, []byte(original+`
2021-12-05 Journal | Late expense
    Expense:615.000:Utility  $1
    Equity:218.000:Owner  $-1
`), 0600))
	// Only the new day is eligible, even if an old expense has changed.
	preview, err = b.BillingClosePeriod(d, options)
	require.NoError(t, err)
	require.Contains(t, preview, "WARNING: expense postings for applied period")
	require.Contains(t, preview, "Assessed: 3.00")
	options.Apply = tokenFrom(preview)
	_, err = b.BillingClosePeriod(d, options)
	require.NoError(t, err)
	options.Apply = ""
	d = BillingDates{Through: "2022-01-07"}
	preview, err = b.BillingClosePeriod(d, options)
	require.NoError(t, err)
	require.Contains(t, preview, "billing-count: 0")
	options.Apply = tokenFrom(preview)
	_, err = b.BillingClosePeriod(d, options)
	require.NoError(t, err, "a zero-charge interval must still persist a cutoff")
	review, err = b.BillingReview(BillingDates{Through: "2022-01-08"})
	require.NoError(t, err)
	require.Contains(t, review, "Next unbilled day: 2022-01-08")
	require.Contains(t, review, "Asset:141.000:Customers:SA-0001  13.00 / 0.00 / 13.00")
	require.NoError(t, os.WriteFile(b.Config.ManualJournal, []byte(strings.Replace(original, "billing-count: 1", "billing-count: 0", 1)), 0600))
	_, err = b.BillingReview(d)
	require.ErrorContains(t, err, "expects 0 assessments, found 1")
}

func TestBillingCutoffAtMove(t *testing.T) {
	move := "2022-01-15"
	r := CustomerRegistry{
		Connections: []Connection{{ID: "CN-0001"}, {ID: "CN-0002"}},
		ServiceAccounts: []ServiceAccount{
			{ID: "SA-0001", Connection: "CN-0001", ServiceFrom: "2021-10-01", ServiceThrough: &move},
			{ID: "SA-0002", Connection: "CN-0002", ServiceFrom: "2021-10-01"},
			{ID: "SA-0003", Connection: "CN-0001", ServiceFrom: "2022-01-16"},
		},
	}
	opt := BillingOptions{DefaultWeight: 1}
	_, err := billingShares(&r, billingSpan{"2021-10-01", "2022-01-31"}, opt)
	require.ErrorContains(t, err, "--through 2022-01-15")
	for _, tc := range []struct {
		period billingSpan
		first  string
	}{
		{billingSpan{"2021-10-01", move}, "SA-0001"},
		{billingSpan{"2022-01-16", "2022-03-31"}, "SA-0003"},
	} {
		shares, err := billingShares(&r, tc.period, opt)
		require.NoError(t, err)
		require.Len(t, shares, 2)
		require.Equal(t, tc.first, shares[0].service.ID)
		require.Equal(t, "SA-0002", shares[1].service.ID)
	}
}
