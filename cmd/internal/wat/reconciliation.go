package wat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const reconciliationVersion = 1

type ReconciledTransaction struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
	Row    int    `json:"statement_row"`
	Date   string `json:"date"`
	Cents  int64  `json:"cents"`
}

type OutstandingTransaction struct {
	ID          string `json:"id"`
	Digest      string `json:"digest"`
	Date        string `json:"date"`
	Description string `json:"description"`
	Cents       int64  `json:"cents"`
}

type ReconciliationCheckpoint struct {
	Version         int                      `json:"version"`
	StatementHash   string                   `json:"statement_hash"`
	StatementDigest string                   `json:"statement_digest"`
	BankAccount     string                   `json:"bank_account"`
	Period          StatementPeriod          `json:"period"`
	OpeningCents    int64                    `json:"opening_cents"`
	ClosingCents    int64                    `json:"closing_cents"`
	BookClosing     int64                    `json:"book_closing_cents"`
	FinishedAt      string                   `json:"finished_at"`
	Transactions    []ReconciledTransaction  `json:"reconciled_transactions"`
	Outstanding     []OutstandingTransaction `json:"outstanding_transactions,omitempty"`
}

type ReconciliationItem struct {
	ID, Date, Description string
	Cents                 int64
	Outstanding           bool
}

type Reconciliation struct {
	Statement         *Statement
	Checkpoint        *ReconciliationCheckpoint
	RowsRecorded      int
	BookOpening       int64
	StatementCredits  int64
	StatementDebits   int64
	BookCredits       int64
	BookDebits        int64
	OutstandingCredit int64
	OutstandingDebit  int64
	BookClosing       int64
	AdjustedStatement int64
	Difference        int64
	Items             []ReconciliationItem
}

type reconciliationWork struct {
	Version       int      `json:"version"`
	StatementHash string   `json:"statement_hash"`
	Outstanding   []string `json:"outstanding_transaction_ids"`
}

func transactionDigest(tx journalTransaction) (string, error) {
	tx.Source = nil
	data, err := json.Marshal(tx)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func statementDigest(statement *Statement) (string, error) {
	data, err := json.Marshal(struct {
		Hash         string
		Period       *StatementPeriod
		OpeningCents int64
		ClosingCents int64
		Rows         []StatementRow
	}{
		statement.Hash, statement.Period, statement.OpeningCents, statement.ClosingCents, statement.Rows,
	})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func (b *Books) reconciliationPath(hash, name string) string {
	return filepath.Join(b.Config.Sources, "statements", hash, name)
}

func decodeJSONFile(path string, target any) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("not a regular file: %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("contains more than one JSON value")
	} else if !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func (b *Books) loadCheckpoint(statement *Statement) (*ReconciliationCheckpoint, error) {
	path := b.reconciliationPath(statement.Hash, "reconciliation.json")
	var checkpoint ReconciliationCheckpoint
	if err := decodeJSONFile(path, &checkpoint); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read reconciliation checkpoint: %w", err)
	}
	if checkpoint.Version != reconciliationVersion || checkpoint.StatementHash != statement.Hash ||
		checkpoint.BankAccount != b.Config.BankAccount || statement.Period == nil ||
		checkpoint.Period != *statement.Period || checkpoint.OpeningCents != statement.OpeningCents ||
		checkpoint.ClosingCents != statement.ClosingCents {
		return nil, fmt.Errorf("reconciliation checkpoint no longer matches its statement evidence")
	}
	digest, err := statementDigest(statement)
	if err != nil {
		return nil, err
	}
	if checkpoint.StatementDigest != digest {
		return nil, fmt.Errorf("reconciled statement evidence changed after Finish")
	}
	if _, err := time.Parse(time.RFC3339, checkpoint.FinishedAt); err != nil {
		return nil, fmt.Errorf("reconciliation checkpoint has an invalid finished_at timestamp")
	}
	if len(checkpoint.Transactions) != len(statement.Rows) {
		return nil, fmt.Errorf("reconciliation checkpoint must cover every statement row")
	}
	rows := make(map[int]bool)
	ids := make(map[string]bool)
	validDigest := func(value string) bool {
		decoded, err := hex.DecodeString(value)
		return err == nil && len(decoded) == sha256.Size && value == strings.ToLower(value)
	}
	for _, item := range checkpoint.Transactions {
		if item.Row < 1 || item.Row > len(statement.Rows) || rows[item.Row] {
			return nil, fmt.Errorf("reconciliation checkpoint has invalid statement-row coverage")
		}
		row := statement.Rows[item.Row-1]
		if strings.TrimSpace(item.ID) == "" || ids[item.ID] || !validDigest(item.Digest) ||
			item.Date != row.Date || item.Cents != row.Cents {
			return nil, fmt.Errorf("reconciliation checkpoint has invalid reconciled transaction evidence")
		}
		rows[item.Row], ids[item.ID] = true, true
	}
	adjusted := checkpoint.ClosingCents
	for _, item := range checkpoint.Outstanding {
		if strings.TrimSpace(item.ID) == "" || ids[item.ID] || !validDigest(item.Digest) ||
			item.Date == "" || item.Date > checkpoint.Period.End || item.Cents == 0 {
			return nil, fmt.Errorf("reconciliation checkpoint has invalid outstanding transaction evidence")
		}
		ids[item.ID] = true
		adjusted += item.Cents
	}
	if checkpoint.BookClosing != adjusted {
		return nil, fmt.Errorf("reconciliation checkpoint does not balance its closing and outstanding amounts")
	}
	return &checkpoint, nil
}

func (b *Books) checkpoints() ([]*ReconciliationCheckpoint, error) {
	statements, err := b.CachedStatements()
	if err != nil {
		return nil, err
	}
	var checkpoints []*ReconciliationCheckpoint
	for _, statement := range statements {
		checkpoint, err := b.loadCheckpoint(statement)
		if err != nil {
			return nil, err
		}
		if checkpoint != nil {
			checkpoints = append(checkpoints, checkpoint)
		}
	}
	slices.SortStableFunc(checkpoints, func(a, b *ReconciliationCheckpoint) int {
		return strings.Compare(a.Period.Start, b.Period.Start)
	})
	return checkpoints, nil
}

func (b *Books) lockedTransactions() (map[string]*ReconciliationCheckpoint, error) {
	checkpoints, err := b.checkpoints()
	if err != nil {
		return nil, err
	}
	if len(checkpoints) == 0 {
		return make(map[string]*ReconciliationCheckpoint), nil
	}
	transactions, err := b.transactions(b.Config.Journal, "--strict")
	if err != nil {
		return nil, err
	}
	current := make(map[string][]journalTransaction)
	for _, tx := range transactions {
		id, err := transactionID(tx)
		if err != nil {
			continue
		}
		current[id] = append(current[id], tx)
	}
	locked := make(map[string]*ReconciliationCheckpoint)
	for _, checkpoint := range checkpoints {
		for _, item := range checkpoint.Transactions {
			if previous := locked[item.ID]; previous != nil {
				return nil, fmt.Errorf("transaction %q is locked by multiple reconciliation checkpoints", item.ID)
			}
			matches := current[item.ID]
			if len(matches) == 0 {
				return nil, fmt.Errorf("reconciled transaction %q is missing from the journal", item.ID)
			}
			if len(matches) != 1 {
				return nil, fmt.Errorf("reconciled transaction %q has a duplicate transaction ID", item.ID)
			}
			digest, err := transactionDigest(matches[0])
			if err != nil {
				return nil, err
			}
			if digest != item.Digest {
				return nil, fmt.Errorf("reconciled transaction %q changed after Finish", item.ID)
			}
			locked[item.ID] = checkpoint
		}
	}
	return locked, nil
}

func (b *Books) verifyReconciliations() error {
	if b.Config.BankAccount == "" {
		return nil
	}
	_, err := b.lockedTransactions()
	return err
}

func (b *Books) loadReconciliationWork(statement *Statement) (map[string]bool, error) {
	path := b.reconciliationPath(statement.Hash, "reconciliation-work.json")
	var work reconciliationWork
	if err := decodeJSONFile(path, &work); err != nil {
		if os.IsNotExist(err) {
			return make(map[string]bool), nil
		}
		return nil, fmt.Errorf("read reconciliation work: %w", err)
	}
	if work.Version != reconciliationVersion || work.StatementHash != statement.Hash {
		return nil, fmt.Errorf("saved reconciliation work does not match this statement")
	}
	selected := make(map[string]bool)
	for _, id := range work.Outstanding {
		if selected[id] || strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("saved reconciliation work contains an invalid outstanding transaction ID")
		}
		selected[id] = true
	}
	return selected, nil
}

func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

func writePrivateJSON(path string, value any) (err error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".wat-reconcile-")
	if err != nil {
		return err
	}
	tempPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return errors.Join(err, os.Remove(tempPath))
	}
	defer func() {
		if removeErr := os.Remove(tempPath); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, removeErr)
		}
	}()
	if err := writeSynced(tempPath, data, 0600); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return syncDirectory(directory)
}

func (b *Books) saveReconciliationWork(statement *Statement, selected map[string]bool) error {
	ids := make([]string, 0, len(selected))
	for id, outstanding := range selected {
		if outstanding {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return writePrivateJSON(b.reconciliationPath(statement.Hash, "reconciliation-work.json"), reconciliationWork{
		Version: reconciliationVersion, StatementHash: statement.Hash, Outstanding: ids,
	})
}

func (b *Books) Reconciliation(statement *Statement) (*Reconciliation, error) {
	if statement.Period == nil {
		return nil, fmt.Errorf("statement dates are required before reconciliation")
	}
	checkpoint, err := b.loadCheckpoint(statement)
	if err != nil {
		return nil, err
	}
	result := &Reconciliation{Statement: statement, Checkpoint: checkpoint}
	if checkpoint != nil {
		result.RowsRecorded = len(checkpoint.Transactions)
		result.BookClosing = checkpoint.BookClosing
		result.AdjustedStatement = checkpoint.BookClosing
		return result, nil
	}
	statements, err := b.CachedStatements()
	if err != nil {
		return nil, err
	}
	for _, earlier := range statements {
		if earlier.Hash == statement.Hash || earlier.Period == nil {
			continue
		}
		if earlier.Period.Start <= statement.Period.End && earlier.Period.End >= statement.Period.Start {
			return nil, fmt.Errorf("statement period overlaps cached statement %s to %s", earlier.Period.Start, earlier.Period.End)
		}
		if earlier.Period.Start >= statement.Period.Start {
			continue
		}
		earlierCheckpoint, err := b.loadCheckpoint(earlier)
		if err != nil {
			return nil, err
		}
		if earlierCheckpoint == nil {
			return nil, fmt.Errorf("finish the earlier statement %s to %s first", earlier.Period.Start, earlier.Period.End)
		}
	}
	checkpoints, err := b.checkpoints()
	if err != nil {
		return nil, err
	}
	var previous *ReconciliationCheckpoint
	for _, item := range checkpoints {
		if item.Period.End < statement.Period.Start && (previous == nil || item.Period.End > previous.Period.End) {
			previous = item
		}
	}
	if previous != nil && previous.ClosingCents != statement.OpeningCents {
		return nil, fmt.Errorf("statement opening balance %s does not equal the prior reconciled closing balance %s",
			formatCents(statement.OpeningCents), formatCents(previous.ClosingCents))
	}
	statuses, err := b.StatementStatuses(statement)
	if err != nil {
		return nil, err
	}
	for _, status := range statuses {
		if status.Recorded {
			result.RowsRecorded++
		}
	}
	for _, row := range statement.Rows {
		if row.Cents > 0 {
			result.StatementCredits += row.Cents
		} else {
			result.StatementDebits -= row.Cents
		}
	}
	transactions, err := b.transactions(b.Config.Journal, "--strict")
	if err != nil {
		return nil, err
	}
	locked, err := b.lockedTransactions()
	if err != nil {
		return nil, err
	}
	matched := make(map[string]bool)
	for index := range statement.Rows {
		for _, tx := range transactions {
			if transactionTag(tx, "bank-row") != statement.key(index) {
				continue
			}
			id, err := transactionID(tx)
			if err != nil {
				return nil, fmt.Errorf("statement row %d: %w", index+1, err)
			}
			if locked[id] != nil {
				return nil, fmt.Errorf("statement row %d transaction is already locked by another reconciliation", index+1)
			}
			matched[id] = true
		}
	}
	selected, err := b.loadReconciliationWork(statement)
	if err != nil {
		return nil, err
	}
	candidates := make(map[string]bool)
	for _, tx := range transactions {
		if tx.Date > statement.Period.End {
			continue
		}
		amount, err := bankAmount(tx, b.Config.BankAccount)
		if err != nil {
			return nil, err
		}
		if amount == 0 {
			continue
		}
		result.BookClosing += amount
		if tx.Date < statement.Period.Start {
			result.BookOpening += amount
		} else if amount > 0 {
			result.BookCredits += amount
		} else {
			result.BookDebits -= amount
		}
		id, err := transactionID(tx)
		if err != nil {
			return nil, fmt.Errorf("%s %s has a bank posting but cannot be reconciled: %w", tx.Date, tx.Description, err)
		}
		if locked[id] != nil || matched[id] {
			continue
		}
		if candidates[id] {
			return nil, fmt.Errorf("duplicate transaction ID %q prevents reconciliation", id)
		}
		candidates[id] = true
		result.Items = append(result.Items, ReconciliationItem{
			ID: id, Date: tx.Date, Description: tx.Description, Cents: amount, Outstanding: selected[id],
		})
	}
	expectedOpening := statement.OpeningCents
	expectedSource := "statement opening"
	if previous != nil {
		expectedOpening = previous.BookClosing
		expectedSource = "prior reconciled book closing"
	}
	if result.BookOpening != expectedOpening {
		return nil, fmt.Errorf(
			"cannot begin reconciliation: book opening balance %s does not equal %s %s; correct the journal before reconciling",
			formatCents(result.BookOpening), expectedSource, formatCents(expectedOpening))
	}
	for id := range selected {
		if !candidates[id] {
			return nil, fmt.Errorf("saved outstanding transaction %q is no longer an unmatched bank entry; clear its saved selection before continuing", id)
		}
	}
	slices.SortStableFunc(result.Items, func(a, b ReconciliationItem) int {
		if order := strings.Compare(a.Date, b.Date); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	result.AdjustedStatement = statement.ClosingCents
	for _, item := range result.Items {
		if item.Outstanding {
			result.AdjustedStatement += item.Cents
			if item.Cents > 0 {
				result.OutstandingCredit += item.Cents
			} else {
				result.OutstandingDebit -= item.Cents
			}
		}
	}
	result.Difference = result.BookClosing - result.AdjustedStatement
	return result, nil
}

func (b *Books) SetOutstanding(statement *Statement, id string, outstanding bool) error {
	reconciliation, err := b.Reconciliation(statement)
	if err != nil {
		return err
	}
	found := false
	selected := make(map[string]bool)
	for _, item := range reconciliation.Items {
		if item.Outstanding {
			selected[item.ID] = true
		}
		if item.ID == id {
			found = true
		}
	}
	if !found {
		return fmt.Errorf("transaction is not an unmatched bank entry in this reconciliation")
	}
	if outstanding {
		selected[id] = true
	} else {
		delete(selected, id)
	}
	return b.saveReconciliationWork(statement, selected)
}

func (b *Books) FinishReconciliation(statement *Statement) (err error) {
	reconciliationLockPath := b.reconciliationPath(statement.Hash, ".reconcile-lock")
	lock, err := os.OpenFile(reconciliationLockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("acquire reconciliation lock: %w", err)
	}
	if err := lock.Close(); err != nil {
		return errors.Join(err, os.Remove(reconciliationLockPath))
	}
	defer func() {
		if removeErr := os.Remove(reconciliationLockPath); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, removeErr)
		}
	}()
	journalLockPath := b.Config.WriteJournal + ".wat-lock"
	journalLock, err := os.OpenFile(journalLockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("acquire journal lock before Finish: %w", err)
	}
	if err := journalLock.Close(); err != nil {
		return errors.Join(err, os.Remove(journalLockPath))
	}
	defer func() {
		if removeErr := os.Remove(journalLockPath); removeErr != nil && !os.IsNotExist(removeErr) {
			err = errors.Join(err, removeErr)
		}
	}()
	reconciliation, err := b.Reconciliation(statement)
	if err != nil {
		return err
	}
	if reconciliation.Checkpoint != nil {
		return fmt.Errorf("statement reconciliation is already finished")
	}
	if reconciliation.RowsRecorded != len(statement.Rows) {
		return fmt.Errorf("record every statement row before Finish")
	}
	if reconciliation.Difference != 0 {
		return fmt.Errorf("unexplained difference must be zero before Finish; currently %s", formatCents(reconciliation.Difference))
	}
	transactions, err := b.transactions(b.Config.Journal, "--strict")
	if err != nil {
		return err
	}
	byID := make(map[string]journalTransaction)
	for _, tx := range transactions {
		id, idErr := transactionID(tx)
		if idErr == nil {
			byID[id] = tx
		}
	}
	checkpoint := ReconciliationCheckpoint{
		Version: reconciliationVersion, StatementHash: statement.Hash, BankAccount: b.Config.BankAccount,
		Period: *statement.Period, OpeningCents: statement.OpeningCents, ClosingCents: statement.ClosingCents,
		BookClosing: reconciliation.BookClosing, FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	checkpoint.StatementDigest, err = statementDigest(statement)
	if err != nil {
		return err
	}
	for index := range statement.Rows {
		found := false
		for _, tx := range transactions {
			if transactionTag(tx, "bank-row") != statement.key(index) {
				continue
			}
			id, err := transactionID(tx)
			if err != nil {
				return err
			}
			digest, err := transactionDigest(tx)
			if err != nil {
				return err
			}
			checkpoint.Transactions = append(checkpoint.Transactions, ReconciledTransaction{
				ID: id, Digest: digest, Row: index + 1, Date: tx.Date, Cents: statement.Rows[index].Cents,
			})
			found = true
		}
		if !found {
			return fmt.Errorf("statement row %d is no longer recorded", index+1)
		}
	}
	for _, item := range reconciliation.Items {
		if !item.Outstanding {
			continue
		}
		tx, ok := byID[item.ID]
		if !ok {
			return fmt.Errorf("outstanding transaction %q disappeared before Finish", item.ID)
		}
		digest, err := transactionDigest(tx)
		if err != nil {
			return err
		}
		checkpoint.Outstanding = append(checkpoint.Outstanding, OutstandingTransaction{
			ID: item.ID, Digest: digest, Date: item.Date, Description: item.Description, Cents: item.Cents,
		})
	}
	path := b.reconciliationPath(statement.Hash, "reconciliation.json")
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("statement reconciliation is already finished")
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := writePrivateJSON(path, checkpoint); err != nil {
		return fmt.Errorf("write reconciliation checkpoint: %w", err)
	}
	work := b.reconciliationPath(statement.Hash, "reconciliation-work.json")
	if err := os.Remove(work); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reconciliation finished, but working-selection cleanup failed: %w", err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return fmt.Errorf("reconciliation finished, but directory sync failed: %w", err)
	}
	return nil
}
