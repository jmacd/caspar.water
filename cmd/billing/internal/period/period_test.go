package period

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jmacd/caspar.water/cmd/billing/internal/constant"
	"github.com/stretchr/testify/require"
)

func quoteBytes(s string) []byte {
	return []byte(fmt.Sprintf("%q", s))
}

func TestPeriodJSONGood(t *testing.T) {
	for _, good := range [][]string{
		{"10/1/2021", "3/31/2022"},
		{"4/1/2022", "9/30/2022"},
		{"10/1/2022", "3/31/2023"},
	} {
		var p Period

		err := json.Unmarshal(quoteBytes(good[0]), &p)
		require.NoError(t, err)

		require.Equal(t, good[0], p.Starting().Date().Format(constant.CsvLayout))
		require.Equal(t, good[1], p.Closing().Date().Format(constant.CsvLayout))
	}
}

func TestPeriodJSONBad(t *testing.T) {
	for _, bad := range []string{
		"9/30/2021",
		"7/1/2022",
		"11/1/2022",
	} {
		var p Period
		err := json.Unmarshal(quoteBytes(bad), &p)
		// These parse correctly, but do not validate.
		require.NoError(t, err)

		require.Error(t, p.Validate())
	}
}
