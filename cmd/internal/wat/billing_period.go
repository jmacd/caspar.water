package wat

import (
	"encoding/hex"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
)

type BillingDates struct{ Month, From, Through string }

type billingSpan struct{ first, last string }

func (s billingSpan) key() string { return s.first + ".." + s.last }

func billingDay(date string, offset int) string {
	t, _ := time.Parse(time.DateOnly, date) // validated before arithmetic
	return t.AddDate(0, 0, offset).Format(time.DateOnly)
}

func (s billingSpan) valid() bool {
	return validServiceDate(s.first) && validServiceDate(s.last) && s.first <= s.last && s.last < "9999-12-31"
}

func monthSpan(month string) (billingSpan, error) {
	start, err := parseBillingMonth(month)
	if err != nil {
		return billingSpan{}, err
	}
	return billingSpan{start.Format(time.DateOnly), start.AddDate(0, 1, -1).Format(time.DateOnly)}, nil
}

// Coverage is authoritative only in included journals. The standalone
// transaction also represents periods with no assessments or money movement.
func appliedBillingSpans(txs []journalTransaction) ([]billingSpan, error) {
	markers := map[billingSpan]int{}
	charges := map[billingSpan]int{}
	legacy := map[billingSpan]bool{}
	ids := map[string]int{}
	for _, tx := range txs {
		for _, t := range tx.Tags {
			if t[0] == "id" {
				ids[t[1]]++
			}
		}
	}
	for _, tx := range txs {
		tags := map[string]string{}
		for _, t := range tx.Tags {
			switch t[0] {
			case "billing-close", "billing-period", "billing-count", "billing-expenses", "billing-month":
				if _, exists := tags[t[0]]; exists || t[1] == "" {
					return nil, fmt.Errorf("%s: empty or duplicate %s", tx.Date, t[0])
				}
				tags[t[0]] = t[1]
			}
		}
		if len(tags) == 0 {
			continue
		}
		id, err := transactionID(tx)
		if err != nil || ids[id] != 1 {
			return nil, fmt.Errorf("%s: billing entry needs a unique id", tx.Date)
		}
		if month := tags["billing-month"]; month != "" {
			s, err := monthSpan(month)
			if err != nil || len(tags) != 1 || tx.Date != s.last {
				return nil, fmt.Errorf("%s: invalid legacy billing-month", tx.Date)
			}
			legacy[s] = true
			continue
		}
		raw := tags["billing-close"]
		marker := raw != ""
		if !marker {
			raw = tags["billing-period"]
		}
		first, last, _ := strings.Cut(raw, "..")
		s := billingSpan{first, last}
		if !s.valid() || s.last != tx.Date {
			return nil, fmt.Errorf("%s: invalid billing interval %q", tx.Date, raw)
		}
		if marker {
			count, err := strconv.Atoi(tags["billing-count"])
			hash, hashErr := hex.DecodeString(tags["billing-expenses"])
			if _, exists := markers[s]; exists || len(tags) != 3 || hashErr != nil || len(hash) != 32 || err != nil || count < 0 ||
				len(tx.Postings) != 0 || id != "billing-close-"+s.key() {
				return nil, fmt.Errorf("%s: invalid or duplicate billing cutoff marker", raw)
			}
			markers[s] = count
		} else {
			if len(tags) != 1 || len(tx.Postings) != 2 || !strings.HasPrefix(id, "billing-"+s.key()+"-SA-") {
				return nil, fmt.Errorf("%s: invalid assessment in billing interval %s", id, raw)
			}
			charges[s]++
		}
	}
	var spans []billingSpan
	for s := range legacy {
		spans = append(spans, s)
	}
	for s, count := range markers {
		if charges[s] != count {
			return nil, fmt.Errorf("%s: cutoff expects %d assessments, found %d", s.key(), count, charges[s])
		}
		spans = append(spans, s)
	}
	for s := range charges {
		if _, ok := markers[s]; !ok {
			return nil, fmt.Errorf("%s: assessments without billing cutoff marker", s.key())
		}
	}
	slices.SortFunc(spans, func(a, b billingSpan) int { return strings.Compare(a.first, b.first) })
	for i := 1; i < len(spans); i++ {
		if billingDay(spans[i-1].last, 1) != spans[i].first {
			return nil, fmt.Errorf("gap or overlap in applied billing coverage: %s and %s", spans[i-1].key(), spans[i].key())
		}
	}
	return spans, nil
}

// Backdated costs belong to the already assessed period. Never silently
// include them in a new pool; flag the original cutoff for reconciliation.
func priorBillingWarnings(txs []journalTransaction) (string, error) {
	var out strings.Builder
	for _, tx := range txs {
		key := transactionTag(tx, "billing-close")
		if key == "" {
			continue
		}
		first, last, _ := strings.Cut(key, "..")
		m, err := billingTotals(billingSpan{first, last}, txs)
		if err != nil {
			return "", err
		}
		if m.expenseDigest() != transactionTag(tx, "billing-expenses") {
			fmt.Fprintf(&out, "; WARNING: expense postings for applied period %s changed. Reconcile the difference separately; this period will not be rebilled.\n", key)
		}
	}
	return out.String(), nil
}

func (d BillingDates) resolve(applied []billingSpan, closing bool) (billingSpan, error) {
	s := billingSpan{d.From, d.Through}
	next := ""
	if len(applied) != 0 {
		next = billingDay(applied[len(applied)-1].last, 1)
	}
	if d.Month != "" {
		if d.From != "" || d.Through != "" {
			return s, fmt.Errorf("use YYYY-MM or --from/--through, not both")
		}
		var err error
		s, err = monthSpan(d.Month)
		if err != nil {
			return s, err
		}
	} else if s.first == "" {
		if next == "" {
			return s, fmt.Errorf("no billing cutoff is recorded; supply --from and --through for the first period")
		}
		s.first = next
	}
	if !s.valid() {
		return s, fmt.Errorf("use an inclusive YYYY-MM-DD billing range with --through on or after %s", s.first)
	}
	if closing && next != "" && s.first != next {
		return s, fmt.Errorf("applied billings already appear through %s; the next period must start %s", applied[len(applied)-1].last, next)
	}
	return s, nil
}

func (b *Books) applyBilling(snapshot *Draft, s billingSpan, text string) (string, error) {
	payees, err := b.declaredPayees()
	if err != nil {
		return "", err
	}
	if !slices.Contains(payees, "Journal") {
		return "", fmt.Errorf("declare the Journal payee before applying a billing cutoff")
	}
	data := snapshot.proposedText(text)
	backup, _, err := saveSingleFile(snapshot.target, snapshot.files[snapshot.target], data, snapshot.unchanged, func() error {
		return snapshot.withProposedBooks(map[string][]byte{snapshot.target: data}, func(journal string) error {
			if err := b.checkSyntax(journal); err != nil {
				return err
			}
			txs, err := b.transactions(journal)
			if err != nil {
				return err
			}
			spans, err := appliedBillingSpans(txs)
			if err != nil {
				return err
			}
			if len(spans) == 0 || spans[len(spans)-1] != s {
				return fmt.Errorf("missing proposed billing cutoff %s", s.key())
			}
			var selected []journalTransaction
			for _, tx := range txs {
				if transactionTag(tx, "billing-period") == s.key() {
					selected = append(selected, tx)
				}
			}
			return checkServiceTransactions(b.registry, selected)
		})
	})
	return backup, err
}
