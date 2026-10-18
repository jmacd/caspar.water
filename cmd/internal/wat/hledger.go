package wat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Books struct {
	Config Config
	binary string
	chart  accountChart
}

type journalTransaction struct {
	Code    string  `json:"tcode"`
	Status  string  `json:"tstatus"`
	Comment string  `json:"tcomment"`
	Date2   *string `json:"tdate2"`
	Source  []struct {
		Name   string `json:"sourceName"`
		Line   int    `json:"sourceLine"`
		Column int    `json:"sourceColumn"`
	} `json:"tsourcepos"`
	Date        string      `json:"tdate"`
	Description string      `json:"tdescription"`
	Tags        [][2]string `json:"ttags"`
	Postings    []struct {
		Status    string          `json:"pstatus"`
		Comment   string          `json:"pcomment"`
		Date      *string         `json:"pdate"`
		Date2     *string         `json:"pdate2"`
		Tags      [][2]string     `json:"ptags"`
		Assertion json.RawMessage `json:"pbalanceassertion"`
		Original  json.RawMessage `json:"poriginal"`
		Account   string          `json:"paccount"`
		Type      string          `json:"ptype"`
		Amounts   []struct {
			Cost      json.RawMessage `json:"acost"`
			CostBasis json.RawMessage `json:"acostbasis"`
			Commodity string          `json:"acommodity"`
			Quantity  struct {
				Mantissa json.Number `json:"decimalMantissa"`
				Places   int         `json:"decimalPlaces"`
			} `json:"aquantity"`
		} `json:"pamount"`
	} `json:"tpostings"`
}

func (b *Books) transactions(journal string, options ...string) ([]journalTransaction, error) {
	output, err := b.run(journal, append([]string{"print", "--output-format=json"}, options...)...)
	if err != nil {
		return nil, err
	}
	var transactions []journalTransaction
	if err := json.Unmarshal([]byte(output), &transactions); err != nil {
		return nil, fmt.Errorf("decode hledger transactions: %w", err)
	}
	if transactions == nil {
		return nil, fmt.Errorf("decode hledger transactions: expected a transaction array")
	}
	return transactions, nil
}

func Open(cfg Config) (*Books, error) {
	binary, err := exec.LookPath("hledger")
	if err != nil {
		return nil, fmt.Errorf("wat requires hledger on PATH: %w", err)
	}
	chart, err := loadAccountChart(cfg.AccountChart)
	if err != nil {
		return nil, err
	}
	books := &Books{Config: cfg, binary: binary, chart: chart}
	if err := books.checkSyntax(cfg.Journal); err != nil {
		return nil, err
	}
	if _, err := books.snapshot(); err != nil {
		return nil, err
	}
	accounts, err := books.Accounts()
	if err != nil {
		return nil, err
	}
	for _, bank := range cfg.BankAccounts {
		if !slices.Contains(accounts, bank.Account) {
			return nil, fmt.Errorf("configured bank account must be a declared local posting account: %q", bank.Account)
		}
	}
	if err := books.verifyReconciliations(); err != nil {
		return nil, fmt.Errorf("verify reconciliations: %w", err)
	}
	return books, nil
}

func (b *Books) run(journal string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	arguments := append([]string{"--no-conf", "--file", journal, "--color=no", "--pager=no"}, args...)
	command := exec.CommandContext(ctx, b.binary, arguments...)
	command.Dir = b.Config.root
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("hledger %s: %w", strings.Join(args, " "), ctx.Err())
		}
		return "", fmt.Errorf("hledger %s: %w\n%s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func (b *Books) Check() error {
	if err := b.checkSyntax(b.Config.Journal); err != nil {
		return err
	}
	accounts, err := b.Accounts()
	if err != nil {
		return err
	}
	transactions, err := b.transactions(b.Config.Journal)
	if err != nil {
		return err
	}
	return checkPostingAccounts(transactions, accounts)
}

func (b *Books) checkSyntax(journal string) error {
	_, err := b.run(journal, "check", "--strict")
	return err
}

func (b *Books) declaredAccounts() ([]string, error) {
	text, err := b.run(b.Config.Journal, "accounts", "--declared", "--flat")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("no declared accounts; add account declarations to the journal")
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n"), nil
}

func (b *Books) Accounts() ([]string, error) {
	accounts, err := b.PostingAccounts()
	if err != nil {
		return nil, err
	}
	posting := make([]string, 0, len(accounts))
	for _, account := range accounts {
		posting = append(posting, account.Name)
	}
	return posting, nil
}

func checkPostingAccounts(transactions []journalTransaction, accounts []string) error {
	for _, tx := range transactions {
		for _, posting := range tx.Postings {
			if !slices.Contains(accounts, posting.Account) {
				return fmt.Errorf("%s %s: posting account %q must be a declared Category:CPUC-number:LocalName account; revise this entry", tx.Date, tx.Description, posting.Account)
			}
		}
	}
	return nil
}

func (b *Books) Report(name string) (string, error) {
	commands := map[string][]string{
		"journal":          {"print"},
		"trial-balance":    {"balance", "--flat", "--no-elide"},
		"balance-sheet":    {"balancesheetequity"},
		"income-statement": {"incomestatement"},
	}
	args, ok := commands[name]
	if !ok {
		return "", fmt.Errorf("unknown report %q", name)
	}
	// Balance reports include prior years; journal and income reports describe
	// activity in the selected year.
	if name == "journal" || name == "income-statement" {
		args = append(args, "--begin", fmt.Sprintf("%04d-01-01", b.Config.ReportYear))
	}
	args = append(args, "--end", fmt.Sprintf("%04d-01-01", b.Config.ReportYear+1))
	text, err := b.run(b.Config.Journal, append(args, "--strict")...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "No transactions for " + strconv.Itoa(b.Config.ReportYear) + ".\n", nil
	}
	return text, nil
}
