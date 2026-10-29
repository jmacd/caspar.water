package wat

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type quickBooksMappingReview struct {
	year     int
	labels   []string
	details  map[string]string
	mappings map[string]string
	original map[string]string
	accounts []PostingAccount
}

func RunQuickBooksMappingTerminal(
	ctx context.Context,
	books *Books,
	paths []string,
	year int,
	progress io.Writer,
) error {
	records, err := books.quickBooksRecords(ctx, paths, year, progress)
	if err != nil {
		return err
	}
	review, err := books.quickBooksMappingReview(records, year)
	if err != nil {
		return err
	}
	applyTUITheme()
	ui := &terminal{app: tview.NewApplication(), books: books}
	ui.captureGlobalInput()
	ui.quickBooksMappings(review)
	return ui.app.Run()
}

func (b *Books) quickBooksMappingReview(
	records []quickBooksRecord,
	year int,
) (*quickBooksMappingReview, error) {
	saved, err := b.quickBooksMappings()
	if err != nil {
		return nil, err
	}
	accounts, err := b.PostingAccounts()
	if err != nil {
		return nil, err
	}
	counts := make(map[string]map[string]int)
	for _, record := range records {
		sourceLabel, exists := quickBooksRecordMappingLabel(record)
		if !exists {
			continue
		}
		if counts[sourceLabel.Label] == nil {
			counts[sourceLabel.Label] = make(map[string]int)
		}
		counts[sourceLabel.Label][sourceLabel.Kind]++
	}
	labels := make([]string, 0, len(counts))
	details := make(map[string]string, len(counts))
	for label, kinds := range counts {
		labels = append(labels, label)
		kindNames := make([]string, 0, len(kinds))
		for kind := range kinds {
			kindNames = append(kindNames, kind)
		}
		slices.Sort(kindNames)
		parts := make([]string, len(kindNames))
		for index, kind := range kindNames {
			parts[index] = fmt.Sprintf("%s=%d", kind, kinds[kind])
		}
		details[label] = strings.Join(parts, ", ")
	}
	mappings := make(map[string]string, len(saved))
	original := make(map[string]string, len(saved))
	for _, mapping := range saved {
		mappings[mapping.Label] = mapping.Account
		original[mapping.Label] = mapping.Account
		if _, exists := counts[mapping.Label]; !exists {
			labels = append(labels, mapping.Label)
			details[mapping.Label] = "not present in selected-year sources"
		}
	}
	slices.Sort(labels)
	return &quickBooksMappingReview{
		year: year, labels: labels, details: details,
		mappings: mappings, original: original, accounts: accounts,
	}, nil
}

func (review *quickBooksMappingReview) dirty() bool {
	if len(review.mappings) != len(review.original) {
		return true
	}
	for label, account := range review.mappings {
		if review.original[label] != account {
			return true
		}
	}
	return false
}

func (review *quickBooksMappingReview) savedMappings() []quickBooksAccountMapping {
	mappings := make([]quickBooksAccountMapping, 0, len(review.mappings))
	for label, account := range review.mappings {
		if account != "" {
			mappings = append(mappings, quickBooksAccountMapping{
				Label: label, Account: account,
			})
		}
	}
	slices.SortStableFunc(mappings, compareQuickBooksMappings)
	return mappings
}

func (ui *terminal) quickBooksMappings(review *quickBooksMappingReview) {
	list := themedList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(
		fmt.Sprintf(" QuickBooks %d account mappings ", review.year),
	)
	for _, label := range review.labels {
		label := label
		account := review.mappings[label]
		status := "UNRESOLVED"
		if account != "" {
			status = account
			configured := false
			for _, candidate := range review.accounts {
				if candidate.Name == account {
					configured = true
					break
				}
			}
			if !configured {
				status = "INVALID - remove or remap: " + account
			}
		}
		list.AddItem(
			tview.Escape(label),
			tview.Escape(review.details[label]+" | "+status),
			0,
			func() { ui.quickBooksMappingAccount(review, label) },
		)
	}
	list.AddItem("Save and exit", "Write private quickbooks-mappings.yaml", 's', func() {
		ui.saveQuickBooksMappingReview(review)
	})
	list.AddItem("Exit without saving", "Discard changes made in this review", 'q', func() {
		ui.leaveQuickBooksMappingReview(review)
	})
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyF2:
			ui.saveQuickBooksMappingReview(review)
			return nil
		case tcell.KeyEscape:
			ui.leaveQuickBooksMappingReview(review)
			return nil
		}
		return event
	})
	ui.screen(
		"QuickBooks mapping review",
		list,
		"Enter Map/unmap | F2 Save and exit | Esc Exit",
	)
}

func (ui *terminal) quickBooksMappingAccount(
	review *quickBooksMappingReview,
	label string,
) {
	list := themedList().ShowSecondaryText(true)
	list.SetBorder(true).SetTitle(" Map " + tview.Escape(label) + " ")
	list.AddItem("UNRESOLVED", "Do not permit a deterministic account mapping", 0, func() {
		delete(review.mappings, label)
		ui.quickBooksMappings(review)
	})
	for _, account := range review.accounts {
		account := account
		description := fmt.Sprintf(
			"%s:%s-%s",
			account.FormalCategory,
			account.Number,
			account.FormalName,
		)
		list.AddItem(tview.Escape(account.Name), tview.Escape(description), 0, func() {
			review.mappings[label] = account.Name
			ui.quickBooksMappings(review)
		})
	}
	list.AddItem("Back", "Leave this mapping unchanged", 'q', func() {
		ui.quickBooksMappings(review)
	})
	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEscape {
			ui.quickBooksMappings(review)
			return nil
		}
		return event
	})
	ui.screen("QuickBooks mapping", list, "Enter Select | Esc Back")
}

func (ui *terminal) saveQuickBooksMappingReview(review *quickBooksMappingReview) {
	if err := ui.books.saveQuickBooksMappings(review.savedMappings()); err != nil {
		ui.message(" QuickBooks mappings not saved ", err.Error(), func() {
			ui.quickBooksMappings(review)
		})
		return
	}
	review.original = make(map[string]string, len(review.mappings))
	for label, account := range review.mappings {
		review.original[label] = account
	}
	ui.message(
		" QuickBooks mappings saved ",
		"Saved private mapping policy to "+ui.books.Config.QuickBooksMappings+".",
		ui.app.Stop,
	)
}

func (ui *terminal) leaveQuickBooksMappingReview(review *quickBooksMappingReview) {
	if !review.dirty() {
		ui.app.Stop()
		return
	}
	modal := themedModal().
		SetText("Discard unsaved QuickBooks mapping changes?").
		AddButtons([]string{"Keep editing", "Discard"})
	modal.SetTitle(" Unsaved QuickBooks mappings ").SetBorder(true)
	modal.SetDoneFunc(func(index int, _ string) {
		if index == 1 {
			ui.app.Stop()
			return
		}
		ui.quickBooksMappings(review)
	})
	ui.app.SetRoot(modal, true).SetFocus(modal)
}
