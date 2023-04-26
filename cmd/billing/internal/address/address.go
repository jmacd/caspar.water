package address

import "strings"

type Address string

// SplitAddress splits a semicolon-delimited multiline address.
func Split(in Address) []string {
	out := strings.Split(string(in), ";")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}
