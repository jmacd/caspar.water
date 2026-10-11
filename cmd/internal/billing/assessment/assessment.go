package assessment

import (
	"fmt"
	"math/rand"

	"github.com/jmacd/caspar.water/cmd/internal/billing/currency"
	"github.com/jmacd/caspar.water/cmd/internal/billing/expense"
	"github.com/jmacd/caspar.water/cmd/internal/billing/user"
)

// Assessment is one customer's allocated share of a billing cycle.
type Assessment struct {
	User     user.User
	Amount   currency.Amount
	Fraction float64
	Weight   int
}

// Allocation includes issued customer assessments and any effective
// connection shares that the historical program left unbilled.
type Allocation struct {
	Assessments []Assessment
	Unassigned  currency.Amount
}

// Allocate reproduces the billing program's deterministic cycle allocation.
func Allocate(users []user.User, cycle expense.Cycle) (Allocation, error) {
	if cycle.EffectiveConnections <= 0 {
		return Allocation{}, fmt.Errorf(
			"effective connection count must be positive, got %d",
			cycle.EffectiveConnections,
		)
	}
	sumExpenses := currency.Sum(
		cycle.Operations,
		cycle.Utilities,
		cycle.Taxes,
		cycle.Insurance,
	)
	total := sumExpenses.Scale(1 + cycle.Margin)

	connectionCount := 0
	for _, customer := range users {
		if customer.FirstPeriodStart.Starting().Date().After(cycle.PeriodStart.Starting().Date()) {
			continue
		}
		if cycle.Inactive.Contains(customer) {
			continue
		}
		connectionCount++
		if customer.Commercial && cycle.Method == expense.NormalMethod {
			connectionCount++
		}
	}
	if connectionCount > cycle.EffectiveConnections {
		return Allocation{}, fmt.Errorf(
			"too many active weighted connections: %d > %d",
			connectionCount,
			cycle.EffectiveConnections,
		)
	}

	charges := total.Split(cycle.EffectiveConnections)
	rand.New(rand.NewSource(cycle.PeriodStart.Closing().Date().UnixNano())).Shuffle(
		len(charges),
		func(i, j int) {
			charges[i], charges[j] = charges[j], charges[i]
		},
	)

	var result Allocation
	for _, customer := range users {
		if customer.FirstPeriodStart.Starting().Date().After(cycle.PeriodStart.Starting().Date()) {
			continue
		}

		item := Assessment{User: customer}
		if cycle.Inactive.Contains(customer) {
			result.Assessments = append(result.Assessments, item)
			continue
		}

		item.Weight = 1
		var first currency.Amount
		first, charges = takeCharge(charges)
		item.Amount = first
		if customer.Commercial && cycle.Method == expense.NormalMethod {
			item.Weight = 2
			var second currency.Amount
			second, charges = takeCharge(charges)
			item.Amount = currency.Sum(item.Amount, second)
		}
		item.Fraction = float64(item.Weight) / float64(cycle.EffectiveConnections)
		result.Assessments = append(result.Assessments, item)
	}

	for _, remaining := range charges {
		result.Unassigned = currency.Sum(result.Unassigned, remaining)
	}
	return result, nil
}

func takeCharge(charges []currency.Amount) (currency.Amount, []currency.Amount) {
	if len(charges) == 0 {
		return currency.Amount{}, charges
	}
	return charges[0], charges[1:]
}
