package wat

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

const quickBooksMappingVersion = 1

type quickBooksAccountMapping struct {
	Label   string `json:"label" yaml:"label"`
	Account string `json:"account" yaml:"account"`
}

type quickBooksMappingFile struct {
	Version  int                        `yaml:"version"`
	Mappings []quickBooksAccountMapping `yaml:"mappings"`
}

type quickBooksSourceLabel struct {
	Kind  string
	Label string
}

func (b *Books) quickBooksMappings() ([]quickBooksAccountMapping, error) {
	info, err := os.Lstat(b.Config.QuickBooksMappings)
	if os.IsNotExist(err) {
		return []quickBooksAccountMapping{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("inspect QuickBooks mappings: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf(
			"QuickBooks mappings must be a nonsymlink regular file: %s",
			b.Config.QuickBooksMappings,
		)
	}
	file, err := os.Open(b.Config.QuickBooksMappings)
	if err != nil {
		return nil, fmt.Errorf("open QuickBooks mappings: %w", err)
	}
	defer file.Close()
	decoder := yaml.NewDecoder(file)
	decoder.KnownFields(true)
	var document quickBooksMappingFile
	if err := decoder.Decode(&document); err != nil {
		return nil, fmt.Errorf("decode QuickBooks mappings: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("QuickBooks mappings must contain exactly one YAML document")
	}
	if document.Version != quickBooksMappingVersion {
		return nil, fmt.Errorf(
			"unsupported QuickBooks mapping version %d (expected %d)",
			document.Version,
			quickBooksMappingVersion,
		)
	}
	if document.Mappings == nil {
		return nil, fmt.Errorf("QuickBooks mappings requires a mappings array")
	}
	if err := validateQuickBooksMappingDocument(document.Mappings); err != nil {
		return nil, err
	}
	slices.SortStableFunc(document.Mappings, compareQuickBooksMappings)
	return document.Mappings, nil
}

func (b *Books) saveQuickBooksMappings(mappings []quickBooksAccountMapping) error {
	mappings = append([]quickBooksAccountMapping(nil), mappings...)
	if mappings == nil {
		mappings = []quickBooksAccountMapping{}
	}
	if err := b.validateQuickBooksMappings(mappings); err != nil {
		return err
	}
	slices.SortStableFunc(mappings, compareQuickBooksMappings)
	var data bytes.Buffer
	data.WriteString("# Managed by wat quickbooks --review.\n")
	encoder := yaml.NewEncoder(&data)
	encoder.SetIndent(2)
	if err := encoder.Encode(quickBooksMappingFile{
		Version: quickBooksMappingVersion, Mappings: mappings,
	}); err != nil {
		return fmt.Errorf("encode QuickBooks mappings: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close QuickBooks mapping encoder: %w", err)
	}
	if err := writeGeneratedFile(b.Config.QuickBooksMappings, data.Bytes()); err != nil {
		return fmt.Errorf("save QuickBooks mappings: %w", err)
	}
	return nil
}

func (b *Books) validateQuickBooksMappings(mappings []quickBooksAccountMapping) error {
	if err := validateQuickBooksMappingDocument(mappings); err != nil {
		return err
	}
	postingAccounts, err := b.PostingAccounts()
	if err != nil {
		return err
	}
	configured := make(map[string]bool, len(postingAccounts))
	for _, account := range postingAccounts {
		configured[account.Name] = true
	}
	for _, mapping := range mappings {
		if !configured[mapping.Account] {
			return fmt.Errorf(
				"QuickBooks label %q maps to unconfigured account %q",
				mapping.Label,
				mapping.Account,
			)
		}
	}
	return nil
}

func validateQuickBooksMappingDocument(mappings []quickBooksAccountMapping) error {
	labels := make(map[string]bool, len(mappings))
	for index, mapping := range mappings {
		if strings.TrimSpace(mapping.Label) != mapping.Label ||
			mapping.Label == "" || !singleLine(mapping.Label) {
			return fmt.Errorf("QuickBooks mapping %d has an invalid label", index+1)
		}
		if labels[mapping.Label] {
			return fmt.Errorf("QuickBooks label %q is mapped more than once", mapping.Label)
		}
		labels[mapping.Label] = true
		if strings.TrimSpace(mapping.Account) != mapping.Account ||
			mapping.Account == "" || !singleLine(mapping.Account) {
			return fmt.Errorf(
				"QuickBooks mapping for label %q has an invalid account",
				mapping.Label,
			)
		}
	}
	return nil
}

func compareQuickBooksMappings(a, b quickBooksAccountMapping) int {
	if order := strings.Compare(a.Label, b.Label); order != 0 {
		return order
	}
	return strings.Compare(a.Account, b.Account)
}

func quickBooksRecordSourceLabels(record quickBooksRecord) []quickBooksSourceLabel {
	var labels []quickBooksSourceLabel
	add := func(kind, value string) {
		value = strings.Join(strings.Fields(value), " ")
		if value != "" {
			labels = append(labels, quickBooksSourceLabel{Kind: kind, Label: value})
		}
	}
	add("report section", record.Section)
	for name, value := range record.Fields {
		switch normalizeQuickBooksHeader(name) {
		case "account", "account name", "distribution account":
			add(normalizeQuickBooksHeader(name), value)
		}
	}
	slices.SortStableFunc(labels, func(a, b quickBooksSourceLabel) int {
		if order := strings.Compare(a.Label, b.Label); order != 0 {
			return order
		}
		return strings.Compare(a.Kind, b.Kind)
	})
	return labels
}

func quickBooksRecordMappingLabel(record quickBooksRecord) (quickBooksSourceLabel, bool) {
	if label := strings.Join(strings.Fields(record.Section), " "); label != "" {
		return quickBooksSourceLabel{Kind: "report section", Label: label}, true
	}
	for _, field := range []struct {
		kind string
		name string
	}{
		{kind: "account", name: "account"},
		{kind: "distribution account", name: "distribution account"},
		{kind: "account name", name: "account name"},
	} {
		for name, value := range record.Fields {
			label := strings.Join(strings.Fields(value), " ")
			if normalizeQuickBooksHeader(name) == field.name && label != "" {
				return quickBooksSourceLabel{
					Kind: field.kind, Label: label,
				}, true
			}
		}
	}
	return quickBooksSourceLabel{}, false
}

func quickBooksMappedAccounts(
	group quickBooksTransactionGroup,
	mappings []quickBooksAccountMapping,
) (map[string]bool, error) {
	byLabel := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		byLabel[mapping.Label] = mapping.Account
	}
	required := make(map[string]bool)
	for _, record := range group.Records {
		sourceLabel, exists := quickBooksRecordMappingLabel(record)
		if !exists {
			continue
		}
		if account := byLabel[sourceLabel.Label]; account != "" {
			required[account] = true
		}
	}
	return required, nil
}
