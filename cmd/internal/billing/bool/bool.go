package bool

import (
	"encoding/json"
	"strings"
)

type Bool bool

func (b *Bool) UnmarshalJSON(d []byte) error {
	var s string
	if err := json.Unmarshal(d, &s); err != nil {
		return err
	}

	var u bool
	d = []byte(strings.ToLower(s))
	if err := json.Unmarshal(d, &u); err != nil {
		return err
	}
	*b = Bool(u)
	return nil
}
