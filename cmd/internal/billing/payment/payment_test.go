package payment

import (
	"bytes"
	"testing"

	"github.com/jmacd/caspar.water/cmd/internal/billing"
	"github.com/jmacd/caspar.water/cmd/internal/billing/csv"
	"github.com/jmacd/caspar.water/cmd/internal/billing/currency"
	"github.com/stretchr/testify/require"
)

const header = "Date,Account Name,Amount"

func TestPaymentRead(t *testing.T) {
	data := header + `
3/3/2023,Name1,"$1,000.01"
3/4/2023,Name2,"$211.11"
`
	payments, err := csv.Read[Payment]("<input>", bytes.NewBufferString(data))
	require.NoError(t, err)
	require.Equal(t, []Payment{
		{
			Date:        internal.Must(csv.ParseDate("3/3/2023")),
			AccountName: "Name1",
			Amount:      currency.Units(100001),
		},
		{
			Date:        internal.Must(csv.ParseDate("3/4/2023")),
			AccountName: "Name2",
			Amount:      currency.Units(21111),
		},
	}, payments)
}

func TestPaymentInvalid(t *testing.T) {
	for _, test := range []string{
		`3/3/2023,,"$1,000.01"`,
		`2023,Name,"$1,000.01"`,
		`3/3/2023,Name,"$1"`,
	} {
		data := header + "\n" + test
		_, err := csv.Read[Payment]("<input>", bytes.NewBufferString(data))
		require.Error(t, err, "for %s", test)
	}
}
