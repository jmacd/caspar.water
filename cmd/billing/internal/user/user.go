package user

import (
	"github.com/jmacd/caspar.water/cmd/billing/internal/address"
	"github.com/jmacd/caspar.water/cmd/billing/internal/bool"
	"github.com/jmacd/caspar.water/cmd/billing/internal/period"
)

// User describes one account for payment.
type User struct {
	// AccountName is an internal identifier, descriptive
	// for the company but does not meaningful on the
	// bill.
	AccountName string

	// UserName is the responsible party's name.
	UserName string

	// ServiceAddress is the location of water service.
	ServiceAddress address.Address

	// BillingAddress is where the user receives mail.
	BillingAddress address.Address

	// Active indicates a viable connection.
	Active bool.Bool

	// FirstPeriodStart is the initial billing cycle.
	FirstPeriodStart period.Period
}
