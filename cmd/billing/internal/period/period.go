package period

import (
	"fmt"
	"time"

	"github.com/jmacd/caspar.water/cmd/billing/internal/csv"
)

// Periods are described by their start in CSV files, which must be
// April or October 1st (any year). They are six months.
type Period struct {
	start time.Time
	end   time.Time
	bill  time.Time
}

func (p *Period) UnmarshalJSON(data []byte) error {
	var d csv.Date
	if err := d.UnmarshalJSON(data); err != nil {
		return err
	}
	date := d.Date()
	if date.Day() != 1 {
		return fmt.Errorf("periods start on the first of the months")
	}
	switch date.Month() {
	case 4, 10:
	default:
		return fmt.Errorf("periods start in April (4) and October (10)")
	}
	p.start = date
	p.bill = date.AddDate(0, 6, 0)
	p.end = date.AddDate(0, 6, -1)
	return nil
}

func (p *Period) Starting() time.Time {
	return p.start
}

func (p *Period) Ending() time.Time {
	return p.end
}

func (p *Period) Billing() time.Time {
	return p.bill
}
