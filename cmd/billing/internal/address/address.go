package address

import "strings"

type Address string

// Split splits a semicolon-delimited multiline address.
func (addr Address) Split() []string {
	out := strings.Split(string(addr), ";")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}
