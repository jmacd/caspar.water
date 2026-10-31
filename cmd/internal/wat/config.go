package wat

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	booksDirectory   = "books"
	masterJournal    = "books.journal"
	chartJournal     = "chart.journal"
	accountsJournal  = "accounts.journal"
	manualJournal    = "manual.journal"
	sourcesDirectory = "sources"
)

var (
	filenameSeparatorPattern = regexp.MustCompile(`[._-]+`)
	localAccountPartPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
)

type Config struct {
	Version            int                 `yaml:"version"`
	Name               string              `yaml:"name"`
	Accounts           []string            `yaml:"accounts"`
	BankAccounts       []BankAccountConfig `yaml:"bank_accounts"`
	ChartDefPaths      []string            `yaml:"chart_defs,omitempty"`
	Statement          StatementConfig     `yaml:"statement"`
	Journal            string              `yaml:"-"`
	ChartDefs          []string            `yaml:"-"`
	LocalAccounts      string              `yaml:"-"`
	ManualJournal      string              `yaml:"-"`
	Sources            string              `yaml:"-"`
	QuickBooksMappings string              `yaml:"-"`
	root               string
}

type BankAccountConfig struct {
	Account string `yaml:"account"`
	Journal string `yaml:"-"`
}

type StatementConfig struct {
	Python     string `yaml:"python"`
	APIBase    string `yaml:"api_base"`
	APIVersion string `yaml:"api_version"`
	Model      string `yaml:"model"`
	OllamaURL  string `yaml:"ollama_url,omitempty"`
}

func (b *BankAccountConfig) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.ScalarNode {
		return fmt.Errorf("bank account must be a Category:CPUC-number:LocalName string")
	}
	b.Account = node.Value
	return nil
}

func LoadConfig(path string) (Config, error) {
	var cfg Config
	file, err := os.Open(path)
	if err != nil {
		return cfg, fmt.Errorf("open configuration: %w", err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return cfg, fmt.Errorf("decode configuration: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return cfg, fmt.Errorf("configuration must contain exactly one YAML document")
	}
	if cfg.Version != 3 {
		return cfg, fmt.Errorf("unsupported configuration version %d (expected 3)", cfg.Version)
	}
	if strings.TrimSpace(cfg.Name) == "" {
		return cfg, fmt.Errorf("name is required")
	}
	if strings.TrimSpace(cfg.Statement.APIBase) != "" && strings.TrimSpace(cfg.Statement.OllamaURL) != "" {
		return cfg, fmt.Errorf("statement.api_base and legacy statement.ollama_url cannot both be set")
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return cfg, err
	}
	cfg.root, err = filepath.EvalSymlinks(filepath.Dir(absolute))
	if err != nil {
		return cfg, fmt.Errorf("resolve configuration directory: %w", err)
	}
	books := filepath.Join(cfg.root, booksDirectory)
	cfg.Journal = filepath.Join(books, masterJournal)
	cfg.LocalAccounts = filepath.Join(books, accountsJournal)
	cfg.ManualJournal = filepath.Join(books, manualJournal)
	cfg.Sources = filepath.Join(cfg.root, sourcesDirectory)
	cfg.QuickBooksMappings = filepath.Join(cfg.root, "quickbooks-mappings.yaml")
	if cfg.ChartDefPaths == nil {
		cfg.ChartDefPaths = []string{filepath.Join(booksDirectory, chartJournal)}
	}
	if len(cfg.ChartDefPaths) == 0 {
		return cfg, fmt.Errorf("chart_defs must contain at least one path")
	}
	chartDefs := make(map[string]bool, len(cfg.ChartDefPaths))
	for index, path := range cfg.ChartDefPaths {
		if strings.TrimSpace(path) != path ||
			!filepath.IsLocal(path) ||
			filepath.Clean(path) == "." {
			return cfg, fmt.Errorf(
				"chart_defs[%d] must be a workspace-relative file path",
				index,
			)
		}
		resolved := filepath.Join(cfg.root, filepath.Clean(path))
		if chartDefs[resolved] {
			return cfg, fmt.Errorf(
				"chart_defs[%d] duplicates chart definition path %q",
				index,
				path,
			)
		}
		chartDefs[resolved] = true
		cfg.ChartDefs = append(cfg.ChartDefs, resolved)
	}

	accounts := make(map[string]bool)
	identifiers := make(map[string]bool)
	journals := make(map[string]bool)
	for index, name := range cfg.Accounts {
		account, valid := parsePostingAccount(name)
		if !valid || account.Category == "Bank" {
			return cfg, fmt.Errorf("accounts[%d] must use Asset, Equity, Liability, Expense, or Income:CPUC-number:local-name", index)
		}
		if accounts[name] {
			return cfg, fmt.Errorf("account %q is configured more than once", name)
		}
		accounts[name] = true
	}
	for index := range cfg.BankAccounts {
		bank := &cfg.BankAccounts[index]
		account, valid := parsePostingAccount(bank.Account)
		if !valid || account.Category != "Bank" {
			return cfg, fmt.Errorf("bank_accounts[%d] must use Bank:CPUC-number:local-name", index)
		}
		if accounts[bank.Account] {
			return cfg, fmt.Errorf("bank account %q is configured more than once", bank.Account)
		}
		identifier := normalizeAccountIdentifier(bank.Account[strings.LastIndex(bank.Account, ":")+1:])
		if identifier == "" || identifiers[identifier] {
			return cfg, fmt.Errorf("bank account %q must have a unique final account-identifier component", bank.Account)
		}
		bank.Journal = filepath.Join(books, bankJournalFilename(bank.Account))
		if journals[bank.Journal] {
			return cfg, fmt.Errorf("bank accounts derive the same journal filename %s", bankJournalFilename(bank.Account))
		}
		accounts[bank.Account], identifiers[identifier], journals[bank.Journal] = true, true, true
	}
	reserved := append(
		[]string{absolute, cfg.Journal, cfg.LocalAccounts, cfg.ManualJournal, cfg.QuickBooksMappings},
		cfg.writableJournals()...,
	)
	for index, chartDef := range cfg.ChartDefs {
		for _, path := range reserved {
			if chartDef == path {
				return cfg, fmt.Errorf(
					"chart_defs[%d] conflicts with workspace file %s",
					index,
					cfg.ChartDefPaths[index],
				)
			}
		}
	}
	return cfg, nil
}

func bankJournalFilename(account string) string {
	parts := strings.Split(account, ":")
	tail := strings.ToLower(strings.Join(parts[2:], "-"))
	tail = filenameSeparatorPattern.ReplaceAllString(tail, "-")
	return "bank-" + tail + ".journal"
}

func validLocalAccountName(name string) bool {
	if strings.TrimSpace(name) != name || !singleLine(name) {
		return false
	}
	parts := strings.Split(name, ":")
	for _, part := range parts[2:] {
		if !localAccountPartPattern.MatchString(part) {
			return false
		}
	}
	return true
}

func (c Config) bankAccount(account string) (BankAccountConfig, bool) {
	for _, bank := range c.BankAccounts {
		if bank.Account == account {
			return bank, true
		}
	}
	return BankAccountConfig{}, false
}

func (c Config) writableJournals() []string {
	journals := []string{c.ManualJournal}
	for _, bank := range c.BankAccounts {
		journals = append(journals, bank.Journal)
	}
	return journals
}

func (c Config) writableJournal(path string) bool {
	for _, journal := range c.writableJournals() {
		if path == journal {
			return true
		}
	}
	return false
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && filepath.IsLocal(rel)
}
