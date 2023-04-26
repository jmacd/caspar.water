package payment

import (
	"github.com/jmacd/caspar.water/cmd/billing/internal/csv"
	"github.com/jmacd/caspar.water/cmd/billing/internal/currency"
)

// Payment records a single user's payment.
type Payment struct {
	Date        csv.Date
	AccountName string
	Amount      currency.Amount
}
