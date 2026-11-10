package wat

import (
	"crypto/sha256"
	"fmt"
	"slices"
	"strings"
	"time"
)

type quickBooksTransactionGroup struct {
	ID                      string             `json:"id"`
	Date                    string             `json:"date"`
	MatchBasis              string             `json:"match_basis"`
	Ambiguous               bool               `json:"ambiguous"`
	SourceAmountsMayOverlap bool               `json:"source_amounts_may_overlap"`
	Records                 []quickBooksRecord `json:"records"`
}

func quickBooksGroupSourceIDs(group quickBooksTransactionGroup) []string {
	ids := make([]string, len(group.Records))
	for index, record := range group.Records {
		ids[index] = record.ID
	}
	return ids
}

func groupQuickBooksRecords(records []quickBooksRecord) ([]quickBooksTransactionGroup, error) {
	buckets := make(map[string][]quickBooksRecord)
	recordIDs := make(map[string]bool, len(records))
	for _, record := range records {
		if record.ID == "" {
			return nil, fmt.Errorf("QuickBooks source record has no id")
		}
		if recordIDs[record.ID] {
			return nil, fmt.Errorf("duplicate QuickBooks source id %q", record.ID)
		}
		recordIDs[record.ID] = true
		if _, err := time.Parse("2006-01-02", record.Date); err != nil {
			return nil, fmt.Errorf(
				"QuickBooks source %q has invalid normalized date %q",
				record.ID,
				record.Date,
			)
		}
		key, err := quickBooksGroupingKey(record)
		if err != nil {
			return nil, err
		}
		if key == "" {
			key = "single\x00" + record.ID
		}
		buckets[key] = append(buckets[key], record)
	}

	groups := make([]quickBooksTransactionGroup, 0, len(buckets))
	ids := make(map[string]bool, len(buckets))
	for key, groupedRecords := range buckets {
		slices.SortStableFunc(groupedRecords, func(a, b quickBooksRecord) int {
			return strings.Compare(a.ID, b.ID)
		})
		ambiguous := false
		sources := make(map[string]bool, len(groupedRecords))
		for _, record := range groupedRecords {
			if sources[record.Source] {
				ambiguous = true
			}
			sources[record.Source] = true
		}
		basis := strings.SplitN(key, "\x00", 2)[0]
		if len(groupedRecords) == 1 {
			basis = "single"
		}
		idInput := make([]string, len(groupedRecords))
		for index, record := range groupedRecords {
			idInput[index] = record.ID
		}
		sum := sha256.Sum256([]byte(strings.Join(idInput, "\x00")))
		id := fmt.Sprintf("qb-%x", sum[:12])
		if ids[id] {
			return nil, fmt.Errorf("QuickBooks transaction group id collision %q", id)
		}
		ids[id] = true
		groups = append(groups, quickBooksTransactionGroup{
			ID:                      id,
			Date:                    groupedRecords[0].Date,
			MatchBasis:              basis,
			Ambiguous:               ambiguous,
			SourceAmountsMayOverlap: quickBooksSourceAmountsMayOverlap(groupedRecords),
			Records:                 groupedRecords,
		})
	}
	slices.SortStableFunc(groups, func(a, b quickBooksTransactionGroup) int {
		if order := strings.Compare(a.Date, b.Date); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return groups, nil
}

func quickBooksSourceAmountsMayOverlap(records []quickBooksRecord) bool {
	nonCashSources := make(map[string]bool)
	for _, record := range records {
		if record.CashSource {
			continue
		}
		nonCashSources[record.Source] = true
	}
	return len(nonCashSources) > 1
}

func quickBooksGroupingKey(record quickBooksRecord) (string, error) {
	amounts, err := quickBooksSourceAmounts([]quickBooksRecord{record})
	if err != nil {
		return "", err
	}
	if len(amounts) != 1 {
		return "", nil
	}
	var amount int64
	for value := range amounts {
		amount = value
	}
	reference := normalizedQuickBooksField(
		record.Fields,
		"num",
		"number",
		"ref no",
		"ref no.",
		"reference",
	)
	payee := normalizedQuickBooksField(record.Fields, "name", "payee")
	description := normalizedQuickBooksField(record.Fields, "description", "memo")
	transactionType := normalizedQuickBooksField(
		record.Fields,
		"transaction type",
		"type",
	)
	if reference != "" && transactionType != "" {
		return fmt.Sprintf(
			"reference\x00%s\x00%s\x00%s",
			record.Date,
			transactionType,
			reference,
		), nil
	}
	if reference != "" {
		return fmt.Sprintf(
			"reference\x00%s\x00%d\x00%s\x00%s",
			record.Date,
			amount,
			reference,
			payee,
		), nil
	}
	if payee != "" && description != "" && transactionType != "" {
		return fmt.Sprintf(
			"description\x00%s\x00%d\x00%s\x00%s\x00%s",
			record.Date,
			amount,
			transactionType,
			payee,
			description,
		), nil
	}
	return "", nil
}

func normalizedQuickBooksField(fields map[string]string, names ...string) string {
	for name, value := range fields {
		key := strings.TrimSuffix(normalizeQuickBooksHeader(name), ".")
		for _, candidate := range names {
			if key == strings.TrimSuffix(candidate, ".") {
				return strings.ToLower(strings.Join(strings.Fields(value), " "))
			}
		}
	}
	return ""
}
