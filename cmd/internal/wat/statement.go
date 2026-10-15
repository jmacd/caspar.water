package wat

import (
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

//go:embed statement_parser.py
var statementParser string

type Statement struct {
	Hash, Source string
	Rows         []StatementRow
	Warnings     []string
}

type StatementRow struct {
	Date, Description string
	Cents             int64
}

type StatementOptions struct {
	Extraction string
	Pages      []int
}

func (s *Statement) key(index int) string {
	return fmt.Sprintf("%s/%d", s.Hash, index+1)
}

func descriptionKey(text string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.Join(strings.Fields(text), " "))))
	return hex.EncodeToString(sum[:])
}

func (b *Books) LoadStatement(path string, options StatementOptions, progress io.Writer) (*Statement, error) {
	if progress == nil {
		progress = io.Discard
	}
	if b.Config.BankAccount == "" {
		return nil, fmt.Errorf("set bank_account in the workspace YAML before opening a statement")
	}
	pdf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read statement PDF: %w", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		return nil, fmt.Errorf("statement must be a PDF")
	}
	sum := sha256.Sum256(pdf)
	hash := hex.EncodeToString(sum[:])
	root := filepath.Join(b.Config.Sources, "statements")
	// Refuse symlink components before creating or writing source files.
	relative, err := filepath.Rel(b.Config.root, root)
	if err != nil || !filepath.IsLocal(relative) {
		return nil, fmt.Errorf("statement source directory must be inside the workspace")
	}
	current := b.Config.root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if err := os.Mkdir(current, 0700); err != nil && !os.IsExist(err) {
			return nil, err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("statement source path is not a regular directory: %s", current)
		}
	}
	dir := filepath.Join(root, hash)
	if info, err := os.Lstat(dir); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("statement cache is not a regular directory")
		}
		for _, name := range []string{"original.pdf", "extracted.json"} {
			info, err := os.Lstat(filepath.Join(dir, name))
			if err != nil || !info.Mode().IsRegular() {
				return nil, fmt.Errorf("statement cache is incomplete or contains nonregular files: %s", dir)
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	cached, err := os.ReadFile(filepath.Join(dir, "extracted.json"))
	if err == nil {
		if options.Extraction != "" || len(options.Pages) != 0 {
			return nil, fmt.Errorf("statement already cached; omit --statement-extraction and --statement-pages to reuse its stable row identities")
		}
		original, err := os.ReadFile(filepath.Join(dir, "original.pdf"))
		if err != nil || sha256.Sum256(original) != sum {
			return nil, fmt.Errorf("cached statement PDF is missing or does not match its fingerprint")
		}
		return b.decodeStatement(cached, hash, dir)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read cached statement: %w", err)
	}
	var data []byte
	if options.Extraction != "" {
		data, err = os.ReadFile(options.Extraction)
	} else {
		settings := b.Config.Statement
		if settings.Python == "" || settings.OllamaURL == "" || settings.Model == "" {
			return nil, fmt.Errorf("uncached PDF: configure statement.python, statement.ollama_url and statement.model in YAML; install cmd/wat/requirements-statement.txt in that Python environment")
		}
		python := settings.Python
		if strings.ContainsAny(python, `/\`) && !filepath.IsAbs(python) {
			python = filepath.Join(b.Config.root, python)
		}
		pages, _ := json.Marshal(options.Pages)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, python, "-c", statementParser, path, settings.OllamaURL, settings.Model, string(pages))
		cmd.Stderr = progress
		fmt.Fprintln(progress, "Extracting bank statement with configured Ollama; no journal entries will be written.")
		data, err = cmd.Output()
		if ctx.Err() != nil {
			return nil, fmt.Errorf("statement extraction: %w", ctx.Err())
		}
	}
	if err != nil {
		return nil, fmt.Errorf("statement extraction failed: %w", err)
	}
	statement, err := b.decodeStatement(data, hash, dir)
	if err != nil {
		return nil, err
	}
	stage, err := os.MkdirTemp(root, ".extract-")
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := os.RemoveAll(stage); err != nil {
			fmt.Fprintf(progress, "Statement temporary-file cleanup failed: %v\n", err)
		}
	}()
	for name, contents := range map[string][]byte{"original.pdf": pdf, "extracted.json": data} {
		file, err := os.CreateTemp(stage, ".source-")
		if err != nil {
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
		if err := writeSynced(file.Name(), contents, 0600); err != nil {
			return nil, err
		}
		if err := os.Rename(file.Name(), filepath.Join(stage, name)); err != nil {
			return nil, err
		}
	}
	stageDir, err := os.Open(stage)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(stageDir.Sync(), stageDir.Close()); err != nil {
		return nil, err
	}
	if err := os.Rename(stage, dir); err != nil {
		return nil, fmt.Errorf("publish statement cache: %w", err)
	}
	parent, err := os.Open(root)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(parent.Sync(), parent.Close()); err != nil {
		return nil, fmt.Errorf("statement cached but directory sync failed: %w", err)
	}
	return statement, nil
}

func (b *Books) decodeStatement(data []byte, hash, dir string) (*Statement, error) {
	var raw struct {
		Transactions []struct {
			Date        string `json:"booking_date"`
			Description string `json:"description"`
			Currency    string `json:"currency"`
			Amount      string `json:"amount"`
		} `json:"transactions"`
		Verification *struct {
			Opening string `json:"opening_balance"`
			Closing string `json:"closing_balance"`
		} `json:"verification"`
		Warnings []string `json:"warnings"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode statement extraction: %w", err)
	}
	if raw.Verification == nil || len(raw.Transactions) == 0 {
		return nil, fmt.Errorf("statement must contain transactions and opening/closing balances")
	}
	opening, err := ParseCents(raw.Verification.Opening)
	if err != nil {
		return nil, fmt.Errorf("statement opening balance: %w", err)
	}
	closing, err := ParseCents(raw.Verification.Closing)
	if err != nil {
		return nil, fmt.Errorf("statement closing balance: %w", err)
	}
	source, err := filepath.Rel(b.Config.root, filepath.Join(dir, "original.pdf"))
	if err != nil {
		return nil, err
	}
	result := &Statement{Hash: hash, Source: filepath.ToSlash(source), Warnings: raw.Warnings}
	total := big.NewInt(opening)
	for i, tx := range raw.Transactions {
		date, dateErr := time.Parse("2006-01-02", tx.Date)
		cents, amountErr := ParseCents(tx.Amount)
		if dateErr != nil || date.Year() != b.Config.Year {
			return nil, fmt.Errorf("statement row %d must have a valid date within %d", i+1, b.Config.Year)
		}
		if amountErr != nil || cents == 0 || tx.Currency != "USD" {
			return nil, fmt.Errorf("statement row %d requires a nonzero exact USD amount", i+1)
		}
		if strings.TrimSpace(tx.Description) == "" || !singleLine(tx.Description) || strings.ContainsAny(tx.Description, ";|") {
			return nil, fmt.Errorf("statement row %d requires a single-line description without journal delimiters ';' or '|'", i+1)
		}
		total.Add(total, big.NewInt(cents))
		result.Rows = append(result.Rows, StatementRow{tx.Date, tx.Description, cents})
	}
	if total.Cmp(big.NewInt(closing)) != 0 {
		return nil, fmt.Errorf("statement transactions do not match opening and closing balances; review the extraction against the PDF")
	}
	return result, nil
}

func bankAmount(tx journalTransaction, account string) (int64, error) {
	total := new(big.Int)
	for _, posting := range tx.Postings {
		if posting.Account != account {
			continue
		}
		if posting.Type != "RegularPosting" {
			return 0, fmt.Errorf("bank posting must be a regular posting")
		}
		for _, amount := range posting.Amounts {
			if amount.Commodity != "$" || amount.Quantity.Places < 0 || amount.Quantity.Places > 1000 {
				return 0, fmt.Errorf("bank posting must use exact USD amounts")
			}
			value, valid := new(big.Int).SetString(amount.Quantity.Mantissa.String(), 10)
			if !valid {
				return 0, fmt.Errorf("bank posting has an invalid USD quantity")
			}
			scale := amount.Quantity.Places - 2
			factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(abs(scale))), nil)
			if scale > 0 {
				remainder := new(big.Int)
				value.QuoRem(value, factor, remainder)
				if remainder.Sign() != 0 {
					return 0, fmt.Errorf("bank posting has fractional cents")
				}
			} else {
				value.Mul(value, factor)
			}
			total.Add(total, value)
		}
	}
	if !total.IsInt64() {
		return 0, fmt.Errorf("bank posting exceeds supported USD range")
	}
	return total.Int64(), nil
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func transactionTag(tx journalTransaction, name string) string {
	for _, tag := range tx.Tags {
		if tag[0] == name {
			return tag[1]
		}
	}
	return ""
}

type StatementStatus struct {
	Recorded      bool
	PossibleMatch bool
	Suggestion    string
}

func (b *Books) StatementStatuses(s *Statement) ([]StatementStatus, error) {
	transactions, err := b.transactions(b.Config.Journal, "--strict")
	if err != nil {
		return nil, err
	}
	accounts, err := b.Accounts()
	if err != nil {
		return nil, err
	}
	statuses := make([]StatementStatus, len(s.Rows))
	for index, row := range s.Rows {
		choices := make(map[string]bool)
		for _, tx := range transactions {
			for _, name := range []string{"bank-row", "bank-description"} {
				count := 0
				for _, tag := range tx.Tags {
					if tag[0] == name {
						count++
					}
				}
				if count > 1 {
					return nil, fmt.Errorf("journal contains duplicate %s tags", name)
				}
			}
			amount, err := bankAmount(tx, b.Config.BankAccount)
			if err != nil {
				return nil, err
			}
			if transactionTag(tx, "bank-row") == s.key(index) {
				if statuses[index].Recorded {
					return nil, fmt.Errorf("statement row %d is recorded more than once", index+1)
				}
				if tx.Date != row.Date || amount != row.Cents || transactionTag(tx, "bank-description") != descriptionKey(row.Description) {
					return nil, fmt.Errorf("recorded statement row %d no longer matches its bank date/amount", index+1)
				}
				statuses[index].Recorded = true
			}
			if tx.Date == row.Date && amount == row.Cents {
				statuses[index].PossibleMatch = true
			}
			payee, _, _ := strings.Cut(tx.Description, "|")
			if amount == 0 || (amount < 0) != (row.Cents < 0) ||
				(transactionTag(tx, "bank-description") != descriptionKey(row.Description) && descriptionKey(payee) != descriptionKey(row.Description)) {
				continue
			}
			other := ""
			for _, posting := range tx.Postings {
				if posting.Account != b.Config.BankAccount {
					if other != "" && other != posting.Account {
						other = ""
						break
					}
					other = posting.Account
				}
			}
			if other != "" && slices.Contains(accounts, other) {
				choices[other] = true
			}
		}
		if len(choices) == 1 {
			for account := range choices {
				statuses[index].Suggestion = account
			}
		}
	}
	return statuses, nil
}

type statementDraft struct {
	statement   *Statement
	index       int
	bankAccount string
}

func (b *Books) StatementDraft(s *Statement, index int) (*Draft, Entry, error) {
	if index < 0 || index >= len(s.Rows) {
		return nil, Entry{}, fmt.Errorf("statement row is out of range")
	}
	statuses, err := b.StatementStatuses(s)
	if err != nil {
		return nil, Entry{}, err
	}
	if statuses[index].Recorded {
		return nil, Entry{}, fmt.Errorf("statement row is already recorded")
	}
	draft, err := b.NewDraft()
	if err != nil {
		return nil, Entry{}, err
	}
	row := s.Rows[index]
	draft.statement = &statementDraft{s, index, b.Config.BankAccount}
	entry := Entry{Date: row.Date, Payee: row.Description, Source: s.Source,
		BankRow: s.key(index), BankDescription: descriptionKey(row.Description),
		Postings: []Posting{{b.Config.BankAccount, formatCents(row.Cents)}, {statuses[index].Suggestion, formatCents(-row.Cents)}}}
	return draft, entry, nil
}

func (d *Draft) checkStatement(tx journalTransaction) error {
	if d.statement == nil {
		return nil
	}
	source := d.statement
	row := source.statement.Rows[source.index]
	for name, expected := range map[string]string{
		"bank-row": source.statement.key(source.index), "bank-description": descriptionKey(row.Description),
	} {
		count := 0
		for _, tag := range tx.Tags {
			if tag[0] == name {
				count++
				if tag[1] != expected {
					return fmt.Errorf("keep statement provenance tag %s unchanged", name)
				}
			}
		}
		if count != 1 {
			return fmt.Errorf("keep exactly one statement provenance tag %s", name)
		}
	}
	amount, err := bankAmount(tx, source.bankAccount)
	if err != nil {
		return err
	}
	if tx.Date != row.Date || amount != row.Cents {
		return fmt.Errorf("keep the statement's bank account, date and signed bank amount unchanged; edit only the counterpostings")
	}
	return nil
}
