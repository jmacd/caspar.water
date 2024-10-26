package user

import (
	"bytes"
	"testing"

	"github.com/jmacd/caspar.water/cmd/internal/billing"
	"github.com/jmacd/caspar.water/cmd/internal/billing/csv"
	"github.com/jmacd/caspar.water/cmd/internal/billing/period"
	"github.com/stretchr/testify/require"
)

func TestUserRead(t *testing.T) {
	data := `Account Name,User Name,Service Address,Billing Address,Active,First Period Start
TestAcct1,Mister and Misses,1 Driveway,1 P.O. Box,TRUE,4/1/2022
TestAcct2,Misses and Mister,2 Driveway,2 P.O. Box,FALSE,10/1/2022
`
	users, err := csv.Read[User]("<input>", bytes.NewBufferString(data))
	require.NoError(t, err)
	require.Equal(t, []User{
		{
			AccountName:      "TestAcct1",
			UserName:         "Mister and Misses",
			ServiceAddress:   "1 Driveway",
			BillingAddress:   "1 P.O. Box",
			FirstPeriodStart: internal.Must(period.ParseStart("4/1/2022")),
		},
		{
			AccountName:      "TestAcct2",
			UserName:         "Misses and Mister",
			ServiceAddress:   "2 Driveway",
			BillingAddress:   "2 P.O. Box",
			FirstPeriodStart: internal.Must(period.ParseStart("10/1/2022")),
		},
	}, users)
}
