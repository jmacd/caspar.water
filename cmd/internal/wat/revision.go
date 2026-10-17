package wat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

type JournalEntry struct {
	ID, Date, Description string
	ReadOnly              string
	BankChange            int64
	BankBalance           int64
}

type entryRevision struct {
	start, end int
	source     string
}

func transactionID(tx journalTransaction) (string, error) {
	id := ""
	count := 0
	for _, tag := range tx.Tags {
		if tag[0] == "id" {
			count++
			id = tag[1]
		}
	}
	if count != 1 || strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("entry needs exactly one nonempty id tag before it can be revised")
	}
	return id, nil
}

func (b *Books) transactionFile(tx journalTransaction) string {
	if len(tx.Source) != 2 {
		return ""
	}
	path := tx.Source[0].Name
	if !filepath.IsAbs(path) {
		path = filepath.Join(b.Config.root, path)
	}
	return filepath.Clean(path)
}

func (b *Books) JournalEntries() ([]JournalEntry, error) {
	transactions, err := b.transactions(b.Config.Journal)
	if err != nil {
		return nil, err
	}
	locked, err := b.lockedTransactions()
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(transactions, func(a, b journalTransaction) int {
		return strings.Compare(a.Date, b.Date)
	})
	counts := make(map[string]int)
	for _, tx := range transactions {
		for _, tag := range tx.Tags {
			if tag[0] == "id" {
				counts[tag[1]]++
			}
		}
	}
	var entries []JournalEntry
	bankBalance := int64(0)
	for _, tx := range transactions {
		date, err := time.Parse("2006-01-02", tx.Date)
		if err != nil {
			return nil, fmt.Errorf("read journal date: %w", err)
		}
		bankChange := int64(0)
		if b.Config.BankAccount != "" {
			bankChange, err = bankAmount(tx, b.Config.BankAccount)
			if err != nil {
				return nil, fmt.Errorf("%s %s: read bank amount: %w", tx.Date, tx.Description, err)
			}
			bankBalance += bankChange
		}
		if date.Year() != b.Config.Year {
			continue
		}
		id, idErr := transactionID(tx)
		entry := JournalEntry{
			ID: id, Date: tx.Date, Description: tx.Description,
			BankChange: bankChange, BankBalance: bankBalance,
		}
		switch {
		case locked[id] != nil:
			entry.ReadOnly = fmt.Sprintf("Locked by finished bank reconciliation %s to %s.",
				locked[id].Period.Start, locked[id].Period.End)
		case b.transactionFile(tx) != b.Config.WriteJournal:
			entry.ReadOnly = "Entry is outside the configured working journal."
		case idErr != nil:
			entry.ReadOnly = idErr.Error()
		case counts[id] != 1:
			entry.ReadOnly = "Entry has a duplicate ID; resolve it before revising."
		}
		if entry.ReadOnly == "" {
			if _, err := editableEntry(tx); err != nil {
				entry.ReadOnly = err.Error()
			}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func presentJSON(value json.RawMessage) bool {
	return len(value) != 0 && !bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

func editableEntry(tx journalTransaction) (Entry, error) {
	unsupported := func() (Entry, error) {
		return Entry{}, fmt.Errorf("this entry has posting annotations, status, or advanced ledger features that the structured form cannot preserve; it remains read-only in wat")
	}
	if tx.Code != "" || tx.Status != "Unmarked" || tx.Date2 != nil {
		return unsupported()
	}
	payee, description, _ := strings.Cut(tx.Description, "|")
	entry := Entry{Date: tx.Date, Payee: strings.TrimSpace(payee), Description: strings.TrimSpace(description)}
	id, err := transactionID(tx)
	if err != nil {
		return Entry{}, err
	}
	values := map[string]string{"id": id}
	counts := make(map[string]int)
	for _, tag := range tx.Tags {
		switch tag[0] {
		case "id":
		case "reference":
			entry.Reference = tag[1]
		case "source":
			entry.Source = tag[1]
		case "bank-row":
			entry.BankRow = tag[1]
		case "bank-description":
			entry.BankDescription = tag[1]
		default:
			continue
		}
		counts[tag[0]]++
		if counts[tag[0]] != 1 {
			return unsupported()
		}
		values[tag[0]] = tag[1]
	}
	commentTags := make(map[string]int)
	for _, line := range strings.Split(tx.Comment, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		name, value, ok := strings.Cut(line, ":")
		expected, known := values[strings.TrimSpace(name)]
		if ok && known {
			if strings.TrimSpace(value) != expected {
				return unsupported()
			}
			commentTags[strings.TrimSpace(name)]++
		} else {
			entry.comments = append(entry.comments, line)
		}
	}
	for name, count := range counts {
		if commentTags[name] != count {
			return unsupported()
		}
	}
	for index, posting := range tx.Postings {
		if posting.Type != "RegularPosting" || posting.Status != "Unmarked" ||
			posting.Comment != "" || posting.Date != nil || posting.Date2 != nil ||
			len(posting.Tags) != 0 || presentJSON(posting.Assertion) || presentJSON(posting.Original) ||
			len(posting.Amounts) != 1 || presentJSON(posting.Amounts[0].Cost) || presentJSON(posting.Amounts[0].CostBasis) {
			return unsupported()
		}
		single := tx
		single.Postings = tx.Postings[index : index+1]
		cents, err := bankAmount(single, posting.Account)
		if err != nil {
			return Entry{}, err
		}
		entry.Postings = append(entry.Postings, Posting{Account: posting.Account, Amount: formatCents(cents)})
	}
	return entry, nil
}

func entryOffsets(tx journalTransaction, data []byte) (int, int, error) {
	if len(tx.Source) != 2 || tx.Source[0].Name != tx.Source[1].Name ||
		tx.Source[0].Column != 1 || tx.Source[1].Column < 1 {
		return 0, 0, fmt.Errorf("hledger did not return a supported transaction source range")
	}
	offsets := []int{0}
	for index, c := range data {
		if c == '\n' {
			offsets = append(offsets, index+1)
		}
	}
	start, end := tx.Source[0].Line-1, tx.Source[1].Line-1
	if tx.Source[1].Column != 1 {
		end++
	}
	if start < 0 || start >= len(offsets) || end < start || end > len(offsets) {
		return 0, 0, fmt.Errorf("hledger transaction source range is outside the working journal")
	}
	endOffset := len(data)
	if end < len(offsets) {
		endOffset = offsets[end]
	}
	if offsets[start] >= endOffset {
		return 0, 0, fmt.Errorf("hledger returned an empty transaction range")
	}
	return offsets[start], endOffset, nil
}

func (b *Books) EditDraft(id string) (*Draft, Entry, error) {
	draft, err := b.NewDraft()
	if err != nil {
		return nil, Entry{}, err
	}
	transactions, err := b.transactions(b.Config.Journal)
	if err != nil {
		return nil, Entry{}, err
	}
	var found *journalTransaction
	count := 0
	for index, tx := range transactions {
		for _, tag := range tx.Tags {
			if tag[0] == "id" && tag[1] == id {
				count++
				found = &transactions[index]
			}
		}
	}
	if count != 1 || strings.TrimSpace(id) == "" {
		return nil, Entry{}, fmt.Errorf("expected exactly one entry with ID %q; found %d", id, count)
	}
	locked, err := b.lockedTransactions()
	if err != nil {
		return nil, Entry{}, err
	}
	if checkpoint := locked[id]; checkpoint != nil {
		return nil, Entry{}, fmt.Errorf("transaction is locked by finished bank reconciliation %s to %s",
			checkpoint.Period.Start, checkpoint.Period.End)
	}
	if b.transactionFile(*found) != b.Config.WriteJournal {
		return nil, Entry{}, fmt.Errorf("only entries in the configured working journal can be revised; other included journals are read-only")
	}
	entry, err := editableEntry(*found)
	if err != nil {
		return nil, Entry{}, err
	}
	start, end, err := entryOffsets(*found, draft.files[b.Config.WriteJournal].data)
	if err != nil {
		return nil, Entry{}, err
	}
	draft.id = id
	draft.revision = &entryRevision{start: start, end: end, source: entry.Source}
	if entry.BankRow != "" {
		hash, number, ok := strings.Cut(entry.BankRow, "/")
		index, err := strconv.Atoi(number)
		if !ok || len(hash) != 64 || strings.ContainsAny(hash, `/\`) || err != nil || index < 1 {
			return nil, Entry{}, fmt.Errorf("recorded entry has an invalid bank-row tag")
		}
		statement, err := b.cachedStatement(hash)
		if err != nil {
			return nil, Entry{}, fmt.Errorf("load recorded statement evidence: %w", err)
		}
		if statement.Hash != hash || index > len(statement.Rows) {
			return nil, Entry{}, fmt.Errorf("recorded entry does not identify a cached statement row")
		}
		draft.statement = &statementDraft{statement, index - 1, b.Config.BankAccount}
		if err := draft.checkStatement(*found); err != nil {
			return nil, Entry{}, err
		}
		if entry.Source == "" {
			return nil, Entry{}, fmt.Errorf("recorded entry must retain its source tag before it can be revised")
		}
	} else if entry.BankDescription != "" {
		return nil, Entry{}, fmt.Errorf("recorded entry has a bank-description without a bank-row")
	}
	if err := draft.unchanged(); err != nil {
		return nil, Entry{}, err
	}
	declared, err := b.declaredAccounts()
	if err != nil {
		return nil, Entry{}, err
	}
	if _, err := entry.render(b.Config.Year, declared, id); err != nil {
		return nil, Entry{}, fmt.Errorf("entry cannot be represented by the structured form: %w", err)
	}
	if err := draft.previewText(string(draft.files[b.Config.WriteJournal].data[start:end]), false); err != nil {
		return nil, Entry{}, fmt.Errorf("verify original transaction source range: %w", err)
	}
	draft.text = ""
	return draft, entry, nil
}

func (b *Books) EditStatementDraft(statement *Statement, index int) (*Draft, Entry, error) {
	if index < 0 || index >= len(statement.Rows) {
		return nil, Entry{}, fmt.Errorf("statement row is out of range")
	}
	statuses, err := b.StatementStatuses(statement)
	if err != nil {
		return nil, Entry{}, err
	}
	if !statuses[index].Recorded {
		return nil, Entry{}, fmt.Errorf("statement row is not recorded")
	}
	transactions, err := b.transactions(b.Config.Journal)
	if err != nil {
		return nil, Entry{}, err
	}
	for _, tx := range transactions {
		if transactionTag(tx, "bank-row") == statement.key(index) {
			id, err := transactionID(tx)
			if err != nil {
				return nil, Entry{}, err
			}
			return b.EditDraft(id)
		}
	}
	return nil, Entry{}, fmt.Errorf("recorded statement row was not found; reopen the list")
}
