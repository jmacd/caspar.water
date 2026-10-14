package wat

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type Posting struct {
	Account string
	Amount  string
}

type Entry struct {
	Date        string
	Payee       string
	Description string
	Reference   string
	Source      string
	Postings    []Posting
}

func ParseCents(text string) (int64, error) {
	original := text
	text = strings.TrimSpace(text)
	sign := int64(1)
	if strings.HasPrefix(text, "-") {
		sign, text = -1, text[1:]
	} else {
		text = strings.TrimPrefix(text, "+")
	}
	parts := strings.Split(text, ".")
	if len(parts) > 2 || parts[0] == "" {
		return 0, fmt.Errorf("invalid USD amount %q; use digits and at most two decimal places", original)
	}
	fraction := "00"
	if len(parts) == 2 {
		if len(parts[1]) < 1 || len(parts[1]) > 2 {
			return 0, fmt.Errorf("invalid USD precision in %q", original)
		}
		fraction = parts[1] + strings.Repeat("0", 2-len(parts[1]))
	}
	for _, c := range parts[0] + fraction {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid USD amount %q; omit currency symbols and grouping", original)
		}
	}
	magnitude, err := strconv.ParseInt(parts[0]+fraction, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("USD amount out of range: %q", original)
	}
	return sign * magnitude, nil
}

func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign, cents = "-", -cents
	}
	return fmt.Sprintf("%s%d.%02d", sign, cents/100, cents%100)
}

func singleLine(text string) bool {
	return !strings.ContainsFunc(text, unicode.IsControl)
}

func (e Entry) render(year int, accounts []string, id string) (string, error) {
	date, err := time.Parse("2006-01-02", e.Date)
	if err != nil || date.Year() != year {
		return "", fmt.Errorf("date must be YYYY-MM-DD within %d", year)
	}
	for _, value := range []string{e.Payee, e.Description, e.Reference, e.Source} {
		if !singleLine(value) {
			return "", fmt.Errorf("transaction fields must not contain control characters")
		}
	}
	if strings.TrimSpace(e.Payee) == "" || strings.ContainsAny(e.Payee+e.Description, ";|") {
		return "", fmt.Errorf("payee is required; payee and description cannot contain ';' or '|'")
	}
	if strings.Contains(e.Reference+e.Source, ",") {
		return "", fmt.Errorf("reference and source cannot contain commas")
	}
	if len(e.Postings) < 2 {
		return "", fmt.Errorf("at least two postings are required")
	}
	normalized := e
	normalized.Postings = append([]Posting(nil), e.Postings...)
	var total int64
	for i, posting := range e.Postings {
		found := false
		for _, account := range accounts {
			if posting.Account == account {
				found = true
				break
			}
		}
		if !found || !singleLine(posting.Account) || strings.Contains(posting.Account, "  ") ||
			strings.ContainsAny(posting.Account, ";[]()") {
			return "", fmt.Errorf("select a declared regular posting account: %q", posting.Account)
		}
		cents, err := ParseCents(posting.Amount)
		if err != nil {
			return "", err
		}
		if cents == 0 {
			return "", fmt.Errorf("posting to %s must be nonzero", posting.Account)
		}
		if (cents > 0 && total > math.MaxInt64-cents) || (cents < 0 && total < math.MinInt64-cents) {
			return "", fmt.Errorf("posting total exceeds supported USD range")
		}
		total += cents
		normalized.Postings[i].Amount = formatCents(cents)
	}
	if total != 0 {
		return "", fmt.Errorf("postings do not balance (signed sum: %d cents)", total)
	}
	return normalized.journalText(id), nil
}

func (e Entry) journalText(id string) string {
	var text strings.Builder
	date := e.Date
	if date == "" {
		date = "YYYY-MM-DD"
	}
	fmt.Fprintf(&text, "%s %s", date, strings.TrimSpace(e.Payee))
	if strings.TrimSpace(e.Description) != "" {
		fmt.Fprintf(&text, " | %s", strings.TrimSpace(e.Description))
	}
	fmt.Fprintf(&text, "\n    ; id: %s\n", id)
	for _, tag := range []struct{ name, value string }{
		{"reference", e.Reference}, {"source", e.Source},
	} {
		if tag.value != "" {
			fmt.Fprintf(&text, "    ; %s: %s\n", tag.name, tag.value)
		}
	}
	for _, posting := range e.Postings {
		fmt.Fprintf(&text, "    %s", posting.Account)
		if strings.TrimSpace(posting.Amount) != "" {
			fmt.Fprintf(&text, "  $%s", strings.TrimSpace(posting.Amount))
		}
		text.WriteByte('\n')
	}
	return text.String()
}

func newID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("generate transaction ID: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
