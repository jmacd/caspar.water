package wat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRevisionReplacesOnlySelectedEntry(t *testing.T) {
	entry := testEntry()
	first := entry.journalText("first")
	middle := entry.journalText("middle")
	last := entry.journalText("last")
	original := "; opening notes\n" + first + "\n; keep preceding comment\n" + middle + "\n" + last + "\n; closing notes\n"
	books, _ := fixture(t, original)
	draft, revised, err := books.EditDraft("middle")
	require.NoError(t, err)
	require.Equal(t, "middle", draft.id)
	require.Equal(t, entry, revised)
	revised.Payee = "Corrected utility"
	revised.Postings[0].Amount = "7.34"
	revised.Postings = append(revised.Postings, Posting{Account: "Income:421:Misc", Amount: "5.00"})
	text, err := draft.Preview(revised)
	require.NoError(t, err)
	backup, err := draft.Save()
	require.NoError(t, err)
	require.Equal(t, original, read(t, backup))
	want := strings.Replace(original, middle, text, 1)
	require.Equal(t, want, read(t, books.Config.ManualJournal))
	require.Equal(t, 1, strings.Count(want, "; id: middle"))
	transactions, err := books.transactions(books.Config.Journal)
	require.NoError(t, err)
	require.Len(t, transactions, 3)
	require.Len(t, transactions[1].Postings, 3)
	_, err = draft.Save()
	require.Error(t, err)
	require.Equal(t, want, read(t, books.Config.ManualJournal))
}

func TestRevisionPreservesTransactionNotes(t *testing.T) {
	text := testEntry().journalText("notes")
	text = strings.Replace(text, "    ; id: notes\n", "    ; id: notes\n    ; Receipt was corrected after review.\n    ; tax-year: 2021\n", 1)
	books, _ := fixture(t, text)
	draft, entry, err := books.EditDraft("notes")
	require.NoError(t, err)
	entry.Description = "Revised description"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	saved := read(t, books.Config.ManualJournal)
	require.Contains(t, saved, "; Receipt was corrected after review.")
	require.Contains(t, saved, "; tax-year: 2021")
	require.Contains(t, saved, "Revised description")
	_, _, err = books.EditDraft("notes")
	require.NoError(t, err, "preserved notes must remain revisable")
}

func TestRevisionRejectsStaleAndInvalidEdits(t *testing.T) {
	original := testEntry().journalText("edit")
	books, _ := fixture(t, original)
	draft, entry, err := books.EditDraft("edit")
	require.NoError(t, err)
	require.Error(t, draft.PreviewText(strings.Replace(original, "id: edit", "id: changed", 1)))
	entry.Postings[0].Amount = "999.00"
	_, err = draft.Preview(entry)
	require.Error(t, err)
	require.Equal(t, original, read(t, books.Config.ManualJournal))
	entry.Postings[0].Amount = "12.34"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(original+"; concurrent change\n"), 0600))
	_, err = draft.Save()
	require.ErrorContains(t, err, "journal changed")
	require.Equal(t, original+"; concurrent change\n", read(t, books.Config.ManualJournal))
}

func TestRevisionIncludedJournalsAreReadOnly(t *testing.T) {
	working := testEntry().journalText("working")
	closed := testEntry().journalText("closed")
	books, root := fixture(t, working)
	path := filepath.Join(root, "books", "closed.journal")
	require.NoError(t, os.WriteFile(path, []byte(closed), 0600))
	require.NoError(t, os.WriteFile(books.Config.Journal, []byte(read(t, books.Config.Journal)+"include closed.journal\n"), 0600))
	_, _, err := books.EditDraft("closed")
	require.ErrorContains(t, err, "read-only")
	entries, err := books.JournalEntries()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Empty(t, entries[0].ReadOnly)
	require.NotEmpty(t, entries[1].ReadOnly)
	require.Equal(t, closed, read(t, path))
	require.Equal(t, working, read(t, books.Config.ManualJournal))
}

func TestJournalEntriesIncludeRunningBankBalance(t *testing.T) {
	prior := Entry{
		Date: "2020-12-31", Payee: "Opening bank balance",
		Postings: []Posting{
			{Account: "Bank:131.2:Checking", Amount: "100.00"},
			{Account: "Equity:218:Owner", Amount: "-100.00"},
		},
	}.journalText("prior")
	payment := testEntry().journalText("payment")
	deposit := Entry{
		Date: "2021-04-06", Payee: "Deposit",
		Postings: []Posting{
			{Account: "Bank:131.2:Checking", Amount: "20.00"},
			{Account: "Equity:218:Owner", Amount: "-20.00"},
		},
	}.journalText("deposit")
	books, _ := fixture(t, payment+"\n"+deposit+"\n"+prior)
	entries, err := books.JournalEntries()
	require.NoError(t, err)
	require.Len(t, entries, 3)
	require.Equal(t, int64(10000), entries[0].BankChange)
	require.Equal(t, int64(10000), entries[0].BankBalance)
	require.Equal(t, int64(-1234), entries[1].BankChange)
	require.Equal(t, int64(8766), entries[1].BankBalance)
	require.Equal(t, int64(2000), entries[2].BankChange)
	require.Equal(t, int64(10766), entries[2].BankBalance)
}

func TestJournalEntriesTrackConfiguredBankAccountsIndependently(t *testing.T) {
	books, _ := fixture(t, "")
	second := addBankAccount(t, books, "Bank:131.2:Savings")
	checking := Entry{
		Date: "2021-04-05", Payee: "Checking deposit",
		Postings: []Posting{
			{Account: "Bank:131.2:Checking", Amount: "10.00"},
			{Account: "Equity:218:Owner", Amount: "-10.00"},
		},
	}.journalText("checking")
	savings := Entry{
		Date: "2021-04-06", Payee: "Savings deposit",
		Postings: []Posting{
			{Account: second.Account, Amount: "20.00"},
			{Account: "Equity:218:Owner", Amount: "-20.00"},
		},
	}.journalText("savings")
	require.NoError(t, os.WriteFile(books.Config.ManualJournal, []byte(checking+savings), 0600))

	entries, err := books.JournalEntries()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "Bank:131.2:Checking", entries[0].BankAccount)
	require.Equal(t, int64(1000), entries[0].BankBalance)
	require.True(t, entries[0].BankValues)
	require.Equal(t, second.Account, entries[1].BankAccount)
	require.Equal(t, int64(2000), entries[1].BankBalance)
	require.True(t, entries[1].BankValues)
}

func TestRevisionRejectsDuplicateOrMissingIDs(t *testing.T) {
	for _, text := range []string{
		testEntry().journalText("duplicate") + "\n" + testEntry().journalText("duplicate"),
		strings.Replace(testEntry().journalText("duplicate"), "; id: duplicate", "; no ID", 1),
	} {
		books, _ := fixture(t, text)
		_, _, err := books.EditDraft("duplicate")
		require.Error(t, err)
		entries, err := books.JournalEntries()
		require.NoError(t, err)
		for _, entry := range entries {
			require.NotEmpty(t, entry.ReadOnly)
		}
		require.Equal(t, text, read(t, books.Config.ManualJournal))
	}
}

func TestRevisionDoesNotDropUnsupportedLedgerFeatures(t *testing.T) {
	original := testEntry().journalText("advanced")
	for name, text := range map[string]string{
		"status":             strings.Replace(original, "2021-04-05 ", "2021-04-05 * ", 1),
		"code":               strings.Replace(original, "2021-04-05 ", "2021-04-05 (check1) ", 1),
		"secondary date":     strings.Replace(original, "2021-04-05 ", "2021-04-05=2021-04-06 ", 1),
		"posting annotation": strings.Replace(original, "$12.34\n", "$12.34 ; keep this posting note\n", 1),
		"assertion":          strings.Replace(original, "$-12.34\n", "$-12.34 = $-12.34\n", 1),
	} {
		t.Run(name, func(t *testing.T) {
			books, _ := fixture(t, text)
			_, _, err := books.EditDraft("advanced")
			require.ErrorContains(t, err, "cannot preserve")
			require.Equal(t, text, read(t, books.Config.ManualJournal))
		})
	}
}

func TestRevisionSourceOffsets(t *testing.T) {
	for _, ending := range []string{"\n", "\r\n", ""} {
		t.Run("ending="+ending, func(t *testing.T) {
			entry := testEntry()
			entry.Payee = "Caf\u00e9 utility"
			text := strings.TrimSuffix(entry.journalText("last"), "\n")
			if ending != "" {
				text = strings.ReplaceAll(text, "\n", ending) + ending
			}
			books, _ := fixture(t, text)
			draft, revised, err := books.EditDraft("last")
			require.NoError(t, err)
			revised.Payee = "Changed"
			expected, err := draft.Preview(revised)
			require.NoError(t, err)
			_, err = draft.Save()
			require.NoError(t, err)
			require.Equal(t, expected, read(t, books.Config.ManualJournal))
		})
	}
}

func TestRevisionRecordedStatementPreservesBankEvidence(t *testing.T) {
	books, statement, _ := statementFixture(t)
	draft, entry, err := books.StatementDraft(statement, 1)
	require.NoError(t, err)
	entry.Postings[1].Account = "Expense:615:Utility"
	_, err = draft.Preview(entry)
	require.NoError(t, err)
	_, err = draft.Save()
	require.NoError(t, err)
	journal := bankJournal(t, books, statement.BankAccount)
	original := read(t, journal)
	revision, revised, err := books.EditStatementDraft(statement, 1)
	require.NoError(t, err)
	require.Equal(t, draft.id, revision.id)
	for _, text := range []string{
		strings.Replace(original, "2021-04-05", "2021-04-06", 1),
		strings.ReplaceAll(original, "12.34", "13.34"),
		strings.Replace(original, "bank-row:", "lost-row:", 1),
		strings.Replace(original, "bank-description:", "lost-description:", 1),
		strings.Replace(original, "source:", "lost-source:", 1),
		strings.Replace(original, "source: "+entry.Source, "source: other.pdf", 1),
	} {
		require.Error(t, revision.PreviewText(text))
		require.Equal(t, original, read(t, journal))
	}
	revised.Payee = "Corrected power payee"
	revised.Postings[1].Account = "Income:421:Misc"
	_, err = revision.Preview(revised)
	require.NoError(t, err)
	_, err = revision.Save()
	require.NoError(t, err)
	statuses, err := books.StatementStatuses(statement)
	require.NoError(t, err)
	require.True(t, statuses[1].Recorded)
	require.Equal(t, "Income:421:Misc", statuses[2].Suggestion)
	saved := read(t, journal)
	require.Equal(t, 1, strings.Count(saved, "; id: "+draft.id))
	require.Equal(t, 1, strings.Count(saved, "; bank-row: "+statement.key(1)))
	require.Contains(t, saved, "Bank:131.2:Checking  $-12.34")
	require.Contains(t, saved, "; source: "+entry.Source)
	require.Contains(t, saved, "Corrected power payee")
}
