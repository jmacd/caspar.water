package wat

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"maps"
	"math"
	"slices"
	"strings"
	"time"
)

func verifyBillingWorkspace(cfg Config) error {
	expected := map[string][]byte{cfg.Journal: generatedMaster(cfg), cfg.LocalAccounts: generatedAccounts(cfg)}
	if cfg.CustomerRegistry {
		expected[cfg.CustomerPayees] = generatedCustomerPayees(cfg)
	}
	if cfg.PayeeRegistry {
		expected[cfg.Payees] = generatedPayees(cfg)
	}
	for path, content := range expected {
		state, err := readNonsymlinkFileState(path, "billing workspace")
		if err != nil {
			return err
		}
		if !bytes.Equal(state.data, content) {
			return fmt.Errorf("%s is stale; open Wat normally to refresh the workspace before reviewing billing", path)
		}
	}
	return nil
}

type billingRollforward struct {
	opening, movement, closing int64
	debits, credits            int
}

type billingWorksheet struct {
	billingSpan
	expenses    map[string]billingRollforward
	balances    map[string]billingRollforward
	expenseRows []string
}

func (m *billingWorksheet) expenseDigest() string {
	slices.Sort(m.expenseRows)
	return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.Join(m.expenseRows, "\n"))))
}

func addBillingCents(a, b int64) (int64, error) {
	if (b > 0 && a > math.MaxInt64-b) || (b < 0 && a < -math.MaxInt64-b) {
		return 0, fmt.Errorf("billing total exceeds supported USD range")
	}
	return a + b, nil
}

func parseBillingMonth(month string) (time.Time, error) {
	t, err := time.Parse("2006-01", month)
	if err != nil || t.Year() == 0 {
		return time.Time{}, fmt.Errorf("expected a calendar month YYYY-MM, got %q", month)
	}
	return t, nil
}

func (b *Books) billingInputs(d BillingDates, closing bool) (billingSpan, []billingSpan, []journalTransaction, error) {
	if err := b.checkInputs(); err != nil {
		return billingSpan{}, nil, nil, err
	}
	txs, err := b.transactions(b.Config.Journal)
	if err != nil {
		return billingSpan{}, nil, nil, err
	}
	applied, err := appliedBillingSpans(txs)
	if err != nil {
		return billingSpan{}, nil, nil, err
	}
	s, err := d.resolve(applied, closing)
	return s, applied, txs, err
}

func billingTotals(s billingSpan, txs []journalTransaction) (*billingWorksheet, error) {
	m := &billingWorksheet{billingSpan: s, expenses: map[string]billingRollforward{}, balances: map[string]billingRollforward{}}
	for _, tx := range txs {
		for _, p := range tx.Postings {
			expense := strings.HasPrefix(p.Account, "Expense:")
			if !expense && !strings.HasPrefix(p.Account, "Liability:") && !strings.HasPrefix(p.Account, "Asset:") {
				continue
			}
			date := tx.Date
			if p.Date != nil {
				date = *p.Date
			}
			if date > m.last || (expense && date < m.first) {
				continue
			}
			if p.Type != "RegularPosting" {
				return nil, fmt.Errorf("%s %s: billing review requires regular postings", date, p.Account)
			}
			for _, a := range p.Amounts {
				if presentJSON(a.Cost) || presentJSON(a.CostBasis) {
					return nil, fmt.Errorf("%s %s: commodity costs are not a USD billing basis", date, p.Account)
				}
			}
			n, err := usdAmountsCents(p.Amounts)
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", date, p.Account, err)
			}
			bucket := m.balances
			if expense {
				bucket = m.expenses
				m.expenseRows = append(m.expenseRows, fmt.Sprintf("%s\t%s\t%d", date, p.Account, n))
			}
			r := bucket[p.Account]
			if date < m.first {
				r.opening, err = addBillingCents(r.opening, n)
			} else {
				r.movement, err = addBillingCents(r.movement, n)
				if n > 0 {
					r.debits++
				} else if n < 0 {
					r.credits++
				}
			}
			bucket[p.Account] = r
			if err != nil {
				return nil, err
			}
		}
	}
	for account, r := range m.balances {
		var err error
		r.closing, err = addBillingCents(r.opening, r.movement)
		if err != nil {
			return nil, err
		}
		m.balances[account] = r
	}
	return m, nil
}

func (m billingWorksheet) review(billable []string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "; Billing worksheet: %s through %s (inclusive, USD; counts are postings)\n", m.first, m.last)
	out.WriteString("; Posted expenses, net of credits:\n")
	accounts := slices.Sorted(maps.Keys(m.expenses))
	for _, account := range billable {
		if _, ok := m.expenses[account]; !ok {
			accounts = append(accounts, account)
		}
	}
	slices.Sort(accounts)
	for _, account := range accounts {
		role := ""
		if billable != nil {
			role = " excluded"
			if slices.Contains(billable, account) {
				role = " billable"
			}
		}
		r := m.expenses[account]
		fmt.Fprintf(&out, ";   %s  %s%s  (charges: %d, credits: %d)\n", account, formatCents(r.movement), role, r.debits, r.credits)
	}
	out.WriteString("; Nonbank assets and liabilities: opening / change / closing (credit balances are negative)\n")
	for _, account := range slices.Sorted(maps.Keys(m.balances)) {
		r := m.balances[account]
		if r.opening != 0 || r.debits+r.credits > 0 {
			fmt.Fprintf(&out, ";   %s  %s / %s / %s  (debits: %d, credits: %d)\n", account, formatCents(r.opening), formatCents(r.movement), formatCents(r.closing), r.debits, r.credits)
			if r.closing > 0 && strings.HasPrefix(account, "Liability:") {
				fmt.Fprintf(&out, ";   Review: %s has a debit balance at cutoff.\n", account)
			}
		}
	}
	out.WriteString("; Review missing accruals and prepayments before allocating. No accruals are inferred.\n")
	return out.String()
}

func (b *Books) BillingReview(d BillingDates) (string, error) {
	s, spans, txs, err := b.billingInputs(d, false)
	if err != nil {
		return "", err
	}
	m, err := billingTotals(s, txs)
	if err != nil {
		return "", err
	}
	status := "; No applied billing cutoff.\n"
	if len(spans) != 0 {
		status = fmt.Sprintf("; Applied coverage: %s through %s. Next unbilled day: %s.\n", spans[0].first, spans[len(spans)-1].last, billingDay(spans[len(spans)-1].last, 1))
	}
	warnings, err := priorBillingWarnings(txs)
	if err != nil {
		return "", err
	}
	return status + warnings + m.review(nil), nil
}
