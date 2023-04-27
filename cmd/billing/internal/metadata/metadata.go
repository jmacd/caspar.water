package metadata

import (
	"fmt"

	"github.com/jmacd/caspar.water/cmd/billing/internal/address"
)

// Metadata describes the billing entity and other static
// information.
type Metadata struct {
	// Name is how to make the payment.
	Name string

	// Address is where to send the payment.
	Address address.Address

	// Contact is how and with whom to discuss the payment.
	Contact string
}

func (m Metadata) Validate() error {
	if m.Name == "" {
		return fmt.Errorf("business name empty")
	}
	if m.Address == "" {
		return fmt.Errorf("business address empty")
	}
	if m.Contact == "" {
		return fmt.Errorf("business contact empty")
	}
	return nil
}
