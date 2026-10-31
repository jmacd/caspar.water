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

var accountNumberPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)

type chartAccount struct {
	Category string `yaml:"category"`
	Number   string `yaml:"number"`
	Name     string `yaml:"name"`
	Comment  string `yaml:"comment,omitempty"`
}

type accountChartDocument struct {
	Version  int            `yaml:"version"`
	Accounts []chartAccount `yaml:"accounts"`
}

type accountChart struct {
	byNumber map[string]chartAccount
}

type PostingAccount struct {
	Name           string
	Category       string
	Number         string
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
	var document accountChartDocument
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&document); err != nil {
		return accountChart{}, fmt.Errorf("decode account chart: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return accountChart{}, fmt.Errorf(
			"account chart must contain exactly one YAML document",
		)
	}
	if document.Version != 1 {
		return accountChart{}, fmt.Errorf(
			"unsupported account chart version %d (expected 1)",
			document.Version,
		)
	}
	chart := accountChart{byNumber: make(map[string]chartAccount)}
	for index, account := range document.Accounts {
		if !baseCategory(account.Category) {
			return accountChart{}, fmt.Errorf(
				"account chart accounts[%d].category must be Asset, Equity, "+
					"Liability, Expense, or Income",
				index,
			)
		}
		if !accountNumberPattern.MatchString(account.Number) {
			return accountChart{}, fmt.Errorf(
				"account chart accounts[%d].number is invalid: %q",
				index,
				account.Number,
			)
		}
		if strings.TrimSpace(account.Name) != account.Name ||
			account.Name == "" ||
			!singleLine(account.Name) ||
			strings.Contains(account.Name, ":") {
			return accountChart{}, fmt.Errorf(
				"account chart accounts[%d].name must be a nonempty "+
					"single-line account component",
				index,
			)
		}
		if strings.TrimSpace(account.Comment) != account.Comment ||
			!singleLine(account.Comment) {
			return accountChart{}, fmt.Errorf(
				"account chart accounts[%d].comment must be single-line "+
					"without surrounding whitespace",
				index,
			)
		}
		if prior, exists := chart.byNumber[account.Number]; exists {
			return accountChart{}, fmt.Errorf(
				"account chart accounts[%d] duplicates CPUC account %s "+
					"(%s:%s:%s and %s:%s:%s)",
				index,
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
		!accountNumberPattern.MatchString(parts[1]) || !validLocalAccountName(name) {
		return PostingAccount{}, false
	}
	return PostingAccount{Name: name, Category: parts[0], Number: parts[1]}, true
}

func (c accountChart) resolve(account PostingAccount) (PostingAccount, error) {
	formal, ok := c.byNumber[account.Number]
	if !ok {
		return PostingAccount{}, fmt.Errorf("posting account %q uses CPUC account %s, which is not in the CPUC chart", account.Name, account.Number)
	}
	expected := account.Category
	if expected == "Bank" {
		expected = "Asset"
	}
	if formal.Category != expected {
		return PostingAccount{}, fmt.Errorf(
			"posting account %q classifies CPUC account %s as %s, but the CPUC chart classifies it as %s",
			account.Name, account.Number, account.Category, formal.Category,
		)
	}
	account.FormalCategory = formal.Category
	account.FormalName = formal.Name
	account.FormalComment = formal.Comment
	return account, nil
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
