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
	"unicode"
)

//go:embed statement_parser.py
var statementParser string

type Statement struct {
	Hash, Source string
	BankAccount  string
	Period       *StatementPeriod
	OpeningCents int64
	ClosingCents int64
	Rows         []StatementRow
	Warnings     []string
}

type StatementPeriod struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type StatementRow struct {
	Date, Description string
	Cents             int64
}

type StatementOptions struct {
	Extraction string
}

type PendingStatement struct {
	Hash, Name, Source string
}

type statementSourceMetadata struct {
	Name string `json:"name"`
}

func (b *Books) ImportStatementInbox(path string, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("open statement inbox: %w", err)
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("statement inbox is not a regular directory: %s", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read statement inbox: %w", err)
	}
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".pdf") {
			continue
		}
		entryInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("inspect statement inbox file %q: %w", entry.Name(), err)
		}
		if !entryInfo.Mode().IsRegular() || entryInfo.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("statement inbox PDF is not a regular file: %s", filepath.Join(path, entry.Name()))
		}
		statementPath := filepath.Join(path, entry.Name())
		imported, err := b.importPendingStatement(statementPath, entry.Name())
		if err != nil {
			return fmt.Errorf("import statement inbox file %q: %w", entry.Name(), err)
		}
		if imported {
			fmt.Fprintln(progress, "Copied statement inbox file into accounting storage:", entry.Name())
		}
	}
	return nil
}

func (b *Books) importPendingStatement(path, name string) (bool, error) {
	pdf, err := readStatementPDF(path)
	if err != nil {
		return false, err
	}
	hash := statementHash(pdf)
	root, err := b.prepareStatementCacheRoot()
	if err != nil {
		return false, err
	}
	dir := filepath.Join(root, hash)
	if info, err := os.Lstat(dir); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("statement cache is not a regular directory: %s", dir)
		}
		if _, err := validatedStatementPDF(dir, hash); err != nil {
			return false, err
		}
		extracted := filepath.Join(dir, "extracted.json")
		if info, err := os.Lstat(extracted); err == nil {
			if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return false, fmt.Errorf("statement cache is incomplete or contains nonregular files: %s", dir)
			}
			return false, nil
		} else if !os.IsNotExist(err) {
			return false, err
		}
		if _, err := b.pendingStatement(hash); err != nil {
			return false, err
		}
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}
	stage, err := os.MkdirTemp(root, ".import-")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(stage)
	if err := createSyncedFile(filepath.Join(stage, "original.pdf"), pdf, 0600); err != nil {
		return false, err
	}
	metadata, err := json.Marshal(statementSourceMetadata{Name: name})
	if err != nil {
		return false, err
	}
	metadata = append(metadata, '\n')
	if err := createSyncedFile(filepath.Join(stage, "source.json"), metadata, 0600); err != nil {
		return false, err
	}
	stageDir, err := os.Open(stage)
	if err != nil {
		return false, err
	}
	if err := errors.Join(stageDir.Sync(), stageDir.Close()); err != nil {
		return false, err
	}
	if err := os.Rename(stage, dir); err != nil {
		return false, fmt.Errorf("publish pending statement: %w", err)
	}
	parent, err := os.Open(root)
	if err != nil {
		return false, err
	}
	if err := errors.Join(parent.Sync(), parent.Close()); err != nil {
		return false, fmt.Errorf("statement imported but directory sync failed: %w", err)
	}
	return true, nil
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
	if len(b.Config.BankAccounts) == 0 {
		return nil, fmt.Errorf("configure at least one bank_accounts entry before opening a statement")
	}
	pdf, err := readStatementPDF(path)
	if err != nil {
		return nil, err
	}
	hash := statementHash(pdf)
	root, err := b.prepareStatementCacheRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, hash)
	if _, err := os.Lstat(dir); err == nil {
		if info, extractedErr := os.Lstat(filepath.Join(dir, "extracted.json")); extractedErr == nil {
			if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("statement cache is incomplete or contains nonregular files: %s", dir)
			}
			if options.Extraction != "" {
				return nil, fmt.Errorf("statement already cached; omit --statement-extraction to reuse its stable row identities")
			}
			return b.cachedStatement(hash)
		} else if !os.IsNotExist(extractedErr) {
			return nil, extractedErr
		}
		if _, err := b.pendingStatement(hash); err != nil {
			return nil, err
		}
		return b.ExtractPendingStatement(hash, options, progress)
	} else if !os.IsNotExist(err) {
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
	pdfFile, err := os.CreateTemp(stage, ".source-")
	if err != nil {
		return nil, err
	}
	if err := pdfFile.Close(); err != nil {
		return nil, err
	}
	if err := writeSynced(pdfFile.Name(), pdf, 0600); err != nil {
		return nil, err
	}
	stagedPDF := filepath.Join(stage, "original.pdf")
	if err := os.Rename(pdfFile.Name(), stagedPDF); err != nil {
		return nil, err
	}
	data, err := b.extractStatement(stagedPDF, options, progress)
	if err != nil {
		return nil, err
	}
	statement, err := b.decodeStatement(data, hash, dir)
	if err != nil {
		return nil, err
	}
	data, err = bindStatementExtraction(data, statement.BankAccount)
	if err != nil {
		return nil, err
	}
	extractionFile, err := os.CreateTemp(stage, ".source-")
	if err != nil {
		return nil, err
	}
	if err := extractionFile.Close(); err != nil {
		return nil, err
	}
	if err := writeSynced(extractionFile.Name(), data, 0600); err != nil {
		return nil, err
	}
	if err := os.Rename(extractionFile.Name(), filepath.Join(stage, "extracted.json")); err != nil {
		return nil, err
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

func readStatementPDF(path string) ([]byte, error) {
	pdf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read statement PDF: %w", err)
	}
	if !strings.HasPrefix(string(pdf), "%PDF-") {
		return nil, fmt.Errorf("statement must be a PDF")
	}
	return pdf, nil
}

func createSyncedFile(path string, data []byte, mode os.FileMode) (err error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}

func statementHash(pdf []byte) string {
	sum := sha256.Sum256(pdf)
	return hex.EncodeToString(sum[:])
}

func (b *Books) prepareStatementCacheRoot() (string, error) {
	root := filepath.Join(b.Config.Sources, "statements")
	relative, err := filepath.Rel(b.Config.root, root)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("statement source directory must be inside the workspace")
	}
	current := b.Config.root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		if err := os.Mkdir(current, 0700); err != nil && !os.IsExist(err) {
			return "", err
		}
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("statement source path is not a regular directory: %s", current)
		}
	}
	return root, nil
}

func (b *Books) extractStatement(path string, options StatementOptions, progress io.Writer) ([]byte, error) {
	if options.Extraction != "" {
		data, err := os.ReadFile(options.Extraction)
		if err != nil {
			return nil, fmt.Errorf("statement extraction failed: %w", err)
		}
		return data, nil
	}
	settings := b.Config.Statement
	python := settings.Python
	if override := strings.TrimSpace(os.Getenv("WAT_STATEMENT_PYTHON")); override != "" {
		python = override
	}
	apiBase := strings.TrimSpace(settings.APIBase)
	apiVersion := strings.TrimSpace(settings.APIVersion)
	model := strings.TrimSpace(settings.Model)
	if apiBase == "" && strings.TrimSpace(settings.OllamaURL) != "" {
		apiBase = strings.TrimSpace(settings.OllamaURL)
		if model != "" && !strings.Contains(model, "/") {
			model = "ollama/" + model
		}
	}
	provider, _, hasProvider := strings.Cut(model, "/")
	if python == "" || apiBase == "" || model == "" || !hasProvider {
		return nil, fmt.Errorf("uncached PDF: configure statement.python (or WAT_STATEMENT_PYTHON), statement.api_base and a provider-prefixed statement.model; install cmd/wat/requirements-statement.txt in that Python environment")
	}
	if provider != "ollama" && provider != "azure" {
		return nil, fmt.Errorf("uncached PDF: statement.model provider must be ollama or azure")
	}
	if provider == "azure" && apiVersion == "" {
		return nil, fmt.Errorf("uncached PDF: configure statement.api_version for Azure")
	}
	if strings.ContainsAny(python, `/\`) && !filepath.IsAbs(python) {
		python = filepath.Join(b.Config.root, python)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "-c", statementParser, path, apiBase, apiVersion, model)
	cmd.Stderr = progress
	fmt.Fprintf(progress, "Extracting bank statement with configured %s model %s; no journal entries will be written.\n", provider, model)
	data, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("statement extraction: %w", ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("statement extraction failed: %w", err)
	}
	return data, nil
}

func (b *Books) ExtractPendingStatement(hash string, options StatementOptions, progress io.Writer) (*Statement, error) {
	if progress == nil {
		progress = io.Discard
	}
	if len(b.Config.BankAccounts) == 0 {
		return nil, fmt.Errorf("configure at least one bank_accounts entry before opening a statement")
	}
	pending, err := b.pendingStatement(hash)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(b.Config.root, filepath.Dir(pending.Source))
	extracted := filepath.Join(dir, "extracted.json")
	if info, err := os.Lstat(extracted); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("statement cache is incomplete or contains nonregular files: %s", dir)
		}
		if options.Extraction != "" {
			return nil, fmt.Errorf("statement already cached; omit extraction options to reuse its stable row identities")
		}
		return b.cachedStatement(hash)
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	data, err := b.extractStatement(filepath.Join(dir, "original.pdf"), options, progress)
	if err != nil {
		return nil, err
	}
	statement, err := b.decodeStatement(data, hash, dir)
	if err != nil {
		return nil, err
	}
	data, err = bindStatementExtraction(data, statement.BankAccount)
	if err != nil {
		return nil, err
	}
	temp, err := os.CreateTemp(dir, ".extracted-")
	if err != nil {
		return nil, err
	}
	tempName := temp.Name()
	if err := temp.Close(); err != nil {
		return nil, err
	}
	defer os.Remove(tempName)
	if err := writeSynced(tempName, data, 0600); err != nil {
		return nil, err
	}
	if err := os.Rename(tempName, extracted); err != nil {
		return nil, fmt.Errorf("publish statement extraction: %w", err)
	}
	cacheDir, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	if err := errors.Join(cacheDir.Sync(), cacheDir.Close()); err != nil {
		return nil, fmt.Errorf("statement extracted but directory sync failed: %w", err)
	}
	return statement, nil
}

func (b *Books) statementCacheRoot() (string, error) {
	root := filepath.Join(b.Config.Sources, "statements")
	relative, err := filepath.Rel(b.Config.root, root)
	if err != nil || !filepath.IsLocal(relative) {
		return "", fmt.Errorf("statement source directory must be inside the workspace")
	}
	current := b.Config.root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return "", err
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("statement source path is not a regular directory: %s", current)
		}
	}
	return root, nil
}

func validatedStatementPDF(dir, hash string) ([]byte, error) {
	path := filepath.Join(dir, "original.pdf")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("statement cache is incomplete or contains nonregular files: %s", dir)
	}
	pdf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read cached statement PDF: %w", err)
	}
	if statementHash(pdf) != hash {
		return nil, fmt.Errorf("cached statement PDF does not match its fingerprint: %s", dir)
	}
	return pdf, nil
}

func (b *Books) pendingStatement(hash string) (*PendingStatement, error) {
	fingerprint, err := hex.DecodeString(hash)
	if err != nil || len(fingerprint) != sha256.Size || hash != strings.ToLower(hash) {
		return nil, fmt.Errorf("invalid statement cache fingerprint")
	}
	root, err := b.statementCacheRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, hash)
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("statement cache is not a regular directory: %s", dir)
	}
	if _, err := validatedStatementPDF(dir, hash); err != nil {
		return nil, err
	}
	sourcePath := filepath.Join(dir, "source.json")
	info, err = os.Lstat(sourcePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("statement cache is incomplete or contains nonregular files: %s", dir)
	}
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return nil, fmt.Errorf("read pending statement source: %w", err)
	}
	var metadata statementSourceMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("decode pending statement source: %w", err)
	}
	if metadata.Name == "" || filepath.Base(metadata.Name) != metadata.Name || !strings.EqualFold(filepath.Ext(metadata.Name), ".pdf") {
		return nil, fmt.Errorf("invalid pending statement source name: %s", sourcePath)
	}
	source, err := filepath.Rel(b.Config.root, filepath.Join(dir, "original.pdf"))
	if err != nil || !filepath.IsLocal(source) {
		return nil, fmt.Errorf("pending statement source must be inside the workspace")
	}
	return &PendingStatement{Hash: hash, Name: metadata.Name, Source: filepath.ToSlash(source)}, nil
}

func (b *Books) PendingStatements() ([]*PendingStatement, error) {
	root, err := b.statementCacheRoot()
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("list pending statements: %w", err)
	}
	var statements []*PendingStatement
	for _, entry := range dirs {
		hash := entry.Name()
		fingerprint, err := hex.DecodeString(hash)
		if err != nil || len(fingerprint) != sha256.Size || hash != strings.ToLower(hash) {
			continue
		}
		if info, err := os.Lstat(filepath.Join(root, hash, "extracted.json")); err == nil {
			if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
				return nil, fmt.Errorf("statement cache is incomplete or contains nonregular files: %s", filepath.Join(root, hash))
			}
			continue
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		statement, err := b.pendingStatement(hash)
		if err != nil {
			return nil, fmt.Errorf("pending statement %s: %w", hash, err)
		}
		statements = append(statements, statement)
	}
	slices.SortStableFunc(statements, func(a, b *PendingStatement) int {
		if byName := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); byName != 0 {
			return byName
		}
		return strings.Compare(a.Hash, b.Hash)
	})
	return statements, nil
}

func (b *Books) cachedStatement(hash string) (*Statement, error) {
	if len(b.Config.BankAccounts) == 0 {
		return nil, fmt.Errorf("configure at least one bank_accounts entry before opening a statement")
	}
	fingerprint, err := hex.DecodeString(hash)
	if err != nil || len(fingerprint) != sha256.Size || hash != strings.ToLower(hash) {
		return nil, fmt.Errorf("invalid statement cache fingerprint")
	}
	root, err := b.statementCacheRoot()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, hash)
	info, err := os.Lstat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("statement cache is not a regular directory: %s", dir)
	}
	if _, err := validatedStatementPDF(dir, hash); err != nil {
		return nil, err
	}
	extracted := filepath.Join(dir, "extracted.json")
	info, err = os.Lstat(extracted)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("statement cache is incomplete or contains nonregular files: %s", dir)
	}
	data, err := os.ReadFile(extracted)
	if err != nil {
		return nil, fmt.Errorf("read cached statement: %w", err)
	}
	return b.decodeStatement(data, hash, dir)
}

func (s *Statement) sortDate() string {
	if s.Period != nil {
		return s.Period.Start
	}
	first := ""
	for _, row := range s.Rows {
		if first == "" || row.Date < first {
			first = row.Date
		}
	}
	return first
}

func (b *Books) CachedStatements() ([]*Statement, error) {
	root, err := b.statementCacheRoot()
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("list cached statements: %w", err)
	}
	var statements []*Statement
	for _, dir := range dirs {
		hash := dir.Name()
		fingerprint, err := hex.DecodeString(hash)
		if err != nil || len(fingerprint) != sha256.Size || hash != strings.ToLower(hash) {
			continue
		}
		if _, err := os.Lstat(filepath.Join(root, hash, "extracted.json")); os.IsNotExist(err) {
			if _, err := b.pendingStatement(hash); err != nil {
				return nil, fmt.Errorf("pending statement %s: %w", hash, err)
			}
			continue
		} else if err != nil {
			return nil, err
		}
		statement, err := b.cachedStatement(hash)
		if err != nil {
			return nil, fmt.Errorf("cached statement %s: %w", hash, err)
		}
		statements = append(statements, statement)
	}
	slices.SortStableFunc(statements, func(a, b *Statement) int {
		return strings.Compare(a.sortDate(), b.sortDate())
	})
	return statements, nil
}

func (b *Books) decodeStatement(data []byte, hash, dir string) (*Statement, error) {
	var raw struct {
		Period       *StatementPeriod `json:"wat_statement_period"`
		BankAccount  string           `json:"wat_bank_account"`
		Transactions []struct {
			Date        string `json:"booking_date"`
			Description string `json:"description"`
			Currency    string `json:"currency"`
			Amount      string `json:"amount"`
			AccountID   string `json:"account_id"`
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
	if raw.Period != nil {
		start, startErr := time.Parse("2006-01-02", raw.Period.Start)
		end, endErr := time.Parse("2006-01-02", raw.Period.End)
		if startErr != nil || endErr != nil || end.Before(start) {
			return nil, fmt.Errorf("statement period requires valid YYYY-MM-DD start/end dates in order")
		}
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
	extractedAccountID := ""
	for _, tx := range raw.Transactions {
		identifier := normalizeAccountIdentifier(tx.AccountID)
		if identifier == "" {
			return nil, fmt.Errorf("statement extraction did not identify its bank account")
		}
		if extractedAccountID != "" && extractedAccountID != identifier {
			return nil, fmt.Errorf("statement extraction returned inconsistent bank account identifiers")
		}
		extractedAccountID = identifier
	}
	bankAccount, err := b.statementBankAccount(raw.BankAccount, extractedAccountID)
	if err != nil {
		return nil, err
	}
	result := &Statement{
		Hash: hash, Source: filepath.ToSlash(source), BankAccount: bankAccount.Account, Period: raw.Period,
		OpeningCents: opening, ClosingCents: closing, Warnings: raw.Warnings,
	}
	total := big.NewInt(opening)
	for i, tx := range raw.Transactions {
		_, dateErr := time.Parse("2006-01-02", tx.Date)
		cents, amountErr := ParseCents(tx.Amount)
		if dateErr != nil {
			return nil, fmt.Errorf("statement row %d must have a valid YYYY-MM-DD date", i+1)
		}
		if raw.Period != nil && (tx.Date < raw.Period.Start || tx.Date > raw.Period.End) {
			return nil, fmt.Errorf("statement row %d is outside its statement period", i+1)
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

func normalizeAccountIdentifier(text string) string {
	var normalized strings.Builder
	for _, r := range strings.ToUpper(text) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			normalized.WriteRune(r)
		}
	}
	return normalized.String()
}

func (b *Books) statementBankAccount(bound, extractedIdentifier string) (BankAccountConfig, error) {
	var matches []BankAccountConfig
	for _, bank := range b.Config.BankAccounts {
		suffix := normalizeAccountIdentifier(bank.Account[strings.LastIndex(bank.Account, ":")+1:])
		if strings.HasSuffix(extractedIdentifier, suffix) {
			matches = append(matches, bank)
		}
	}
	if len(matches) != 1 {
		return BankAccountConfig{}, fmt.Errorf("statement account identifier must match exactly one configured bank account; matched %d", len(matches))
	}
	if bound != "" && bound != matches[0].Account {
		return BankAccountConfig{}, fmt.Errorf("statement cache is bound to bank account %q but its extracted identifier matches %q", bound, matches[0].Account)
	}
	return matches[0], nil
}

func bindStatementExtraction(data []byte, bankAccount string) ([]byte, error) {
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("decode statement extraction for account binding: %w", err)
	}
	if existing, ok := document["wat_bank_account"]; ok && existing != bankAccount {
		return nil, fmt.Errorf("statement extraction is already bound to a different bank account")
	}
	document["wat_bank_account"] = bankAccount
	bound, err := json.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("encode statement account binding: %w", err)
	}
	return append(bound, '\n'), nil
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
	if _, ok := b.Config.bankAccount(s.BankAccount); !ok {
		return nil, fmt.Errorf("statement bank account is no longer configured: %s", s.BankAccount)
	}
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
			amount, err := bankAmount(tx, s.BankAccount)
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
				if posting.Account != s.BankAccount {
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
	bank, ok := b.Config.bankAccount(s.BankAccount)
	if !ok {
		return nil, Entry{}, fmt.Errorf("statement bank account is no longer configured: %s", s.BankAccount)
	}
	draft, err := b.newDraft(bank.Journal)
	if err != nil {
		return nil, Entry{}, err
	}
	row := s.Rows[index]
	draft.statement = &statementDraft{s, index, s.BankAccount}
	entry := Entry{Date: row.Date, Payee: row.Description, Source: s.Source,
		BankRow: s.key(index), BankDescription: descriptionKey(row.Description),
		Postings: []Posting{{s.BankAccount, formatCents(row.Cents)}, {statuses[index].Suggestion, formatCents(-row.Cents)}}}
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
