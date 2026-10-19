package wat

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

var accountNumberPattern = regexp.MustCompile(`^[0-9]+(?:\.[0-9]+)*$`)

type chartAccount struct {
	Category string
	Number   string
	Name     string
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
}

func loadAccountChart(path string) (accountChart, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return accountChart{}, fmt.Errorf("read account chart: %w", err)
	}
	return parseAccountChart(data)
}

func parseAccountChart(data []byte) (accountChart, error) {
	chart := accountChart{byNumber: make(map[string]chartAccount)}
	for number, line := range strings.Split(string(data), "\n") {
		account, ok := parseChartDeclaration(line)
		if !ok {
			continue
		}
		if prior, exists := chart.byNumber[account.Number]; exists {
			if prior != account {
				return accountChart{}, fmt.Errorf(
					"account chart line %d conflicts with CPUC account %s (%s:%s and %s:%s)",
					number+1, account.Number, prior.Category, prior.Name, account.Category, account.Name,
				)
			}
			continue
		}
		chart.byNumber[account.Number] = account
	}
	if len(chart.byNumber) == 0 {
		return accountChart{}, fmt.Errorf("account chart contains no CPUC account declarations")
	}
	return chart, nil
}

func parseChartDeclaration(line string) (chartAccount, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "account ") {
		return chartAccount{}, false
	}
	name := strings.TrimSpace(strings.TrimPrefix(line, "account "))
	if comment, _, ok := strings.Cut(name, ";"); ok {
		name = strings.TrimSpace(comment)
	}
	category, rest, ok := strings.Cut(name, ":")
	if !ok || !baseCategory(category) {
		return chartAccount{}, false
	}
	designation, _, _ := strings.Cut(rest, ":")
	number, formalName, ok := strings.Cut(designation, "-")
	if !ok || !accountNumberPattern.MatchString(number) || formalName == "" {
		return chartAccount{}, false
	}
	return chartAccount{Category: category, Number: number, Name: formalName}, true
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
