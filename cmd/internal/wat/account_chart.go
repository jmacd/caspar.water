package wat

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

var (
	accountNumberPattern        = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)
	postingAccountNumberPattern = regexp.MustCompile(`^([0-9]{3})\.([0-9]{3})$`)
)

type chartAccount struct {
	Category string `yaml:"category"`
	Number   string `yaml:"number"`
	Name     string `yaml:"name"`
	Comment  string `yaml:"comment,omitempty"`
}

type chartAccountDefinition struct {
	Number  string `yaml:"number"`
	Name    string `yaml:"name"`
	Comment string `yaml:"comment,omitempty"`
}

type accountChartDocumentV1 struct {
	Version  int            `yaml:"version"`
	Accounts []chartAccount `yaml:"accounts"`
}

type accountChartDocumentV2 struct {
	Version  int                                 `yaml:"version"`
	Accounts map[string][]chartAccountDefinition `yaml:"accounts"`
}

type chartAccountInput struct {
	path    string
	account chartAccount
}

type accountChart struct {
	byNumber map[string]chartAccount
}

type PostingAccount struct {
	Name           string
	Category       string
	Number         string
	FormalNumber   string
	LocalExtension bool
	FormalCategory string
	FormalName     string
	FormalComment  string
}

func loadAccountChart(path string) (accountChart, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return accountChart{}, fmt.Errorf("read account chart: %w", err)
	}
	return parseAccountChartFile(path, data)
}

func parseAccountChart(data []byte) (accountChart, error) {
	var header struct {
		Version int `yaml:"version"`
	}
	if err := yaml.Unmarshal(data, &header); err != nil {
		return accountChart{}, fmt.Errorf("decode account chart: %w", err)
	}
	var accounts []chartAccountInput
	switch header.Version {
	case 1:
		var document accountChartDocumentV1
		if err := decodeAccountChartDocument(data, &document); err != nil {
			return accountChart{}, err
		}
		accounts = make([]chartAccountInput, len(document.Accounts))
		for index, account := range document.Accounts {
			accounts[index] = chartAccountInput{
				path:    fmt.Sprintf("accounts[%d]", index),
				account: account,
			}
		}
	case 2:
		var document accountChartDocumentV2
		if err := decodeAccountChartDocument(data, &document); err != nil {
			return accountChart{}, err
		}
		categories := make([]string, 0, len(document.Accounts))
		for category := range document.Accounts {
			categories = append(categories, category)
		}
		sort.Strings(categories)
		for _, category := range categories {
			if !baseCategory(category) {
				return accountChart{}, fmt.Errorf(
					"account chart accounts category %q must be Asset, Equity, "+
						"Liability, Expense, or Income",
					category,
				)
			}
			for index, definition := range document.Accounts[category] {
				accounts = append(accounts, chartAccountInput{
					path: fmt.Sprintf("accounts.%s[%d]", category, index),
					account: chartAccount{
						Category: category,
						Number:   definition.Number,
						Name:     definition.Name,
						Comment:  definition.Comment,
					},
				})
			}
		}
	default:
		return accountChart{}, fmt.Errorf(
			"unsupported account chart version %d (expected 1 or 2)",
			header.Version,
		)
	}
	chart := accountChart{byNumber: make(map[string]chartAccount)}
	for _, input := range accounts {
		account := input.account
		if !baseCategory(account.Category) {
			return accountChart{}, fmt.Errorf(
				"account chart %s.category must be Asset, Equity, "+
					"Liability, Expense, or Income",
				input.path,
			)
		}
		if !accountNumberPattern.MatchString(account.Number) {
			return accountChart{}, fmt.Errorf(
				"account chart %s.number is invalid: %q",
				input.path,
				account.Number,
			)
		}
		if strings.TrimSpace(account.Name) != account.Name ||
			account.Name == "" ||
			!singleLine(account.Name) ||
			strings.Contains(account.Name, ":") {
			return accountChart{}, fmt.Errorf(
				"account chart %s.name must be a nonempty "+
					"single-line account component",
				input.path,
			)
		}
		if strings.TrimSpace(account.Comment) != account.Comment ||
			!singleLine(account.Comment) {
			return accountChart{}, fmt.Errorf(
				"account chart %s.comment must be single-line "+
					"without surrounding whitespace",
				input.path,
			)
		}
		if prior, exists := chart.byNumber[account.Number]; exists {
			return accountChart{}, fmt.Errorf(
				"account chart %s duplicates CPUC account %s "+
					"(%s:%s:%s and %s:%s:%s)",
				input.path,
				account.Number,
				prior.Category,
				prior.Number,
				prior.Name,
				account.Category,
				account.Number,
				account.Name,
			)
		}
		chart.byNumber[account.Number] = account
	}
	if len(chart.byNumber) == 0 {
		return accountChart{}, fmt.Errorf("account chart contains no CPUC accounts")
	}
	return chart, nil
}

func decodeAccountChartDocument(data []byte, document any) error {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(document); err != nil {
		return fmt.Errorf("decode account chart: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf(
			"account chart must contain exactly one YAML document",
		)
	}
	return nil
}

func parseAccountChartFile(path string, data []byte) (accountChart, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return parseAccountChart(data)
	case ".journal", ".ledger":
		return parseLegacyAccountChart(data)
	default:
		return accountChart{}, fmt.Errorf(
			"account chart must use .yaml, .yml, .journal, or .ledger: %s",
			path,
		)
	}
}

func parseLegacyAccountChart(data []byte) (accountChart, error) {
	chart := accountChart{byNumber: make(map[string]chartAccount)}
	for number, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "account ") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(line, "account "))
		if declaration, _, ok := strings.Cut(name, ";"); ok {
			name = strings.TrimSpace(declaration)
		}
		category, rest, ok := strings.Cut(name, ":")
		if !ok || !baseCategory(category) {
			continue
		}
		designation, _, _ := strings.Cut(rest, ":")
		accountNumber, formalName, ok := strings.Cut(designation, "-")
		if !ok || !accountNumberPattern.MatchString(accountNumber) || formalName == "" {
			continue
		}
		account := chartAccount{
			Category: category,
			Number:   accountNumber,
			Name:     formalName,
		}
		if prior, exists := chart.byNumber[account.Number]; exists {
			if prior != account {
				return accountChart{}, fmt.Errorf(
					"legacy account chart line %d conflicts with CPUC "+
						"account %s (%s:%s:%s and %s:%s:%s)",
					number+1,
					account.Number,
					prior.Category,
					prior.Number,
					prior.Name,
					account.Category,
					account.Number,
					account.Name,
				)
			}
			continue
		}
		chart.byNumber[account.Number] = account
	}
	if len(chart.byNumber) == 0 {
		return accountChart{}, fmt.Errorf(
			"legacy account chart contains no CPUC account declarations",
		)
	}
	return chart, nil
}

func baseCategory(category string) bool {
	switch category {
	case "Asset", "Equity", "Liability", "Expense", "Income":
		return true
	default:
		return false
	}
}

func parsePostingAccount(name string) (PostingAccount, bool) {
	parts := strings.Split(name, ":")
	if len(parts) < 3 || (!baseCategory(parts[0]) && parts[0] != "Bank") ||
		!postingAccountNumberPattern.MatchString(parts[1]) || !validLocalAccountName(name) {
		return PostingAccount{}, false
	}
	return PostingAccount{Name: name, Category: parts[0], Number: parts[1]}, true
}

func (c accountChart) resolve(account PostingAccount) (PostingAccount, error) {
	match := postingAccountNumberPattern.FindStringSubmatch(account.Number)
	if match == nil {
		return PostingAccount{}, fmt.Errorf(
			"posting account %q must use a three-digit CPUC/local code such as "+
				"104.000 or 131.200",
			account.Name,
		)
	}
	base, local := match[1], match[2]
	branch := local[:1]
	formalNumber := base
	if branch == "0" {
		formalBranches := c.formalBranches(base)
		if len(formalBranches) != 0 {
			return PostingAccount{}, fmt.Errorf(
				"posting account %q uses local code %s, but CPUC account %s "+
					"defines formal subaccounts %s; use a local code in the "+
					"matching formal branch range",
				account.Name,
				account.Number,
				base,
				strings.Join(formalBranches, ", "),
			)
		}
	} else {
		formalNumber += "." + branch
	}
	formal, ok := c.byNumber[formalNumber]
	localExtension := false
	if !ok && branch >= "4" {
		parent, parentExists := c.byNumber[base]
		if parentExists && len(c.formalBranches(base)) != 0 {
			formalNumber = base
			formal = parent
			ok = true
			localExtension = true
		}
	}
	if !ok {
		return PostingAccount{}, fmt.Errorf(
			"posting account %q uses local code %s, which maps to CPUC account %s "+
				"that is not in the CPUC chart",
			account.Name,
			account.Number,
			formalNumber,
		)
	}
	expected := account.Category
	if expected == "Bank" {
		expected = "Asset"
	}
	if formal.Category != expected {
		return PostingAccount{}, fmt.Errorf(
			"posting account %q classifies CPUC account %s as %s, but the CPUC chart classifies it as %s",
			account.Name, formalNumber, account.Category, formal.Category,
		)
	}
	account.FormalNumber = formalNumber
	account.LocalExtension = localExtension
	account.FormalCategory = formal.Category
	account.FormalName = formal.Name
	account.FormalComment = formal.Comment
	return account, nil
}

func (c accountChart) formalBranches(base string) []string {
	prefix := base + "."
	var branches []string
	for number := range c.byNumber {
		if len(number) == len(prefix)+1 &&
			strings.HasPrefix(number, prefix) &&
			number[len(prefix)] >= '1' &&
			number[len(prefix)] <= '9' {
			branches = append(branches, number)
		}
	}
	sort.Strings(branches)
	return branches
}

func (b *Books) PostingAccounts() ([]PostingAccount, error) {
	declared, err := b.declaredAccounts()
	if err != nil {
		return nil, err
	}
	var accounts []PostingAccount
	for _, name := range declared {
		account, ok := parsePostingAccount(name)
		if !ok {
			continue
		}
		account, err = b.chart.resolve(account)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].Name < accounts[j].Name })
	return accounts, nil
}
