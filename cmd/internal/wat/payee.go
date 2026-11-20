package wat

import (
	"fmt"
	"strings"
)

func checkTransactionDescriptions(transactions []journalTransaction) error {
	for _, tx := range transactions {
		payee, err := transactionPayee(tx.Description)
		if err != nil {
			return fmt.Errorf("%s %s: %w", tx.Date, tx.Description, err)
		}
		if payee != "Deposit" {
			continue
		}
		if transactionHasExpense(tx) {
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
