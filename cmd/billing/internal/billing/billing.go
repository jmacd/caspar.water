package billing

import (
	"github.com/jmacd/caspar.water/cmd/billing/internal/account"
	"github.com/jmacd/caspar.water/cmd/billing/internal/constant"
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
