package billing

import (
	"fmt"

	"github.com/jmacd/caspar.water/cmd/billing/internal/account"
	"github.com/jmacd/caspar.water/cmd/billing/internal/constant"
	"github.com/jmacd/caspar.water/cmd/billing/internal/csv"
	"github.com/jmacd/caspar.water/cmd/billing/internal/currency"
	"github.com/jmacd/caspar.water/cmd/billing/internal/expense"
	"github.com/jmacd/caspar.water/cmd/billing/internal/payment"
	"github.com/jmacd/caspar.water/cmd/billing/internal/user"
)

// Billing is the billing state that evolves from one period
// to the next, including cumulative cost-of-living adjustments
// and introductory reweighting.
type Billing struct {
	// effectiveUserCount is maxConnections at the baseline.
	effectiveUserCount int

	// communityCenterCount is 1 at the baseline.
	communityCenterCount int

	// savingsRate is 1 + margin.
	savingsRate float64

	// adjustments counts the number of adjustments.
	adjustments int

	// accounts are a record of user payments.
	accounts *account.Accounts
}

func New() *Billing {
	return &Billing{
		effectiveUserCount:   constant.MaxConnections,
		communityCenterCount: 1,
		savingsRate:          1 + constant.InitialMargin,
		accounts:             account.NewAccounts(),
	}
}

func (b *Billing) EnterPayments(payments []payment.Payment) error {
	for _, pay := range payments {
		acct := b.accounts.Lookup(pay.AccountName)
		if acct == nil {
			return fmt.Errorf("payment account not found: %s", pay.AccountName)
		}
		acct.EnterPayment(pay)
	}
	return nil
}

func (b *Billing) EnterUsers(users []user.User) error {
	for _, user := range users {
		b.accounts.Register(user)
	}
	return nil
}

func (b *Billing) firstAdjustment() {
	b.communityCenterCount = constant.CommunityCenterAdjustedUserCount
	b.effectiveUserCount += constant.CommunityCenterAdjustment
}

func (b *Billing) normalAdjustment() {
	b.adjustments++

	// The margin updates every other period, up until the number
	// of statements required to reach the target margin.
	if b.adjustments < constant.MarginIncreaseYears*constant.StatementsPerYear {
		ratio := float64(b.adjustments) / (constant.MarginIncreaseYears * constant.StatementsPerYear)
		b.savingsRate = 1 + ratio*constant.TargetMargin
	}
}

func (b *Billing) StartCycle(cycle expense.Cycle) {
	switch cycle.Method {
	case "Baseline":
		// No adjustments

	case "InitialAdjustment":
		b.firstAdjustment()

	case "NormalAdjustment":
		// Adjustments happen in the second cycle of the year.
		if cycle.PeriodStart.Starting().Date().Month() == constant.SecondCycleMonth {
			b.normalAdjustment()
		}

	default:
		panic(fmt.Sprintf("Unknown accounting method for %s: %s", cycle.PeriodStart.Starting().Date(), cycle.Method))
	}

}

func (b *Billing) SavingsRate() float64 {
	return b.savingsRate
}

func (b *Billing) EffectiveUserCount() int {
	return b.effectiveUserCount
}

func (b *Billing) CommunityCenterCount() int {
	return b.communityCenterCount
}

func (b *Billing) EnterAmountDue(u user.User, date csv.Date, due currency.Amount) {
	b.accounts.Lookup(u.AccountName).EnterAmountDue(date, due)
}

func (b *Billing) Balance(u user.User, on csv.Date) currency.Amount {
	return b.accounts.Lookup(u.AccountName).Balance(on)
}

func (b *Billing) LastPayment(u user.User) payment.Payment {
	return b.accounts.Lookup(u.AccountName).LastPayment()
}
