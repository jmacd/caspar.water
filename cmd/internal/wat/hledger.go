package wat

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type Books struct {
	Config Config
	binary string
}

func Open(cfg Config) (*Books, error) {
	binary, err := exec.LookPath("hledger")
	if err != nil {
		return nil, fmt.Errorf("wat requires hledger on PATH: %w", err)
	}
	books := &Books{Config: cfg, binary: binary}
	if err := books.Check(); err != nil {
		return nil, err
	}
	if _, err := books.snapshot(); err != nil {
		return nil, err
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
	_, err := b.run(b.Config.Journal, "check", "--strict")
	return err
}

func (b *Books) Accounts() ([]string, error) {
	text, err := b.run(b.Config.Journal, "accounts", "--declared", "--flat")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("no declared accounts; add account declarations to the journal")
	}
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n"), nil
}

func (b *Books) Report(name string) (string, error) {
	commands := map[string][]string{
		"journal":          {"print"},
		"trial-balance":    {"balance", "--flat", "--no-elide"},
		"balance-sheet":    {"balancesheet"},
		"income-statement": {"incomestatement"},
	}
	args, ok := commands[name]
	if !ok {
		return "", fmt.Errorf("unknown report %q", name)
	}
	// Balance reports include prior years; journal and income reports describe
	// activity in the selected year.
	if name == "journal" || name == "income-statement" {
		args = append(args, "--begin", fmt.Sprintf("%04d-01-01", b.Config.Year))
	}
	args = append(args, "--end", fmt.Sprintf("%04d-01-01", b.Config.Year+1))
	text, err := b.run(b.Config.Journal, append(args, "--strict")...)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(text) == "" {
		return "No transactions for " + strconv.Itoa(b.Config.Year) + ".\n", nil
	}
	return text, nil
}
