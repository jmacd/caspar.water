package wat

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"fmt"
	"io"
	"maps"
	"math"
	"math/big"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type billingPolicy struct {
	Version          int               `yaml:"version"`
	BillFrom         string            `yaml:"bill_from"`
	Expenses         []string          `yaml:"billable_expenses"`
	Revenue          string            `yaml:"revenue_account"`
	ConnectionIncome map[string]string `yaml:"revenue_by_connection,omitempty"`
}

type BillingOptions struct {
	Policy, MarginPercent, Output, Apply string
	Weights                              []string
	DefaultWeight, UnbilledWeight        int
}

func (b *Books) billingPolicy(data []byte) (billingPolicy, error) {
	var p billingPolicy
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&p); err != nil {
		return p, fmt.Errorf("billing policy: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("billing policy must contain exactly one YAML document")
	}
	if p.Version != 1 || len(p.Expenses) == 0 {
		return p, fmt.Errorf("billing policy requires version 1 and nonempty billable_expenses")
	}
	if _, err := parseBillingMonth(p.BillFrom); err != nil {
		return p, fmt.Errorf("bill_from: %w", err)
	}
	for i, account := range p.Expenses {
		if !strings.HasPrefix(account, "Expense:") || !slices.Contains(b.Config.Accounts, account) || slices.Contains(p.Expenses[:i], account) {
			return p, fmt.Errorf("billable_expenses requires distinct exact configured Expense accounts: %q", account)
		}
	}
	validateRevenue := func(name string) error {
		a, ok := parsePostingAccount(name)
		if !ok || a.Category != "Income" {
			return fmt.Errorf("billing revenue must be a CPUC Income posting account: %q", name)
		}
		_, err := b.chart.resolve(a)
		return err
	}
	if err := validateRevenue(p.Revenue); err != nil {
		return p, err
	}
	for id, account := range p.ConnectionIncome {
		c, err := b.registry.Connection(id)
		if err != nil || c.ID != id {
			return p, fmt.Errorf("unknown connection ID in revenue_by_connection: %q", id)
		}
		if err := validateRevenue(account); err != nil {
			return p, err
		}
	}
	return p, nil
}

type billShare struct {
	key     string
	service ServiceAccount // empty for an unbilled share
	weight  int64
	cents   int64
}

func billingShares(registry *CustomerRegistry, period billingSpan, options BillingOptions) ([]billShare, error) {
	if options.DefaultWeight < -1 || options.DefaultWeight > math.MaxInt32 || options.UnbilledWeight < 0 || options.UnbilledWeight > math.MaxInt32 {
		return nil, fmt.Errorf("weights must be nonnegative integers up to %d", math.MaxInt32)
	}
	overrides := map[string]int64{}
	for _, spec := range options.Weights {
		id, raw, ok := strings.Cut(spec, "=")
		w, err := strconv.ParseUint(raw, 10, 31)
		if !ok || err != nil {
			return nil, fmt.Errorf("expected --weight CN-ID=nonnegative-integer, got %q", spec)
		}
		c, err := registry.Connection(id)
		if err != nil || c.ID != id {
			return nil, fmt.Errorf("unknown connection ID in weight %q", spec)
		}
		if _, ok := overrides[id]; ok {
			return nil, fmt.Errorf("duplicate weight for %s", id)
		}
		overrides[id] = int64(w)
	}
	var result []billShare
	for _, c := range registry.Connections {
		w, explicit := overrides[c.ID]
		if !explicit {
			w = int64(options.DefaultWeight)
		}
		var active ServiceAccount
		for _, s := range registry.ServiceAccounts {
			if s.Connection != c.ID || s.ServiceFrom > period.last || (s.ServiceThrough != nil && *s.ServiceThrough < period.first) {
				continue
			}
			if w < 0 {
				return nil, fmt.Errorf("missing weight for %s: set --weight or --default-weight", c.ID)
			}
			if w != 0 && (s.ServiceFrom > period.first || (s.ServiceThrough != nil && *s.ServiceThrough < period.last)) {
				date := billingDay(s.ServiceFrom, -1)
				if s.ServiceFrom <= period.first {
					date = *s.ServiceThrough
				}
				return nil, fmt.Errorf("%s changes service within this interval; split at the move (try --through %s), rather than billing the wrong occupant", c.ID, date)
			}
			active = s
		}
		if explicit || active.ID != "" {
			result = append(result, billShare{key: c.ID, service: active, weight: w})
		}
	}
	if options.UnbilledWeight > 0 {
		result = append(result, billShare{key: "unbilled", weight: int64(options.UnbilledWeight)})
	}
	slices.SortFunc(result, func(a, b billShare) int { return strings.Compare(a.key, b.key) })
	return result, nil
}

func billingMarkup(cost, basisPoints int64) (int64, error) {
	// Round once on the monthly pool, half away from zero. Never use floats for money.
	product := new(big.Int).Mul(big.NewInt(cost), big.NewInt(basisPoints))
	product.Abs(product).Add(product, big.NewInt(5000)).Quo(product, big.NewInt(10000))
	if !product.IsInt64() {
		return 0, fmt.Errorf("billing markup exceeds supported USD range")
	}
	markup := product.Int64()
	if cost < 0 {
		markup = -markup
	}
	return markup, nil
}

func distributeBilling(total int64, shares []billShare) error {
	var weights int64
	for _, s := range shares {
		var err error
		if weights, err = addBillingCents(weights, s.weight); err != nil {
			return fmt.Errorf("total billing weights out of range")
		}
	}
	if weights == 0 {
		return nil // No eligible service: the whole pool remains unassigned.
	}
	magnitude := new(big.Int).Abs(big.NewInt(total))
	type remainder struct {
		index int
		value int64
	}
	var remainders []remainder
	left := magnitude.Int64()
	for i, s := range shares {
		product := new(big.Int).Mul(magnitude, big.NewInt(s.weight))
		rem := new(big.Int)
		product.QuoRem(product, big.NewInt(weights), rem)
		shares[i].cents = product.Int64()
		left -= shares[i].cents
		if rem.Sign() > 0 {
			remainders = append(remainders, remainder{i, rem.Int64()})
		}
	}
	// Shares arrive in ID order, so equal remainders have a stable tiebreaker.
	slices.SortStableFunc(remainders, func(a, b remainder) int {
		return cmp.Compare(b.value, a.value)
	})
	for _, r := range remainders[:int(left)] {
		shares[r.index].cents++
	}
	if total < 0 {
		for i := range shares {
			shares[i].cents = -shares[i].cents
		}
	}
	return nil
}

func policyCost(m *billingWorksheet, policy billingPolicy) (int64, error) {
	var cost int64
	for _, a := range policy.Expenses {
		var err error
		if cost, err = addBillingCents(cost, m.expenses[a].movement); err != nil {
			return 0, err
		}
	}
	return cost, nil
}

func (b *Books) billingJournal(s billingSpan, txs []journalTransaction, policy billingPolicy, opt BillingOptions, digest [32]byte) (string, error) {
	bps, err := ParseCents(opt.MarginPercent) // percent with two decimal places = basis points
	if err != nil || bps < 0 {
		return "", fmt.Errorf("margin-percent must be a nonnegative percentage with at most two decimal places")
	}
	m, err := billingTotals(s, txs)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("; Water-service assessments. Not an issued invoice or a bank receipt.\n")
	fmt.Fprintf(&out, "; Policy SHA256: %x; bill from: %s\n", digest, policy.BillFrom)
	warnings, err := priorBillingWarnings(txs)
	if err != nil {
		return "", err
	}
	out.WriteString(warnings)
	out.WriteString(m.review(policy.Expenses))
	billFrom := policy.BillFrom + "-01"
	basis := m.expenseDigest()
	marker := func(count int) string {
		return fmt.Sprintf("\n%s Journal | Billing cutoff %s\n    ; id: billing-close-%s\n    ; billing-close: %s\n    ; billing-count: %d\n    ; billing-expenses: %s\n", s.last, s.key(), s.key(), s.key(), count, basis)
	}
	if s.first < billFrom {
		early, err := billingTotals(billingSpan{s.first, min(s.last, billingDay(billFrom, -1))}, txs)
		if err != nil {
			return "", err
		}
		cost, err := policyCost(early, policy)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&out, "; Before billing began (%s). Unassigned cost: %s; no markup or carry-forward.\n", early.key(), formatCents(cost))
		if s.last < billFrom {
			return out.String() + marker(0), nil
		}
		m, err = billingTotals(billingSpan{billFrom, s.last}, txs)
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&out, "; Allocation covers %s only.\n", m.key())
	}
	cost, err := policyCost(m, policy)
	if err != nil {
		return "", err
	}
	shares, err := billingShares(b.registry, m.billingSpan, opt)
	if err != nil {
		return "", err
	}
	markup, err := billingMarkup(cost, bps)
	if err != nil {
		return "", err
	}
	total, err := addBillingCents(cost, markup)
	if err != nil {
		return "", err
	}
	if err := distributeBilling(total, shares); err != nil {
		return "", err
	}
	fmt.Fprintf(&out, "; Billable cost: %s; markup at %s%%: %s; pool: %s\n",
		formatCents(cost), formatCents(bps), formatCents(markup), formatCents(total))
	accounts, err := b.Accounts()
	if err != nil {
		return "", err
	}
	var entries strings.Builder
	decls := map[string]bool{}
	var billed int64
	count := 0
	coverageKey := s.key()
	for _, share := range shares {
		s := share.service
		label := s.ID
		if label == "" {
			label = "unassigned"
		}
		fmt.Fprintf(&out, ";   %s %s weight=%d  %s\n", share.key, label, share.weight, formatCents(share.cents))
		if s.ID == "" || share.cents == 0 {
			continue
		}
		income := policy.Revenue
		if override, ok := policy.ConnectionIncome[s.Connection]; ok {
			income = override
		}
		if !slices.Contains(accounts, income) {
			decls[income] = true
			accounts = append(accounts, income)
		}
		customer, err := b.registry.Customer(s.Customer)
		if err != nil {
			return "", err
		}
		entry := Entry{Date: m.last, Payee: customer.Payee, Description: "Water service assessment " + m.key() + " " + s.ID,
			comments: []string{"billing-period: " + coverageKey, "service-through: " + m.last},
			Postings: []Posting{{s.ReceivableAccount, formatCents(share.cents)}, {income, formatCents(-share.cents)}}}
		text, err := entry.render(accounts, "billing-"+coverageKey+"-"+s.ID)
		if err != nil {
			return "", err
		}
		entries.WriteString("\n" + text)
		billed += share.cents // same sign; bounded by the validated pool
		count++
	}
	fmt.Fprintf(&out, "; Assessed: %s; unassigned: %s\n", formatCents(billed), formatCents(total-billed))
	for _, income := range slices.Sorted(maps.Keys(decls)) {
		fmt.Fprintf(&out, "\naccount %s\n", income)
	}
	out.WriteString(entries.String())
	out.WriteString(marker(count))
	return out.String(), nil
}

func (b *Books) BillingClose(month string, opt BillingOptions) (string, error) {
	return b.BillingClosePeriod(BillingDates{Month: month}, opt)
}

func (b *Books) BillingClosePeriod(d BillingDates, opt BillingOptions) (string, error) {
	if b.registry == nil {
		return "", fmt.Errorf("billing close requires the customer registry")
	}
	if opt.Apply != "" && opt.Output != "" {
		return "", fmt.Errorf("choose either --apply or --output")
	}
	policyPath, err := filepath.Abs(opt.Policy)
	if err != nil {
		return "", err
	}
	state, err := readNonsymlinkFileState(policyPath, "billing policy")
	if err != nil {
		return "", err
	}
	policy, err := b.billingPolicy(state.data)
	if err != nil {
		return "", err
	}
	files, err := b.snapshot()
	if err != nil {
		return "", err
	}
	snapshot := Draft{books: b, files: files, inputs: b.inputStates(), target: b.Config.ManualJournal}
	snapshot.inputs[policyPath] = state
	s, _, txs, err := b.billingInputs(d, true)
	if err != nil {
		return "", err
	}
	text, err := b.billingJournal(s, txs, policy, opt, sha256.Sum256(state.data))
	if err != nil {
		return "", err
	}
	if err := snapshot.unchanged(); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(text))
	token := fmt.Sprintf("%x", sum[:16])
	text = "; Review token: " + token + "\n" + text
	if opt.Apply != "" {
		if opt.Apply != token {
			return "", fmt.Errorf("review token does not match; preview the current charges and balances before applying")
		}
		backup, err := b.applyBilling(&snapshot, s, text)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Applied billing %s. Next unbilled day: %s. Recovery copy: %s\n", s.key(), billingDay(s.last, 1), backup), nil
	}
	if opt.Output != "" {
		path, err := filepath.Abs(opt.Output)
		if err != nil {
			return "", err
		}
		dir, err := filepath.EvalSymlinks(filepath.Dir(path))
		if err != nil {
			return "", err
		}
		path = filepath.Join(dir, filepath.Base(path))
		if within(b.Config.root, path) || filepath.Ext(path) != ".journal" {
			return "", fmt.Errorf("use a new .journal file outside the live workspace so includes cannot book a hypothetical charge")
		}
		if err := createExclusiveFile(path, []byte(text)); err != nil {
			return "", err
		}
	}
	return text, nil
}
