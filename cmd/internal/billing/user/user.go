package user

import (
	"fmt"

	"github.com/jmacd/caspar.water/cmd/internal/billing/address"
	"github.com/jmacd/caspar.water/cmd/internal/billing/bool"
	"github.com/jmacd/caspar.water/cmd/internal/billing/period"
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

	// FirstPeriodStart is the initial billing cycle.
	FirstPeriodStart period.Period

	// Commercial indicates double weight.
	Commercial bool.Bool
}

func (u User) Validate() error {
	if u.AccountName == "" {
		return fmt.Errorf("empty user account name")
	}
	if u.UserName == "" {
		return fmt.Errorf("empty user name")
	}
	if u.ServiceAddress == "" {
		return fmt.Errorf("empty service address")
	}
	if u.BillingAddress == "" {
		return fmt.Errorf("empty service address")
	}
	if err := u.FirstPeriodStart.Validate(); err != nil {
		return err
	}
	return nil
}
