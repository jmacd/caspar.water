package wat

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode"
)

//go:embed statement_payees.py
var statementPayeeParser string

const statementCheckPayeesFile = "check-payees.json"

type statementAISettings struct {
	Python, APIBase, APIVersion, Model, Provider string
}

type statementCheckPayee struct {
	Row        int     `json:"row"`
	Payee      string  `json:"payee"`
	Confidence float64 `json:"confidence"`
}

type statementCheckPayeeCache struct {
	StatementHash string                `json:"statement_hash"`
	Model         string                `json:"model"`
	CreatedAt     string                `json:"created_at"`
	Candidates    []int                 `json:"candidates"`
	Payees        []statementCheckPayee `json:"payees"`
}

type statementCheckPayeeRequest struct {
	Rows []statementCheckPayeeRequestRow `json:"rows"`
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
	return false
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
	request := statementCheckPayeeRequest{}
	for _, rowNumber := range candidates {
		row := statement.Rows[rowNumber-1]
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
