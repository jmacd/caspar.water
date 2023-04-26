package csv

import (
	"encoding/json"
	"time"

	"github.com/jmacd/caspar.water/cmd/billing/internal/constant"
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
