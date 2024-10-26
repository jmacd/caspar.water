package period

import (
	"fmt"

	"github.com/jmacd/caspar.water/cmd/internal/billing/csv"
)

// Periods are described by their start in CSV files, which must be
// April or October 1st (any year). They are six months.
type Period struct {
	start csv.Date
	close csv.Date
}

func (p *Period) UnmarshalJSON(data []byte) error {
	var d csv.Date
	if err := d.UnmarshalJSON(data); err != nil {
		return err
	}
	p.start = d
	p.close = csv.DateFromTime(d.Date().AddDate(0, 6, -1))
	return nil
}

func (p *Period) Starting() csv.Date {
	return p.start
}

func (p *Period) Closing() csv.Date {
	return p.close
}

func ParseStart(s string) (Period, error) {
	var p Period
	if err := p.UnmarshalJSON([]byte(fmt.Sprintf("%q", s))); err != nil {
		return p, err
	}
	return p, p.Validate()
}

func (p Period) Validate() error {
	if p.start.Date().Day() != 1 {
		return fmt.Errorf("periods start on the first of the months")
	}
	switch p.start.Date().Month() {
	case 4, 10:
	default:
		return fmt.Errorf("periods start in April (4) and October (10)")
	}
	return nil
}
