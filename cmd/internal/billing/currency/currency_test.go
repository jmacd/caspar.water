package currency

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func quoteBytes(s string) []byte {
	return []byte(fmt.Sprintf("%q", s))
}

func TestCurrencyGood(t *testing.T) {
	for ok, val := range map[string]int64{
		"$1.00":     100,
		"$1,000.00": 100000,
		"$1,001.01": 100101,
		"$3333.01":  333301,
	} {
		var a Amount

		err := json.Unmarshal(quoteBytes(ok), &a)
		require.NoError(t, err)
		require.Equal(t, val, a.units)
	}
}

func TestCurrencyBad(t *testing.T) {
	for _, bad := range []string{
		"$$1",
		"$",
		"1",
		"1.00",
		"1,00",
		"1,000",
	} {
		var a Amount

		err := json.Unmarshal(quoteBytes(bad), &a)
		require.Error(t, err)
	}
}
