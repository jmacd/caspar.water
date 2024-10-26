package csv

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmacd/caspar.water/cmd/internal/billing"
	"github.com/jmacd/caspar.water/cmd/internal/billing/constant"
)

type Date struct {
	date time.Time
}

func (d Date) Date() time.Time {
	return d.date
}

func (d *Date) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}
	t, err := time.Parse(constant.CsvLayout, s)
	if err != nil {
		return err
	}
	d.date = t
	return nil
}

func ParseDate(s string) (Date, error) {
	t, err := time.Parse(constant.CsvLayout, s)
	if err != nil {
		return Date{}, err
	}
	d := Date{date: t}
	return d, d.Validate()
}

var dateTooOld = Date{
	date: internal.Must(time.Parse(constant.CsvLayout, "1/1/1900")),
}

func (d Date) Validate() error {
	if d.Date().Before(dateTooOld.Date()) {
		return fmt.Errorf("date is too old: %v", d.Date())
	}
	return nil
}

func DateFromTime(t time.Time) Date {
	return Date{
		date: t,
	}
}

func (d Date) Before(x Date) bool {
	return d.date.Before(x.date)
}
