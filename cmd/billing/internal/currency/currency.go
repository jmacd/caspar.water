package currency

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Rhymond/go-money"
)

var dollarsAndCentsRe = regexp.MustCompile(`\$(\d+(?:,\d\d\d)*)\.(\d\d)`)

func must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

type Amount struct {
	units int64
}

func (a Amount) money() *money.Money {
	return money.New(a.units, money.USD)
}

func (a Amount) Split(n int) []Amount {
	var r []Amount
	for _, in := range must(a.money().Split(n)) {
		r = append(r, Amount{
			units: in.Amount(),
		})
	}
	return r
}

func (a Amount) IsZero() bool {
	return a == Amount{}
}

func (a Amount) Scale(f float64) Amount {
	return Amount{
		units: int64(f * float64(a.units)),
	}
}

func (a Amount) Display() string {
	return a.money().Display()
}

func Sum(inputs ...Amount) Amount {
	total := money.New(0, money.USD)
	for _, in := range inputs {
		total = must(total.Add(in.money()))
	}
	return Amount{
		units: total.Amount(),
	}
}

func (a *Amount) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	parts := dollarsAndCentsRe.FindStringSubmatch(s)
	if parts == nil {
		return fmt.Errorf("not a currency amount: %v", s)
	}

	dollars, err := strconv.Atoi(strings.ReplaceAll(parts[1], ",", ""))
	if err != nil {
		return err
	}
	cents, err := strconv.Atoi(parts[2])
	if err != nil {
		return err
	}
	a.units = int64(dollars*100 + cents)
	return nil
}
