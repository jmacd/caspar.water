package wat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"
)

const quickBooksSystemPrompt = `You reconstruct double-entry bookkeeping from exported QuickBooks reports.

Return one JSON object with exactly these fields:
{
  "candidates": [
    {
      "date": "YYYY-MM-DD",
      "payee": "single-line payee",
      "description": "single-line description or empty string",
      "reference": "check/reference number or empty string",
      "confidence": "high|medium|low",
      "postings": [
        {"account": "exact configured account", "amount": "signed decimal USD without $"}
      ]
    }
  ],
  "warnings": ["single-line warning"]
}

The input contains one month of rows gathered from multiple QuickBooks account
reports and a bank register. The same transaction can appear in several reports.
Deduplicate those appearances and reconstruct each real transaction once. Treat
the bank register as the cash-side chronology. Use later journal examples to
learn payee/account mappings, but never copy their dates or amounts.

Use only exact account strings from configured_accounts. Each candidate must
have at least two nonzero postings whose signed amounts total exactly 0.00.
Do not invent transactions, amounts, or accounts. Omit a transaction rather
than inventing missing monetary facts, and explain omissions in warnings.
Produce only candidates in input.year and input.month. Return JSON only.`

type quickBooksAIInput struct {
	Year               int                        `json:"year"`
	Month              string                     `json:"month"`
	ConfiguredAccounts []string                   `json:"configured_accounts"`
	LaterExamples      []quickBooksJournalExample `json:"later_journal_examples"`
	Records            []quickBooksRecord         `json:"quickbooks_records"`
}

type quickBooksAIPosting struct {
	Account string `json:"account"`
	Amount  string `json:"amount"`
}

type quickBooksAICandidate struct {
	Date        string                `json:"date"`
	Payee       string                `json:"payee"`
	Description string                `json:"description"`
	Reference   string                `json:"reference"`
	Confidence  string                `json:"confidence"`
	Postings    []quickBooksAIPosting `json:"postings"`
}

type quickBooksAIResponse struct {
	Candidates []quickBooksAICandidate `json:"candidates"`
	Warnings   []string                `json:"warnings"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func (b *Books) GenerateQuickBooksJournal(
	ctx context.Context,
	paths []string,
	year int,
	progress io.Writer,
) (string, error) {
	if progress == nil {
		progress = io.Discard
	}
	records, err := b.quickBooksRecords(ctx, paths, year, progress)
	if err != nil {
		return "", err
	}
	examples, err := b.quickBooksJournalExamples(year)
	if err != nil {
		return "", fmt.Errorf("load later journal examples: %w", err)
	}
	accounts, err := b.PostingAccounts()
	if err != nil {
		return "", err
	}
	accountNames := make([]string, len(accounts))
	for index, account := range accounts {
		accountNames[index] = account.Name
	}
	byMonth := make(map[string][]quickBooksRecord)
	var months []string
	for _, record := range records {
		month := record.Date[:7]
		if _, exists := byMonth[month]; !exists {
			months = append(months, month)
		}
		byMonth[month] = append(byMonth[month], record)
	}
	slices.Sort(months)

	var journal strings.Builder
	candidateCount := 0
	for _, month := range months {
		fmt.Fprintf(progress,
			"Requesting QuickBooks reconstruction for %s: %d source rows, %d later examples.\n",
			month, len(byMonth[month]), len(examples))
		input := quickBooksAIInput{
			Year: year, Month: month, ConfiguredAccounts: accountNames,
			LaterExamples: examples, Records: byMonth[month],
		}
		response, err := b.quickBooksCompletion(ctx, input)
		if err != nil {
			return "", fmt.Errorf("reconstruct QuickBooks month %s: %w", month, err)
		}
		text, err := renderQuickBooksCandidates(response, input)
		if err != nil {
			return "", fmt.Errorf("validate QuickBooks month %s: %w", month, err)
		}
		for _, warning := range response.Warnings {
			fmt.Fprintf(progress, "QuickBooks reconstruction warning (%s): %s\n", month, warning)
		}
		if text != "" {
			if journal.Len() != 0 {
				journal.WriteByte('\n')
			}
			journal.WriteString(text)
		}
		candidateCount += len(response.Candidates)
	}
	if candidateCount == 0 {
		return "", fmt.Errorf("AI produced no candidate transactions for %d", year)
	}
	fmt.Fprintf(progress, "Generated %d balanced QuickBooks candidates for manual review.\n",
		candidateCount)
	return journal.String(), nil
}

func (b *Books) quickBooksCompletion(
	ctx context.Context,
	input quickBooksAIInput,
) (quickBooksAIResponse, error) {
	settings, err := configuredQuickBooksAI(b.Config.Statement)
	if err != nil {
		return quickBooksAIResponse{}, err
	}
	user, err := json.Marshal(input)
	if err != nil {
		return quickBooksAIResponse{}, err
	}
	messages := []chatMessage{
		{Role: "system", Content: quickBooksSystemPrompt},
		{Role: "user", Content: string(user)},
	}
	content, err := settings.complete(ctx, messages)
	if err != nil {
		return quickBooksAIResponse{}, err
	}
	decoder := json.NewDecoder(strings.NewReader(content))
	decoder.DisallowUnknownFields()
	var response quickBooksAIResponse
	if err := decoder.Decode(&response); err != nil {
		return response, fmt.Errorf("decode model JSON: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return response, fmt.Errorf("model response must contain exactly one JSON object")
	}
	if response.Candidates == nil || response.Warnings == nil {
		return response, fmt.Errorf("model response requires candidates and warnings arrays")
	}
	return response, nil
}

type quickBooksAISettings struct {
	provider   string
	model      string
	apiBase    *url.URL
	apiVersion string
	token      string
}

func configuredQuickBooksAI(config StatementConfig) (quickBooksAISettings, error) {
	apiBase := strings.TrimSpace(config.APIBase)
	model := strings.TrimSpace(config.Model)
	if apiBase == "" && strings.TrimSpace(config.OllamaURL) != "" {
		apiBase = strings.TrimSpace(config.OllamaURL)
		if model != "" && !strings.Contains(model, "/") {
			model = "ollama/" + model
		}
	}
	provider, providerModel, ok := strings.Cut(model, "/")
	if apiBase == "" || !ok || providerModel == "" {
		return quickBooksAISettings{}, fmt.Errorf(
			"configure statement.api_base and a provider-prefixed statement.model",
		)
	}
	if provider != "ollama" && provider != "azure" {
		return quickBooksAISettings{}, fmt.Errorf(
			"statement.model provider must be ollama or azure",
		)
	}
	base, err := url.Parse(apiBase)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") ||
		base.Hostname() == "" || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return quickBooksAISettings{}, fmt.Errorf(
			"statement.api_base must be an HTTP(S) API base URL",
		)
	}
	settings := quickBooksAISettings{
		provider: provider, model: providerModel, apiBase: base,
		apiVersion: strings.TrimSpace(config.APIVersion),
	}
	if provider == "azure" {
		if base.Scheme != "https" ||
			(!strings.HasSuffix(base.Hostname(), ".openai.azure.com") &&
				!strings.HasSuffix(base.Hostname(), ".cognitiveservices.azure.com")) ||
			(base.Path != "" && base.Path != "/") {
			return quickBooksAISettings{}, fmt.Errorf(
				"Azure statement.api_base must use an https://*.openai.azure.com or " +
					"https://*.cognitiveservices.azure.com base URL",
			)
		}
		if settings.apiVersion == "" {
			return quickBooksAISettings{}, fmt.Errorf(
				"statement.api_version is required for Azure",
			)
		}
		if !regexp.MustCompile(`^[A-Za-z0-9._-]+$`).MatchString(providerModel) {
			return quickBooksAISettings{}, fmt.Errorf(
				"Azure statement.model deployment contains unsupported characters",
			)
		}
		settings.token = strings.TrimSpace(os.Getenv("AZURE_AD_TOKEN"))
		if settings.token == "" {
			return quickBooksAISettings{}, fmt.Errorf(
				"AZURE_AD_TOKEN is required for Azure QuickBooks reconstruction",
			)
		}
	}
	return settings, nil
}

func (s quickBooksAISettings) complete(
	ctx context.Context,
	messages []chatMessage,
) (string, error) {
	var endpoint string
	var request any
	if s.provider == "azure" {
		base := strings.TrimRight(s.apiBase.String(), "/")
		endpoint = base + "/openai/deployments/" + url.PathEscape(s.model) +
			"/chat/completions?api-version=" + url.QueryEscape(s.apiVersion)
		request = struct {
			Messages       []chatMessage     `json:"messages"`
			ResponseFormat map[string]string `json:"response_format"`
		}{Messages: messages, ResponseFormat: map[string]string{"type": "json_object"}}
	} else {
		endpoint = strings.TrimRight(s.apiBase.String(), "/") + "/api/chat"
		request = struct {
			Model    string        `json:"model"`
			Messages []chatMessage `json:"messages"`
			Stream   bool          `json:"stream"`
			Format   string        `json:"format"`
		}{Model: s.model, Messages: messages, Format: "json"}
	}
	body, err := json.Marshal(request)
	if err != nil {
		return "", err
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	if s.provider == "azure" {
		httpRequest.Header.Set("Authorization", "Bearer "+s.token)
	}
	response, err := (&http.Client{Timeout: 10 * time.Minute}).Do(httpRequest)
	if err != nil {
		return "", fmt.Errorf("%s model request: %w", s.provider, err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 16<<20))
	if err != nil {
		return "", fmt.Errorf("read %s model response: %w", s.provider, err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("%s model returned %s: %s",
			s.provider, response.Status, strings.TrimSpace(string(responseBody)))
	}
	if s.provider == "azure" {
		var payload struct {
			Choices []struct {
				Message chatMessage `json:"message"`
			} `json:"choices"`
		}
		if err := json.Unmarshal(responseBody, &payload); err != nil {
			return "", fmt.Errorf("decode Azure response: %w", err)
		}
		if len(payload.Choices) != 1 || strings.TrimSpace(payload.Choices[0].Message.Content) == "" {
			return "", fmt.Errorf("Azure response must contain exactly one nonempty choice")
		}
		return payload.Choices[0].Message.Content, nil
	}
	var payload struct {
		Message chatMessage `json:"message"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return "", fmt.Errorf("decode Ollama response: %w", err)
	}
	if strings.TrimSpace(payload.Message.Content) == "" {
		return "", fmt.Errorf("Ollama response contains no message content")
	}
	return payload.Message.Content, nil
}

func renderQuickBooksCandidates(
	response quickBooksAIResponse,
	input quickBooksAIInput,
) (string, error) {
	accounts := make(map[string]bool, len(input.ConfiguredAccounts))
	for _, account := range input.ConfiguredAccounts {
		accounts[account] = true
	}
	sourceAmounts, err := quickBooksSourceAmounts(input.Records)
	if err != nil {
		return "", err
	}
	seen := make(map[string]bool)
	var journal strings.Builder
	for index, candidate := range response.Candidates {
		date, err := time.Parse("2006-01-02", candidate.Date)
		if err != nil || date.Year() != input.Year || candidate.Date[:7] != input.Month {
			return "", fmt.Errorf("candidate %d has date outside %s", index+1, input.Month)
		}
		if candidate.Confidence != "high" &&
			candidate.Confidence != "medium" && candidate.Confidence != "low" {
			return "", fmt.Errorf("candidate %d has invalid confidence %q",
				index+1, candidate.Confidence)
		}
		entry := Entry{
			Date: candidate.Date, Payee: candidate.Payee,
			Description: candidate.Description, Reference: candidate.Reference,
			Postings: make([]Posting, len(candidate.Postings)),
		}
		var identity strings.Builder
		fmt.Fprintf(&identity, "%s\x00%s\x00%s\x00%s",
			entry.Date, entry.Payee, entry.Description, entry.Reference)
		postingIdentities := make([]string, len(candidate.Postings))
		var positiveTotal int64
		for postingIndex, posting := range candidate.Postings {
			if !accounts[posting.Account] {
				return "", fmt.Errorf("candidate %d uses unconfigured account %q",
					index+1, posting.Account)
			}
			cents, err := ParseCents(posting.Amount)
			if err != nil {
				return "", fmt.Errorf("candidate %d posting %d: %w",
					index+1, postingIndex+1, err)
			}
			if cents > 0 {
				if positiveTotal > math.MaxInt64-cents {
					return "", fmt.Errorf("candidate %d debit total exceeds supported USD range",
						index+1)
				}
				positiveTotal += cents
			}
			entry.Postings[postingIndex] = Posting{
				Account: posting.Account, Amount: posting.Amount,
			}
			postingIdentities[postingIndex] = posting.Account + "\x00" + formatCents(cents)
		}
		slices.Sort(postingIdentities)
		for _, posting := range postingIdentities {
			fmt.Fprintf(&identity, "\x00%s", posting)
		}
		if seen[identity.String()] {
			return "", fmt.Errorf("candidate %d duplicates another generated transaction", index+1)
		}
		seen[identity.String()] = true
		id, err := newID()
		if err != nil {
			return "", err
		}
		text, err := entry.render(input.ConfiguredAccounts, id)
		if err != nil {
			return "", fmt.Errorf("candidate %d: %w", index+1, err)
		}
		if len(sourceAmounts) != 0 && !sourceAmounts[positiveTotal] {
			return "", fmt.Errorf(
				"candidate %d total %s is not present in the month's QuickBooks transaction amounts",
				index+1, formatCents(positiveTotal),
			)
		}
		if journal.Len() != 0 {
			journal.WriteByte('\n')
		}
		journal.WriteString(text)
	}
	for index, warning := range response.Warnings {
		if !singleLine(warning) {
			return "", fmt.Errorf("warning %d contains control characters", index+1)
		}
	}
	return journal.String(), nil
}

func quickBooksSourceAmounts(records []quickBooksRecord) (map[int64]bool, error) {
	amounts := make(map[int64]bool)
	for _, record := range records {
		for name, value := range record.Fields {
			switch normalizeQuickBooksHeader(name) {
			case "amount", "payment", "deposit", "debit", "credit":
			default:
				continue
			}
			cents, err := parseQuickBooksCents(value)
			if err != nil {
				return nil, fmt.Errorf("%s field %q: %w", record.ID, name, err)
			}
			if cents == math.MinInt64 {
				return nil, fmt.Errorf("%s field %q amount is out of range", record.ID, name)
			}
			if cents < 0 {
				cents = -cents
			}
			if cents != 0 {
				amounts[cents] = true
			}
		}
	}
	return amounts, nil
}

func parseQuickBooksCents(value string) (int64, error) {
	value = strings.TrimSpace(value)
	negative := strings.HasPrefix(value, "(") && strings.HasSuffix(value, ")")
	if negative {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	value = strings.ReplaceAll(value, ",", "")
	value = strings.TrimSpace(strings.TrimPrefix(value, "$"))
	if strings.HasSuffix(value, "-") {
		negative = true
		value = strings.TrimSpace(strings.TrimSuffix(value, "-"))
	}
	cents, err := ParseCents(value)
	if err != nil {
		return 0, err
	}
	if negative && cents > 0 {
		cents = -cents
	}
	return cents, nil
}
