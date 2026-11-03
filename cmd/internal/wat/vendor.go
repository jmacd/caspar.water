package wat

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strings"
)

func (b *Books) checkRegisteredVendors(transactions []journalTransaction) error {
	if len(b.Config.Vendors) == 0 {
		return nil
	}
	vendors := make(map[string]string)
	for _, path := range b.Config.Vendors {
		fileVendors, err := readVendors(path)
		if err != nil {
			return fmt.Errorf("load vendor file %s: %w", path, err)
		}
		for name := range fileVendors {
			if first, exists := vendors[name]; exists {
				return fmt.Errorf(
					"vendor %q is defined in both %s and %s",
					name,
					first,
					path,
				)
			}
			vendors[name] = path
		}
	}
	for _, tx := range transactions {
		payee, err := transactionPayee(tx.Description)
		if err != nil {
			return fmt.Errorf("%s %s: %w", tx.Date, tx.Description, err)
		}
		hasExpense := transactionHasExpense(tx)
		switch payee {
		case "Journal":
		case "Deposit":
			if hasExpense {
				return fmt.Errorf(
					"%s %s: Deposit transaction must not contain Expense postings",
					tx.Date,
					tx.Description,
				)
			}
			if !transactionHasBank(tx) {
				return fmt.Errorf(
					"%s %s: Deposit transaction requires a Bank posting",
					tx.Date,
					tx.Description,
				)
			}
		default:
			if _, registered := vendors[payee]; registered {
				continue
			}
			return fmt.Errorf(
				"%s %s: transaction payee %q is not registered in the configured vendor files; "+
					"use a canonical vendor name, Journal, or Deposit",
				tx.Date,
				tx.Description,
				payee,
			)
		}
	}
	return nil
}

func transactionPayee(description string) (string, error) {
	if strings.Count(description, "|") != 1 {
		return "", fmt.Errorf(`transaction description must contain exactly one " | " separator`)
	}
	payee, memo, found := strings.Cut(description, " | ")
	if !found ||
		payee == "" ||
		memo == "" ||
		strings.TrimSpace(payee) != payee ||
		strings.TrimSpace(memo) != memo {
		return "", fmt.Errorf(`transaction description must use "PAYEE | MEMO" with nonempty fields`)
	}
	return payee, nil
}

func transactionHasExpense(tx journalTransaction) bool {
	for _, posting := range tx.Postings {
		if strings.HasPrefix(posting.Account, "Expense:") {
			return true
		}
	}
	return false
}

func transactionHasBank(tx journalTransaction) bool {
	for _, posting := range tx.Postings {
		if strings.HasPrefix(posting.Account, "Bank:") {
			return true
		}
	}
	return false
}

func readVendors(path string) (map[string]bool, error) {
	state, err := readNonsymlinkFileState(path, "vendors file")
	if err != nil {
		return nil, err
	}
	reader := csv.NewReader(bytes.NewReader(state.data))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("decode vendors CSV: %w", err)
	}
	if len(rows) == 0 || len(rows[0]) != 1 ||
		strings.TrimPrefix(rows[0][0], "\ufeff") != "name" {
		return nil, fmt.Errorf("vendors CSV header must be exactly: name")
	}
	vendors := make(map[string]bool, len(rows)-1)
	for index, row := range rows[1:] {
		line := index + 2
		if len(row) != 1 {
			return nil, fmt.Errorf("vendors CSV row %d must contain exactly one name", line)
		}
		name := row[0]
		if strings.TrimSpace(name) != name || name == "" || !singleLine(name) ||
			strings.ContainsAny(name, ";|") {
			return nil, fmt.Errorf("vendors CSV row %d has an invalid canonical name", line)
		}
		if name == "Journal" || name == "Deposit" {
			return nil, fmt.Errorf(
				"vendors CSV row %d uses reserved transaction payee %q",
				line,
				name,
			)
		}
		if vendors[name] {
			return nil, fmt.Errorf("vendors CSV row %d duplicates vendor %q", line, name)
		}
		vendors[name] = true
	}
	if len(vendors) == 0 {
		return nil, fmt.Errorf("vendors CSV must contain at least one vendor")
	}
	return vendors, nil
}
