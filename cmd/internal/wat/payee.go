package wat

import (
	"fmt"
	"strings"
)

func checkTransactionDescriptions(transactions []journalTransaction) error {
	for _, tx := range transactions {
		// In hledger, the first pipe separates an optional memo from the payee.
		payee, _, _ := strings.Cut(tx.Description, "|")
		if strings.TrimSpace(payee) != "Deposit" {
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
