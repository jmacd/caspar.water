package expense

import (
	"fmt"

	"github.com/jmacd/caspar.water/cmd/billing/internal/currency"
	"github.com/jmacd/caspar.water/cmd/billing/internal/period"
)

// Expenses describes the cost of doing business.
type Expenses struct {
	// PeriodStart is the end of the billing cycle.
	PeriodStart period.Period

	// Operations includes treatment, chemicals, and lab analysis.
	Operations currency.Amount

	// Utilities includes electricity.
	Utilities currency.Amount

	// Insurance is general liability for the water company.
	Insurance currency.Amount

	// Taxes are property taxes, business licensing, and
	// certification costs.
	Taxes currency.Amount

	// Method describes the billing method, values include:
	// - Baseline: the initial condition has no reserve.
	// - FirstAdjustment: a billing cycle where the CommCtr
	//   doubles in weight and the first cost-of-living
	//   adjustment is applied.
	Method string
}

func SplitAnnual(periods []Expenses) error {
	// Every other period we split the taxes and insurance, which
	// are yearly expenses paid during the October-March period.
	for acctNo := 0; acctNo < len(periods); acctNo += 2 {
		yearlyTax := periods[acctNo].Taxes.Split(2)
		yearlyIns := periods[acctNo].Insurance.Split(2)

		periods[acctNo].Taxes = yearlyTax[0]
		periods[acctNo].Insurance = yearlyIns[0]

		if len(periods) > (acctNo+1) &&
			(!periods[acctNo+1].Taxes.IsZero() ||
				!periods[acctNo+1].Insurance.IsZero()) {
			return fmt.Errorf("taxes and insurance October-March not handled")
		}

		// The final period will be missing every other cycle.
		if acctNo+1 < len(periods) {
			periods[acctNo+1].Taxes = yearlyTax[1]
			periods[acctNo+1].Insurance = yearlyIns[1]
		}
	}
	return nil
}
