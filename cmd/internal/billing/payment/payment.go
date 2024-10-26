package payment

import (
	"fmt"

	"github.com/jmacd/caspar.water/cmd/internal/billing/csv"
	"github.com/jmacd/caspar.water/cmd/internal/billing/currency"
)

// Payment records a single user's payment.
type Payment struct {
	Date        csv.Date
	AccountName string
	Amount      currency.Amount

	// This adjustment is a hack; need a new data model to handle account
	// changeover and would help for multi-account payers.
	Comments string
}

func (p Payment) Validate() error {
	if err := p.Date.Validate(); err != nil {
		return err
	}
	if p.AccountName == "" {
		return fmt.Errorf("empty payment account name")
	}
	if p.Amount.Units() < 0 {
		return fmt.Errorf("negative or zero payment is invalid")
	}
	return nil
}
