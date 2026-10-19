package wat

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/google/shlex"
)

const editorHelpPrefix = "; wat-help: "

func editorHelp(accounts []string) string {
	lines := []string{
		"EDITOR CHEATSHEET - temporary, removed before preview and save.",
		"Edit the transaction ABOVE this cheatsheet; keep its id tag unchanged.",
		"Use a valid YYYY-MM-DD date. Reference/source tags are optional.",
		"Posting syntax: indent the account, then at least two spaces, then $amount.",
		"Positive amounts are debits; negative amounts are credits. The sum must be zero.",
		"Example posting (replace the account and amount):",
		"    ACCOUNT-NAME  $125.00",
		"    ACCOUNT-NAME  $-125.00",
		"",
		"Copy/paste these declared Category:CPUC-number:LocalName accounts into postings above:",
	}
	for _, account := range accounts {
		lines = append(lines, "    "+account)
	}
	var help strings.Builder
	for _, line := range lines {
		help.WriteString(editorHelpPrefix + line + "\n")
	}
	return help.String()
}

func stripEditorHelp(text string) string {
	var transaction strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		if !strings.HasPrefix(strings.TrimLeft(line, " \t"), editorHelpPrefix) {
			transaction.WriteString(line)
		}
	}
	return transaction.String()
}

func (b *Books) editText(initial string) (text string, err error) {
	text = stripEditorHelp(initial)
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}
	args, err := shlex.Split(editor)
	if err != nil || len(args) == 0 {
		return text, fmt.Errorf("invalid editor command %q: expected an executable and optional quoted arguments", editor)
	}
	accounts, err := b.Accounts()
	if err != nil {
		return text, fmt.Errorf("load editor account cheatsheet: %w", err)
	}
	root, err := os.MkdirTemp("", "wat-editor-")
	if err != nil {
		return text, err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(root)) }()
	path := filepath.Join(root, "transaction.journal")
	editorText := text
	if !strings.HasSuffix(editorText, "\n") {
		editorText += "\n"
	}
	editorText += editorHelp(accounts)
	if err := os.WriteFile(path, []byte(editorText), 0600); err != nil {
		return text, err
	}
	command := exec.Command(args[0], append(args[1:], path)...)
	command.Dir = b.Config.root
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	runErr := command.Run()
	data, readErr := os.ReadFile(path)
	if readErr == nil {
		text = stripEditorHelp(string(data))
	}
	if runErr != nil {
		runErr = fmt.Errorf("editor exited unsuccessfully: %w", runErr)
	}
	if readErr != nil {
		readErr = fmt.Errorf("read edited transaction: %w", readErr)
	}
	return text, errors.Join(runErr, readErr)
}

func (d *Draft) PreviewText(text string) (err error) {
	return d.previewText(text, true)
}

func (d *Draft) previewText(text string, requireDetail bool) (err error) {
	if d.saved {
		return fmt.Errorf("this transaction has already been saved")
	}
	d.text = ""
	// Only the transaction is editable, not directives affecting other books.
	headers := 0
	for _, line := range strings.Split(text, "\n") {
		if strings.ContainsFunc(line, func(r rune) bool {
			return unicode.IsControl(r) && r != '\t' && r != '\r'
		}) {
			return fmt.Errorf("transaction text contains a control character")
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			headers++
			if len(line) < 10 {
				return fmt.Errorf("edit exactly one transaction with a YYYY-MM-DD date; journal directives are not allowed")
			}
			if _, err := time.Parse("2006-01-02", line[:10]); err != nil {
				return fmt.Errorf("edit exactly one transaction with a YYYY-MM-DD date; journal directives are not allowed")
			}
		}
	}
	if headers != 1 {
		return fmt.Errorf("edit exactly one transaction, not %d transaction headers", headers)
	}
	root, err := os.MkdirTemp("", "wat-parse-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(root)) }()
	path := filepath.Join(root, "transaction.journal")
	if err := os.WriteFile(path, []byte(text), 0600); err != nil {
		return err
	}
	// Parse with hledger, not a Go journal parser. Assertions are validated
	// below against the complete books, rather than this isolated transaction.
	transactions, err := d.books.transactions(path, "--ignore-assertions")
	if err != nil {
		return err
	}
	if len(transactions) != 1 {
		return fmt.Errorf("expected exactly one transaction, got %d", len(transactions))
	}
	tx := transactions[0]
	if _, err := time.Parse("2006-01-02", tx.Date); err != nil {
		return fmt.Errorf("transaction must have a valid YYYY-MM-DD date")
	}
	if strings.TrimSpace(tx.Description) == "" || len(tx.Postings) < 2 {
		return fmt.Errorf("a payee/description and at least two postings are required")
	}
	ids := 0
	for _, tag := range tx.Tags {
		if tag[0] == "id" {
			ids++
			if tag[1] != d.id {
				return fmt.Errorf("keep the transaction identifier: ; id: %s", d.id)
			}
		}
	}
	if ids != 1 {
		return fmt.Errorf("keep exactly one transaction identifier: ; id: %s", d.id)
	}
	for _, posting := range tx.Postings {
		if posting.Type != "RegularPosting" {
			return fmt.Errorf("only regular financial postings are supported")
		}
		for _, amount := range posting.Amounts {
			if amount.Commodity != "$" {
				return fmt.Errorf("use the $ commodity for USD postings")
			}
		}
	}
	if err := d.checkStatement(tx); err != nil {
		return err
	}
	if d.revision != nil && d.statement != nil {
		sources := 0
		for _, tag := range tx.Tags {
			if tag[0] == "source" {
				sources++
				if tag[1] != d.revision.source {
					return fmt.Errorf("keep the recorded statement's source tag unchanged")
				}
			}
		}
		if sources != 1 {
			return fmt.Errorf("keep exactly one recorded statement source tag")
		}
	}
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if err := d.unchanged(); err != nil {
		return err
	}
	if err := d.validateBooks(d.proposedText(text), requireDetail); err != nil {
		return fmt.Errorf("proposed books failed validation: %w", err)
	}
	d.text = text
	return nil
}
