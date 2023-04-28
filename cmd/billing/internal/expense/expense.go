package expense

import (
	"fmt"

	"github.com/jmacd/caspar.water/cmd/billing/internal/csv"
	"github.com/jmacd/caspar.water/cmd/billing/internal/currency"
	"github.com/jmacd/caspar.water/cmd/billing/internal/period"
)

// Cycle describes the cost of doing business for one billing cycle.
type Cycle struct {
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

	// BillDate is the date the statement was prepared.
	BillDate csv.Date

	// Method describes the billing method, values include:
	// - Baseline: the initial condition has no reserve.
	// - FirstAdjustment: a billing cycle where the CommCtr
	//   doubles in weight and the first cost-of-living
	//   adjustment is applied.
	Method string
}

func SplitAnnual(cycles []Cycle) error {
	// Every other period we split the taxes and insurance, which
	// are yearly expenses paid during the October-March period.
	for cycleNo := 0; cycleNo < len(cycles); cycleNo += 2 {
		yearlyTax := cycles[cycleNo].Taxes.Split(2)
		yearlyIns := cycles[cycleNo].Insurance.Split(2)

		cycles[cycleNo].Taxes = yearlyTax[0]
		cycles[cycleNo].Insurance = yearlyIns[0]

		if len(cycles) > (cycleNo+1) &&
			(!cycles[cycleNo+1].Taxes.IsZero() ||
				!cycles[cycleNo+1].Insurance.IsZero()) {
			return fmt.Errorf("taxes and insurance October-March not handled")
		}

		// The final period will be missing every other cycle.
		if cycleNo+1 < len(cycles) {
			cycles[cycleNo+1].Taxes = yearlyTax[1]
			cycles[cycleNo+1].Insurance = yearlyIns[1]
		}
	}
	return nil
}

func (c Cycle) Validate() error {
	if err := c.PeriodStart.Validate(); err != nil {
		return err
	}
	if c.Operations.Units() <= 0 {
		return fmt.Errorf("expenses cannot be negative")
	}
	if c.Utilities.Units() <= 0 {
		return fmt.Errorf("expenses cannot be negative")
	}
	if c.Insurance.Units() < 0 {
		return fmt.Errorf("expenses cannot be negative")
	}
	if c.Taxes.Units() < 0 {
		return fmt.Errorf("expenses cannot be negative")
	}
	if c.Method == "" {
		return fmt.Errorf("expenses method is empty")
	}
	return nil
}
