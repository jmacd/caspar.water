package wat

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type statementRegisterDefault struct {
	Matches      int
	Payee        string
	Description  string
	Reference    string
	AccountLabel string
	Account      string
}

func (d statementRegisterDefault) exact() bool {
	return d.Matches == 1
}

func (d statementRegisterDefault) hint() string {
	switch {
	case d.Matches == 0:
		return ""
	case d.Matches > 1:
		return fmt.Sprintf("QuickBooks: %d exact date/amount matches; no defaults", d.Matches)
	case d.Account != "":
		return fmt.Sprintf("QuickBooks: exact match | %s -> %s", d.AccountLabel, d.Account)
	case d.AccountLabel != "":
		return fmt.Sprintf("QuickBooks: exact match | account %q is unmapped", d.AccountLabel)
	default:
		return "QuickBooks: exact match | no account label"
	}
}

func (b *Books) statementRegisterDefault(row StatementRow) (statementRegisterDefault, error) {
	if b.Config.Statement.QuickBooksRegister == "" {
		return statementRegisterDefault{}, nil
	}
	year, err := strconv.Atoi(row.Date[:4])
	if err != nil {
		return statementRegisterDefault{}, fmt.Errorf("statement row has invalid year: %s", row.Date)
	}
	records, err := b.statementRegisterRecords(year)
	if err != nil {
		return statementRegisterDefault{}, err
	}
	var matches []quickBooksRecord
	for _, record := range records {
		cents, ok, err := quickBooksRegisterAmount(record)
		if err != nil {
			return statementRegisterDefault{}, err
		}
		if ok && record.Date == row.Date && cents == row.Cents {
			matches = append(matches, record)
		}
	}
	matches = strengthenStatementRegisterMatches(row, matches)
	result := statementRegisterDefault{Matches: len(matches)}
	if len(matches) != 1 {
		return result, nil
	}
	record := matches[0]
	result.Payee = quickBooksField(record.Fields, "name", "payee")
	result.Description = quickBooksField(record.Fields, "description", "memo")
	result.Reference = quickBooksField(
		record.Fields,
		"num",
		"number",
		"ref no",
		"ref no.",
		"reference",
	)
	if label, ok := quickBooksRecordMappingLabel(record); ok {
		result.AccountLabel = label.Label
		mappings, err := b.quickBooksMappings()
		if err != nil {
			return statementRegisterDefault{}, err
		}
		if err := b.validateQuickBooksMappings(mappings); err != nil {
			return statementRegisterDefault{}, err
		}
		for _, mapping := range mappings {
			if mapping.Label == label.Label {
				result.Account = mapping.Account
				break
			}
		}
	}
	return result, nil
}

func (b *Books) statementRegisterRecords(year int) ([]quickBooksRecord, error) {
	if records, ok := b.quickBooksRegisterYear[year]; ok {
		return records, nil
	}
	path := b.Config.Statement.QuickBooksRegister
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.Config.root, path)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	data, err := quickBooksInputCSV(ctx, path, io.Discard)
	if err != nil {
		return nil, fmt.Errorf("load statement QuickBooks register: %w", err)
	}
	parsed, err := parseQuickBooksCSVRows(filepath.Base(path), data, year)
	if err != nil {
		return nil, fmt.Errorf("load statement QuickBooks register: %w", err)
	}
	records := parsed.Records
	b.quickBooksRegisterYear[year] = records
	return records, nil
}

func quickBooksRegisterAmount(record quickBooksRecord) (int64, bool, error) {
	payment := quickBooksField(record.Fields, "payment")
	deposit := quickBooksField(record.Fields, "deposit")
	if payment != "" && deposit != "" {
		return 0, false, fmt.Errorf("%s has both payment and deposit amounts", record.ID)
	}
	value, sign := payment, int64(-1)
	if value == "" {
		value, sign = deposit, 1
	}
	if value == "" {
		return 0, false, nil
	}
	cents, err := parseQuickBooksCents(value)
	if err != nil {
		return 0, false, fmt.Errorf("%s bank amount: %w", record.ID, err)
	}
	if cents < 0 {
		cents = -cents
	}
	return sign * cents, cents != 0, nil
}

func quickBooksField(fields map[string]string, names ...string) string {
	for name, value := range fields {
		key := strings.TrimSuffix(normalizeQuickBooksHeader(name), ".")
		for _, candidate := range names {
			if key == strings.TrimSuffix(candidate, ".") {
				return strings.Join(strings.Fields(value), " ")
			}
		}
	}
	return ""
}

func strengthenStatementRegisterMatches(row StatementRow, matches []quickBooksRecord) []quickBooksRecord {
	if len(matches) <= 1 {
		return matches
	}
	var referenced []quickBooksRecord
	descriptionFields := strings.Fields(strings.ToLower(row.Description))
	for _, record := range matches {
		reference := strings.ToLower(quickBooksField(
			record.Fields,
			"num",
			"number",
			"ref no",
			"ref no.",
			"reference",
		))
		for _, field := range descriptionFields {
			if reference != "" && field == reference {
				referenced = append(referenced, record)
				break
			}
		}
	}
	if len(referenced) != 0 {
		return referenced
	}
	var described []quickBooksRecord
	source := descriptionKey(row.Description)
	for _, record := range matches {
		for _, candidate := range []string{
			quickBooksField(record.Fields, "description", "memo"),
			quickBooksField(record.Fields, "name", "payee"),
		} {
			if candidate != "" && descriptionKey(candidate) == source {
				described = append(described, record)
				break
			}
		}
	}
	if len(described) != 0 {
		return described
	}
	return matches
}
