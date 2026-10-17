package wat

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type completionHistory struct {
	payees       []string
	descriptions []string
}

func (b *Books) completionHistory() (completionHistory, error) {
	transactions, err := b.transactions(b.Config.Journal, "--strict")
	if err != nil {
		return completionHistory{}, fmt.Errorf("load transaction completion history: %w", err)
	}
	payees, descriptions := make(map[string]bool), make(map[string]bool)
	for _, transaction := range transactions {
		payee, description, _ := strings.Cut(transaction.Description, "|")
		if value := strings.TrimSpace(payee); value != "" {
			payees[value] = true
		}
		if value := strings.TrimSpace(description); value != "" {
			descriptions[value] = true
		}
	}
	sorted := func(values map[string]bool) []string {
		result := make([]string, 0, len(values))
		for value := range values {
			result = append(result, value)
		}
		sort.Strings(result)
		return result
	}
	return completionHistory{payees: sorted(payees), descriptions: sorted(descriptions)}, nil
}

func matchingText(values []string, query string) []string {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	folded := strings.ToLower(strings.TrimSpace(query))
	var matches []string
	for _, value := range values {
		if value == query {
			return nil
		}
		if strings.Contains(strings.ToLower(value), folded) {
			matches = append(matches, value)
		}
	}
	return matches
}

var accountCodePattern = regexp.MustCompile(`(?:^|:)([0-9]+(?:\.[0-9]+)*)(?:-|\s|$)`)

func matchingAccounts(accounts []string, query string) []string {
	query = strings.TrimSpace(query)
	if query == "" || query[0] < '0' || query[0] > '9' {
		return matchingText(accounts, query)
	}
	var matches []string
	for _, account := range accounts {
		code := accountCodePattern.FindStringSubmatch(account)
		if len(code) > 1 && strings.HasPrefix(code[1], query) {
			matches = append(matches, account)
		}
	}
	return matches
}

func completeInput(input *tview.InputField, match func(string) []string) {
	var matches []string
	input.SetAutocompleteFunc(func(query string) []string {
		matches = match(query)
		entries := make([]string, len(matches))
		for i, text := range matches {
			entries[i] = tview.Escape(text)
		}
		return entries
	}).SetAutocompletedFunc(func(_ string, index, source int) bool {
		if source == tview.AutocompletedNavigate {
			return false
		}
		if index >= 0 && index < len(matches) {
			input.SetText(matches[index])
		}
		return true
	})
	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case tcell.KeyCtrlP:
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		}
		return event
	})
}

func completeAccountInput(input *tview.InputField, accounts []string) *bool {
	browsing := false
	var matches []string
	input.SetAutocompleteFunc(func(query string) []string {
		if browsing {
			matches = append(matches[:0], accounts...)
			if index := slices.Index(matches, query); index > 0 {
				matches[0], matches[index] = matches[index], matches[0]
			}
		} else {
			matches = matchingAccounts(accounts, query)
		}
		entries := make([]string, len(matches))
		for i, text := range matches {
			entries[i] = tview.Escape(text)
		}
		return entries
	}).SetAutocompletedFunc(func(_ string, index, source int) bool {
		if source == tview.AutocompletedNavigate {
			return false
		}
		if index >= 0 && index < len(matches) {
			input.SetText(matches[index])
		}
		browsing = false
		return true
	})
	input.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyCtrlN:
			return tcell.NewEventKey(tcell.KeyDown, 0, tcell.ModNone)
		case tcell.KeyCtrlP:
			return tcell.NewEventKey(tcell.KeyUp, 0, tcell.ModNone)
		case tcell.KeyEnter:
			if !browsing && len(matchingAccounts(accounts, input.GetText())) == 0 {
				browsing = true
				input.Autocomplete()
				return nil
			}
		}
		return event
	})
	return &browsing
}
