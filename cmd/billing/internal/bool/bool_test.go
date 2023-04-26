package bool

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func quoteBytes(s string) []byte {
	return []byte(fmt.Sprintf("%q", s))
}

func TestBoolGood(t *testing.T) {
	for ok, val := range map[string]bool{
		"true":  true,
		"false": false,
		"TRUE":  true,
		"FALSE": false,
		"FaLsE": false,
	} {
		var b Bool

		err := json.Unmarshal(quoteBytes(ok), &b)
		require.NoError(t, err)
		require.Equal(t, val, bool(b))
	}
}

func TestBoolBad(t *testing.T) {
	for _, bad := range []string{
		"other",
		"value",
	} {
		var b Bool

		err := json.Unmarshal(quoteBytes(bad), &b)
		require.Error(t, err)
	}
}
