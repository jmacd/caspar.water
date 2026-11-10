package wat

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

//go:embed statement_payees.py
var statementPayeeParser string

const statementCheckPayeesFile = "check-payees.json"

var statementCheckMetadata = regexp.MustCompile(
	`^[0-9]{3,10}[[:space:]]+([0-9]{2}/[0-9]{2}/[0-9]{4})[[:space:]]+\$([0-9][0-9,]*\.[0-9]{2})$`,
)

var statementSuggestedAmount = regexp.MustCompile(`^[0-9]+\.[0-9]{2}$`)

type statementAISettings struct {
	Python, APIBase, APIVersion, Model, Provider string
}

type statementCheckPayee struct {
	Row        int                         `json:"row"`
	Payee      string                      `json:"payee"`
	Confidence float64                     `json:"confidence"`
	Memo       string                      `json:"memo,omitempty"`
	Postings   []statementSuggestedPosting `json:"postings,omitempty"`
}

type statementSuggestedPosting struct {
	Account string `json:"account"`
	Amount  string `json:"amount"`
}

type statementCheckPayeeCache struct {
	Version            int                          `json:"version"`
	StatementHash      string                       `json:"statement_hash"`
	Model              string                       `json:"model"`
	CreatedAt          string                       `json:"created_at"`
	Candidates         []int                        `json:"candidates"`
	DeclaredPayees     []string                     `json:"declared_payees,omitempty"`
	DeclaredAccounts   []string                     `json:"declared_accounts,omitempty"`
	TransactionPrompts []StatementTransactionPrompt `json:"transaction_prompts,omitempty"`
	Payees             []statementCheckPayee        `json:"payees"`
}

type statementCheckPayeeRequest struct {
	Rows               []statementCheckPayeeRequestRow `json:"rows"`
	DeclaredPayees     []string                        `json:"declared_payees"`
	DeclaredAccounts   []string                        `json:"declared_accounts"`
	TransactionPrompts []StatementTransactionPrompt    `json:"transaction_prompts"`
}

type statementCheckPayeeRequestRow struct {
	Row         int    `json:"row"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Amount      string `json:"amount"`
}

func (b *Books) statementAISettings() (statementAISettings, error) {
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
		return statementAISettings{}, fmt.Errorf(
			"configure statement.python (or WAT_STATEMENT_PYTHON), statement.api_base " +
				"and a provider-prefixed statement.model; install " +
				"cmd/wat/requirements-statement.txt in that Python environment",
		)
	}
	if provider != "ollama" && provider != "azure" {
		return statementAISettings{}, fmt.Errorf("statement.model provider must be ollama or azure")
	}
	if provider == "azure" && apiVersion == "" {
		return statementAISettings{}, fmt.Errorf("configure statement.api_version for Azure")
	}
	if strings.ContainsAny(python, `/\`) && !filepath.IsAbs(python) {
		python = filepath.Join(b.Config.root, python)
	}
	return statementAISettings{
		Python: python, APIBase: apiBase, APIVersion: apiVersion, Model: model, Provider: provider,
	}, nil
}

func statementRowIsCheck(row StatementRow) bool {
	if row.Cents >= 0 {
		return false
	}
	for _, field := range strings.FieldsFunc(strings.ToUpper(row.Description), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	}) {
		if field == "CHECK" {
			return true
		}
	}
	match := statementCheckMetadata.FindStringSubmatch(strings.TrimSpace(row.Description))
	if match == nil {
		return false
	}
	date, err := time.Parse("01/02/2006", match[1])
	if err != nil || date.Format("2006-01-02") != row.Date {
		return false
	}
	cents, err := ParseCents(strings.ReplaceAll(match[2], ",", ""))
	return err == nil && cents == -row.Cents
}

func statementCheckPayeeCandidates(statement *Statement) []int {
	var candidates []int
	for index, row := range statement.Rows {
		if statementRowIsCheck(row) {
			candidates = append(candidates, index+1)
		}
	}
	return candidates
}

func decodeStatementCheckPayees(data []byte, statement *Statement) (*statementCheckPayeeCache, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cache statementCheckPayeeCache
	if err := decoder.Decode(&cache); err != nil {
		return nil, fmt.Errorf("decode check payee extraction: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("decode check payee extraction: expected one JSON object")
	}
	if cache.Version != 0 && cache.Version != 1 {
		return nil, fmt.Errorf("check payee extraction has unsupported version %d", cache.Version)
	}
	if cache.StatementHash != statement.Hash {
		return nil, fmt.Errorf("check payee extraction does not match its statement fingerprint")
	}
	if strings.TrimSpace(cache.Model) == "" || !singleLine(cache.Model) {
		return nil, fmt.Errorf("check payee extraction has an invalid model")
	}
	if _, err := time.Parse(time.RFC3339, cache.CreatedAt); err != nil {
		return nil, fmt.Errorf("check payee extraction has an invalid created_at time")
	}
	expected := statementCheckPayeeCandidates(statement)
	if !slices.Equal(cache.Candidates, expected) {
		return nil, fmt.Errorf("check payee extraction does not cover the statement's check rows")
	}
	candidates := make(map[int]bool, len(cache.Candidates))
	for _, row := range cache.Candidates {
		if candidates[row] {
			return nil, fmt.Errorf("check payee extraction has duplicate candidate rows")
		}
		candidates[row] = true
	}
	seen := make(map[int]bool, len(cache.Payees))
	declared := make(map[string]bool, len(cache.DeclaredPayees))
	for index, payee := range cache.DeclaredPayees {
		if payee == "" ||
			strings.TrimSpace(payee) != payee ||
			!singleLine(payee) ||
			strings.ContainsAny(payee, ";|") {
			return nil, fmt.Errorf("check payee extraction declared payee %d is invalid", index+1)
		}
		if declared[payee] {
			return nil, fmt.Errorf("check payee extraction declared payee %q is duplicated", payee)
		}
		declared[payee] = true
	}
	if !slices.IsSorted(cache.DeclaredPayees) {
		return nil, fmt.Errorf("check payee extraction declared payees are not sorted")
	}
	accounts := make(map[string]bool, len(cache.DeclaredAccounts))
	for index, account := range cache.DeclaredAccounts {
		parsed, ok := parsePostingAccount(account)
		if !ok || parsed.Category == "Bank" {
			return nil, fmt.Errorf("check payee extraction declared account %d is invalid", index+1)
		}
		if accounts[account] {
			return nil, fmt.Errorf("check payee extraction declared account %q is duplicated", account)
		}
		accounts[account] = true
	}
	if !slices.IsSorted(cache.DeclaredAccounts) {
		return nil, fmt.Errorf("check payee extraction declared accounts are not sorted")
	}
	rules := make(map[string]bool, len(cache.TransactionPrompts))
	for index, rule := range cache.TransactionPrompts {
		if rule.Payee == "" ||
			strings.TrimSpace(rule.Payee) != rule.Payee ||
			!singleLine(rule.Payee) ||
			strings.ContainsAny(rule.Payee, ";|") ||
			!validStatementTransactionPrompt(rule.Prompt) ||
			strings.TrimSpace(rule.Prompt) != rule.Prompt {
			return nil, fmt.Errorf("check payee extraction transaction prompt %d is invalid", index+1)
		}
		if !declared[rule.Payee] {
			return nil, fmt.Errorf(
				"check payee extraction transaction prompt payee %q is undeclared",
				rule.Payee,
			)
		}
		if rules[rule.Payee] {
			return nil, fmt.Errorf("check payee extraction transaction prompt payee %q is duplicated", rule.Payee)
		}
		rules[rule.Payee] = true
	}
	for _, suggestion := range cache.Payees {
		if !candidates[suggestion.Row] || seen[suggestion.Row] {
			return nil, fmt.Errorf("check payee extraction has an invalid or duplicate suggested row")
		}
		if suggestion.Payee == "" ||
			strings.TrimSpace(suggestion.Payee) != suggestion.Payee ||
			!singleLine(suggestion.Payee) ||
			strings.ContainsAny(suggestion.Payee, ";|") {
			return nil, fmt.Errorf("check payee extraction row %d has an invalid payee", suggestion.Row)
		}
		if suggestion.Confidence < 0 || suggestion.Confidence > 1 {
			return nil, fmt.Errorf("check payee extraction row %d has an invalid confidence", suggestion.Row)
		}
		if strings.TrimSpace(suggestion.Memo) != suggestion.Memo ||
			!singleLine(suggestion.Memo) ||
			strings.ContainsAny(suggestion.Memo, ";|") {
			return nil, fmt.Errorf("check payee extraction row %d has an invalid memo", suggestion.Row)
		}
		if (suggestion.Memo != "" || len(suggestion.Postings) != 0) && !rules[suggestion.Payee] {
			return nil, fmt.Errorf(
				"check payee extraction row %d applied instructions for an unmatched payee",
				suggestion.Row,
			)
		}
		postingAccounts := make(map[string]bool, len(suggestion.Postings))
		var total int64
		for postingIndex, posting := range suggestion.Postings {
			if !accounts[posting.Account] {
				return nil, fmt.Errorf(
					"check payee extraction row %d posting %d uses undeclared account %q",
					suggestion.Row,
					postingIndex+1,
					posting.Account,
				)
			}
			if postingAccounts[posting.Account] {
				return nil, fmt.Errorf(
					"check payee extraction row %d uses account %q more than once",
					suggestion.Row,
					posting.Account,
				)
			}
			cents, err := ParseCents(posting.Amount)
			if !statementSuggestedAmount.MatchString(posting.Amount) ||
				err != nil ||
				cents <= 0 ||
				total > math.MaxInt64-cents {
				return nil, fmt.Errorf(
					"check payee extraction row %d posting %d has an invalid amount",
					suggestion.Row,
					postingIndex+1,
				)
			}
			postingAccounts[posting.Account] = true
			total += cents
		}
		if statement.Rows[suggestion.Row-1].Cents == math.MinInt64 {
			return nil, fmt.Errorf(
				"check payee extraction row %d bank amount exceeds supported range",
				suggestion.Row,
			)
		}
		if len(suggestion.Postings) != 0 && total != -statement.Rows[suggestion.Row-1].Cents {
			return nil, fmt.Errorf(
				"check payee extraction row %d split postings do not balance to the bank amount",
				suggestion.Row,
			)
		}
		seen[suggestion.Row] = true
	}
	return &cache, nil
}

func applyStatementCheckPayees(statement *Statement, cache *statementCheckPayeeCache) {
	statement.checkPayees = make(map[int]statementCheckPayee, len(cache.Payees))
	for _, suggestion := range cache.Payees {
		statement.checkPayees[suggestion.Row-1] = suggestion
	}
	statement.checkPayeesLoaded = true
}

func (b *Books) loadStatementCheckPayees(statement *Statement, dir string) error {
	path := filepath.Join(dir, statementCheckPayeesFile)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("statement check payee cache is not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read check payee extraction: %w", err)
	}
	cache, err := decodeStatementCheckPayees(data, statement)
	if err != nil {
		return err
	}
	if cache.Version == 0 ||
		!slices.Equal(cache.TransactionPrompts, b.Config.Statement.TransactionPrompts) {
		return nil
	}
	applyStatementCheckPayees(statement, cache)
	return nil
}

func (b *Books) extractStatementCheckPayees(
	path string,
	statement *Statement,
	progress io.Writer,
) ([]byte, error) {
	candidates := statementCheckPayeeCandidates(statement)
	if len(candidates) == 0 {
		return nil, nil
	}
	settings, err := b.statementAISettings()
	if err != nil {
		return nil, fmt.Errorf("check payee extraction: %w", err)
	}
	declaredPayees, err := b.declaredPayees()
	if err != nil {
		return nil, fmt.Errorf("load declared payees for check image context: %w", err)
	}
	if declaredPayees == nil {
		declaredPayees = []string{}
	}
	for index, rule := range b.Config.Statement.TransactionPrompts {
		if !slices.Contains(declaredPayees, rule.Payee) {
			return nil, fmt.Errorf(
				"statement.transaction_prompts[%d] payee %q is not declared in books/payees.journal",
				index,
				rule.Payee,
			)
		}
	}
	postingAccounts, err := b.PostingAccounts()
	if err != nil {
		return nil, fmt.Errorf("load declared accounts for transaction prompt context: %w", err)
	}
	declaredAccounts := []string{}
	for _, account := range postingAccounts {
		if account.Category != "Bank" {
			declaredAccounts = append(declaredAccounts, account.Name)
		}
	}
	request := statementCheckPayeeRequest{
		DeclaredPayees:     declaredPayees,
		DeclaredAccounts:   declaredAccounts,
		TransactionPrompts: append([]StatementTransactionPrompt{}, b.Config.Statement.TransactionPrompts...),
	}
	for _, rowNumber := range candidates {
		row := statement.Rows[rowNumber-1]
		if row.Cents == math.MinInt64 {
			return nil, fmt.Errorf(
				"statement row %d amount exceeds supported check-image context",
				rowNumber,
			)
		}
		request.Rows = append(request.Rows, statementCheckPayeeRequestRow{
			Row: rowNumber, Date: row.Date, Description: row.Description, Amount: formatCents(row.Cents),
		})
	}
	input, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(
		ctx,
		settings.Python,
		"-c",
		statementPayeeParser,
		path,
		settings.APIBase,
		settings.APIVersion,
		settings.Model,
		statement.Hash,
	)
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stderr = progress
	fmt.Fprintf(
		progress,
		"Reading check payees with configured %s model %s; sending statement page images to %s. No journal entries will be written.\n",
		settings.Provider,
		settings.Model,
		settings.APIBase,
	)
	data, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("check payee extraction: %w", ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("check payee extraction failed: %w", err)
	}
	cache, err := decodeStatementCheckPayees(data, statement)
	if err != nil {
		return nil, err
	}
	if cache.Version != 1 {
		return nil, fmt.Errorf("check payee extraction did not return version 1")
	}
	if !slices.Equal(cache.DeclaredPayees, declaredPayees) {
		return nil, fmt.Errorf("check payee extraction did not preserve its declared payee context")
	}
	if !slices.Equal(cache.DeclaredAccounts, declaredAccounts) {
		return nil, fmt.Errorf("check payee extraction did not preserve its declared account context")
	}
	if !slices.Equal(cache.TransactionPrompts, b.Config.Statement.TransactionPrompts) {
		return nil, fmt.Errorf("check payee extraction did not preserve its transaction prompts")
	}
	applyStatementCheckPayees(statement, cache)
	return data, nil
}

func publishStatementCheckPayees(dir string, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	temp, err := os.CreateTemp(dir, ".check-payees-")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	if err := temp.Close(); err != nil {
		return err
	}
	defer os.Remove(tempName)
	if err := writeSynced(tempName, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tempName, filepath.Join(dir, statementCheckPayeesFile)); err != nil {
		return fmt.Errorf("publish check payee extraction: %w", err)
	}
	cacheDir, err := os.Open(dir)
	if err != nil {
		return err
	}
	if err := errors.Join(cacheDir.Sync(), cacheDir.Close()); err != nil {
		return fmt.Errorf("check payee extraction saved but directory sync failed: %w", err)
	}
	return nil
}

func (b *Books) EnsureStatementCheckPayees(statement *Statement, progress io.Writer) error {
	if progress == nil {
		progress = io.Discard
	}
	if statement.checkPayeesLoaded || len(statementCheckPayeeCandidates(statement)) == 0 {
		return nil
	}
	path := filepath.Join(b.Config.root, filepath.FromSlash(statement.Source))
	data, err := b.extractStatementCheckPayees(path, statement, progress)
	if err != nil {
		return err
	}
	return publishStatementCheckPayees(filepath.Dir(path), data)
}
