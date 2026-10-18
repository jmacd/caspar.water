package wat

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version       int                 `yaml:"version"`
	Name          string              `yaml:"name"`
	ReportYear    int                 `yaml:"report_year"`
	Journal       string              `yaml:"journal"`
	AccountChart  string              `yaml:"account_chart"`
	ManualJournal string              `yaml:"manual_journal"`
	Sources       string              `yaml:"sources"`
	BankAccounts  []BankAccountConfig `yaml:"bank_accounts"`
	Statement     StatementConfig     `yaml:"statement"`
	root          string
}

type BankAccountConfig struct {
	Account string `yaml:"account"`
	Journal string `yaml:"journal"`
}

type StatementConfig struct {
	Python     string `yaml:"python"`
	APIBase    string `yaml:"api_base"`
	APIVersion string `yaml:"api_version"`
	Model      string `yaml:"model"`
	OllamaURL  string `yaml:"ollama_url,omitempty"`
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
	if cfg.Version != 2 {
		return cfg, fmt.Errorf("unsupported configuration version %d (expected 2)", cfg.Version)
	}
	if strings.TrimSpace(cfg.Name) == "" || cfg.ReportYear < 1 || cfg.ReportYear > 9998 {
		return cfg, fmt.Errorf("name and a report_year between 1 and 9998 are required")
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
	for _, item := range []struct {
		name string
		path *string
	}{
		{"journal", &cfg.Journal},
		{"account_chart", &cfg.AccountChart},
		{"manual_journal", &cfg.ManualJournal},
		{"sources", &cfg.Sources},
	} {
		if err := resolveConfigPath(cfg.root, item.name, item.path); err != nil {
			return cfg, err
		}
	}
	accounts := make(map[string]bool)
	identifiers := make(map[string]bool)
	journals := map[string]bool{cfg.ManualJournal: true}
	for index := range cfg.BankAccounts {
		bank := &cfg.BankAccounts[index]
		account, valid := parsePostingAccount(bank.Account)
		if strings.TrimSpace(bank.Account) == "" || !singleLine(bank.Account) ||
			strings.Contains(bank.Account, "  ") || strings.ContainsAny(bank.Account, ";[]()") ||
			!valid || account.Category != "Bank" {
			return cfg, fmt.Errorf("bank_accounts[%d].account must use Bank:CPUC-number:local-name", index)
		}
		if accounts[bank.Account] {
			return cfg, fmt.Errorf("bank account %q is configured more than once", bank.Account)
		}
		identifier := normalizeAccountIdentifier(bank.Account[strings.LastIndex(bank.Account, ":")+1:])
		if identifier == "" || identifiers[identifier] {
			return cfg, fmt.Errorf("bank account %q must have a unique final account-identifier component", bank.Account)
		}
		if err := resolveConfigPath(cfg.root, fmt.Sprintf("bank_accounts[%d].journal", index), &bank.Journal); err != nil {
			return cfg, err
		}
		if journals[bank.Journal] {
			return cfg, fmt.Errorf("journal %s is assigned to more than one writable account", bank.Journal)
		}
		accounts[bank.Account], identifiers[identifier], journals[bank.Journal] = true, true, true
	}
	return cfg, nil
}

func resolveConfigPath(root, name string, path *string) error {
	if strings.TrimSpace(*path) == "" || strings.ContainsAny(*path, "\r\n") {
		return fmt.Errorf("%s must be a nonempty local path", name)
	}
	resolved := *path
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(root, resolved)
	}
	resolved = filepath.Clean(resolved)
	if !within(root, resolved) {
		return fmt.Errorf("%s must be inside the configuration directory", name)
	}
	*path = resolved
	return nil
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
