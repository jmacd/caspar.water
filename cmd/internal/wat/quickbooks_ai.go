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
      "group_id": "exact input transaction group id",
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
  "unresolved": [
    {
      "group_id": "exact input transaction group id",
      "date": "YYYY-MM-DD",
      "payee": "single-line payee or empty string",
      "description": "single-line description or empty string",
      "reference": "check/reference number or empty string",
      "known_postings": [{
        "account": "exact configured account or empty string",
        "quickbooks_account": "source QuickBooks account label",
        "amount": "signed decimal USD without $"
      }],
      "missing_fact": "single-line description of evidence that is unavailable",
      "question": "single-line question a human must answer"
    }
  ],
  "warnings": ["single-line warning"]
}

The input contains one month of deterministic transaction_groups gathered from
multiple QuickBooks account reports and a bank register. Records already grouped
together are appearances of the same transaction. Never merge groups, split a
group, or redo grouping. A group with ambiguous=true must be unresolved. Treat
the bank register as the cash-side chronology. Use later journal examples to
learn payee/account mappings, but never copy their dates or amounts.

Use only exact account strings from configured_accounts. Each candidate must
have at least two nonzero postings whose signed amounts total exactly 0.00.
account_mappings are human-approved and immutable. If a group contains a mapped
label, use that exact configured account in the candidate postings or unresolved
known_postings. Never substitute another account for a mapped label.
Use an account from bank_accounts only when at least one record in that group
has cash_source=true. A QuickBooks account report, payee,
check number, or plausible date is not evidence that a configured bank account
was involved. If no cash-source row is cited, preserve the supported noncash
side in unresolved and ask for the missing counterposting.
Do not invent transactions, amounts, or accounts. Preserve every distinct
source amount in the candidate postings or unresolved known_postings. When
monetary or accounting facts are missing, put the transaction in unresolved,
preserving all supported business-side postings and asking for the missing
counterposting.
Warnings are for batch-level notices, never omitted transactions.

Every transaction_groups id must appear exactly once across candidates and
unresolved. Use only group ids present in the input. Produce only dispositions
in input.year and input.month. Return JSON only.`

type quickBooksAIInput struct {
	Year               int                          `json:"year"`
	Month              string                       `json:"month"`
	ConfiguredAccounts []string                     `json:"configured_accounts"`
	BankAccounts       []string                     `json:"bank_accounts"`
	AccountMappings    []quickBooksAccountMapping   `json:"account_mappings"`
	LaterExamples      []quickBooksJournalExample   `json:"later_journal_examples"`
	Groups             []quickBooksTransactionGroup `json:"transaction_groups"`
}

type quickBooksAIPosting struct {
	Account string `json:"account"`
	Amount  string `json:"amount"`
}

type quickBooksAICandidate struct {
	GroupID     string                `json:"group_id"`
	Date        string                `json:"date"`
	Payee       string                `json:"payee"`
	Description string                `json:"description"`
	Reference   string                `json:"reference"`
	Confidence  string                `json:"confidence"`
	Postings    []quickBooksAIPosting `json:"postings"`
}

type quickBooksAIKnownPosting struct {
	Account           string `json:"account"`
	QuickBooksAccount string `json:"quickbooks_account"`
	Amount            string `json:"amount"`
}

type quickBooksAIUnresolved struct {
	GroupID       string                     `json:"group_id"`
	Date          string                     `json:"date"`
	Payee         string                     `json:"payee"`
	Description   string                     `json:"description"`
	Reference     string                     `json:"reference"`
	KnownPostings []quickBooksAIKnownPosting `json:"known_postings"`
	MissingFact   string                     `json:"missing_fact"`
	Question      string                     `json:"question"`
}

type quickBooksAIResponse struct {
	Candidates []quickBooksAICandidate  `json:"candidates"`
	Unresolved []quickBooksAIUnresolved `json:"unresolved"`
	Warnings   []string                 `json:"warnings"`
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
	groups, err := groupQuickBooksRecords(records)
	if err != nil {
		return "", fmt.Errorf("group QuickBooks transactions: %w", err)
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
	bankAccounts := make([]string, len(b.Config.BankAccounts))
	for index, bank := range b.Config.BankAccounts {
		bankAccounts[index] = bank.Account
	}
	accountMappings, err := b.quickBooksMappings()
	if err != nil {
		return "", err
	}
	if err := b.validateQuickBooksMappings(accountMappings); err != nil {
		return "", err
	}
	byMonth := make(map[string][]quickBooksTransactionGroup)
	var months []string
	for _, group := range groups {
		month := group.Date[:7]
		if _, exists := byMonth[month]; !exists {
			months = append(months, month)
		}
		byMonth[month] = append(byMonth[month], group)
	}
	slices.Sort(months)

	var journal strings.Builder
	candidateCount := 0
	unresolvedCount := 0
	for _, month := range months {
		sourceRows := 0
		for _, group := range byMonth[month] {
			sourceRows += len(group.Records)
		}
		fmt.Fprintf(progress,
			"Requesting QuickBooks reconstruction for %s: %d source rows in %d "+
				"deterministic groups, %d later examples.\n",
			month, sourceRows, len(byMonth[month]), len(examples))
		input := quickBooksAIInput{
			Year: year, Month: month, ConfiguredAccounts: accountNames,
			BankAccounts: bankAccounts, AccountMappings: accountMappings,
			LaterExamples: examples, Groups: byMonth[month],
		}
		response, err := b.quickBooksCompletion(ctx, input)
		if err != nil {
			return "", fmt.Errorf("reconstruct QuickBooks month %s: %w", month, err)
		}
		text, err := renderQuickBooksCandidates(response, input)
		if err != nil {
			fmt.Fprintf(
				progress,
				"QuickBooks reconstruction response invalid (%s): %v; "+
					"requesting one correction.\n",
				month,
				err,
			)
			response, err = b.quickBooksCorrection(ctx, input, response, err)
			if err != nil {
				return "", fmt.Errorf("correct QuickBooks month %s: %w", month, err)
			}
			text, err = renderQuickBooksCandidates(response, input)
			if err != nil {
				return "", fmt.Errorf(
					"validate corrected QuickBooks month %s: %w",
					month,
					err,
				)
			}
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
		unresolvedCount += len(response.Unresolved)
	}
	if candidateCount == 0 && unresolvedCount == 0 {
		return "", fmt.Errorf("AI produced no transaction dispositions for %d", year)
	}
	fmt.Fprintf(
		progress,
		"Generated %d balanced QuickBooks candidates and %d unresolved entry skeletons "+
			"for manual review.\n",
		candidateCount,
		unresolvedCount,
	)
	return journal.String(), nil
}

func (b *Books) quickBooksCompletion(
	ctx context.Context,
	input quickBooksAIInput,
) (quickBooksAIResponse, error) {
	user, err := json.Marshal(input)
	if err != nil {
		return quickBooksAIResponse{}, err
	}
	messages := []chatMessage{
		{Role: "system", Content: quickBooksSystemPrompt},
		{Role: "user", Content: string(user)},
	}
	return b.quickBooksCompletionMessages(ctx, messages)
}

func (b *Books) quickBooksCorrection(
	ctx context.Context,
	input quickBooksAIInput,
	previous quickBooksAIResponse,
	validationErr error,
) (quickBooksAIResponse, error) {
	user, err := json.Marshal(input)
	if err != nil {
		return quickBooksAIResponse{}, err
	}
	assistant, err := json.Marshal(previous)
	if err != nil {
		return quickBooksAIResponse{}, err
	}
	messages := []chatMessage{
		{Role: "system", Content: quickBooksSystemPrompt},
		{Role: "user", Content: string(user)},
		{Role: "assistant", Content: string(assistant)},
		{
			Role: "user",
			Content: "Your previous response failed validation: " +
				validationErr.Error() +
				". Return one complete corrected JSON object covering every input " +
				"transaction group exactly once. Do not return commentary.",
		},
	}
	return b.quickBooksCompletionMessages(ctx, messages)
}

func (b *Books) quickBooksCompletionMessages(
	ctx context.Context,
	messages []chatMessage,
) (quickBooksAIResponse, error) {
	settings, err := configuredQuickBooksAI(b.Config.Statement)
	if err != nil {
		return quickBooksAIResponse{}, err
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
	if response.Candidates == nil || response.Unresolved == nil || response.Warnings == nil {
		return response, fmt.Errorf(
			"model response requires candidates, unresolved, and warnings arrays",
		)
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
	bankAccounts := make(map[string]bool, len(input.BankAccounts))
	for _, account := range input.BankAccounts {
		if !accounts[account] {
			return "", fmt.Errorf("bank account %q is not a configured account", account)
		}
		bankAccounts[account] = true
	}
	mappingLabels := make(map[string]bool, len(input.AccountMappings))
	for _, mapping := range input.AccountMappings {
		if strings.TrimSpace(mapping.Label) != mapping.Label ||
			mapping.Label == "" || !singleLine(mapping.Label) {
			return "", fmt.Errorf("QuickBooks input contains an invalid mapped label")
		}
		if mappingLabels[mapping.Label] {
			return "", fmt.Errorf("QuickBooks input maps label %q more than once", mapping.Label)
		}
		mappingLabels[mapping.Label] = true
		if !accounts[mapping.Account] {
			return "", fmt.Errorf(
				"QuickBooks input maps label %q to unconfigured account %q",
				mapping.Label,
				mapping.Account,
			)
		}
	}
	groupsByID := make(map[string]quickBooksTransactionGroup, len(input.Groups))
	sourceIDs := make(map[string]bool)
	for _, group := range input.Groups {
		if group.ID == "" || !singleLine(group.ID) {
			return "", fmt.Errorf("QuickBooks input contains an invalid group id")
		}
		if _, exists := groupsByID[group.ID]; exists {
			return "", fmt.Errorf("QuickBooks input contains duplicate group id %q", group.ID)
		}
		if len(group.Records) == 0 {
			return "", fmt.Errorf("QuickBooks group %q contains no source records", group.ID)
		}
		for _, record := range group.Records {
			if record.ID == "" || !singleLine(record.ID) {
				return "", fmt.Errorf("QuickBooks input contains an invalid source id")
			}
			if sourceIDs[record.ID] {
				return "", fmt.Errorf("QuickBooks input reuses source id %q across groups", record.ID)
			}
			if record.Date != group.Date {
				return "", fmt.Errorf(
					"QuickBooks group %q date %s disagrees with source %q date %s",
					group.ID,
					group.Date,
					record.ID,
					record.Date,
				)
			}
			sourceIDs[record.ID] = true
		}
		groupsByID[group.ID] = group
	}
	claimed := make(map[string]string, len(input.Groups))
	claimGroup := func(groupID, disposition string) (quickBooksTransactionGroup, error) {
		if groupID == "" {
			return quickBooksTransactionGroup{}, fmt.Errorf("%s cites no transaction group", disposition)
		}
		group, exists := groupsByID[groupID]
		if !exists {
			return quickBooksTransactionGroup{}, fmt.Errorf(
				"%s cites unknown transaction group %q",
				disposition,
				groupID,
			)
		}
		if prior, exists := claimed[groupID]; exists {
			return quickBooksTransactionGroup{}, fmt.Errorf(
				"%s reuses transaction group %q already claimed by %s",
				disposition,
				groupID,
				prior,
			)
		}
		claimed[groupID] = disposition
		return group, nil
	}
	seen := make(map[string]bool)
	var journal strings.Builder
	for index, candidate := range response.Candidates {
		disposition := fmt.Sprintf("candidate %d", index+1)
		group, err := claimGroup(candidate.GroupID, disposition)
		if err != nil {
			return "", err
		}
		if group.Ambiguous {
			return "", fmt.Errorf(
				"%s uses ambiguous transaction group %q; it must remain unresolved",
				disposition,
				group.ID,
			)
		}
		sourceRecords := group.Records
		requiredAccounts, err := quickBooksMappedAccounts(group, input.AccountMappings)
		if err != nil {
			return "", err
		}
		date, err := time.Parse("2006-01-02", candidate.Date)
		if err != nil || date.Year() != input.Year || candidate.Date[:7] != input.Month {
			return "", fmt.Errorf("%s has date outside %s", disposition, input.Month)
		}
		if candidate.Date != group.Date {
			return "", fmt.Errorf(
				"%s date %s disagrees with transaction group %q date %s",
				disposition,
				candidate.Date,
				group.ID,
				group.Date,
			)
		}
		if candidate.Confidence != "high" &&
			candidate.Confidence != "medium" && candidate.Confidence != "low" {
			return "", fmt.Errorf("%s has invalid confidence %q",
				disposition, candidate.Confidence)
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
		postingAmounts := make(map[int64]bool, len(candidate.Postings))
		candidateAccounts := make(map[string]bool, len(candidate.Postings))
		var positiveTotal int64
		for postingIndex, posting := range candidate.Postings {
			if !accounts[posting.Account] {
				return "", fmt.Errorf("%s uses unconfigured account %q",
					disposition, posting.Account)
			}
			candidateAccounts[posting.Account] = true
			cents, err := ParseCents(posting.Amount)
			if err != nil {
				return "", fmt.Errorf("%s posting %d: %w",
					disposition, postingIndex+1, err)
			}
			if cents > 0 {
				if positiveTotal > math.MaxInt64-cents {
					return "", fmt.Errorf("%s debit total exceeds supported USD range", disposition)
				}
				positiveTotal += cents
			}
			absoluteCents := cents
			if absoluteCents < 0 {
				if absoluteCents == math.MinInt64 {
					return "", fmt.Errorf("%s posting %d amount is out of range",
						disposition, postingIndex+1)
				}
				absoluteCents = -absoluteCents
			}
			if absoluteCents != 0 {
				postingAmounts[absoluteCents] = true
			}
			if bankAccounts[posting.Account] && !quickBooksCashEvidence(sourceRecords) {
				return "", fmt.Errorf(
					"%s posts to bank account %q without a cited cash-source row",
					disposition,
					posting.Account,
				)
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
			return "", fmt.Errorf("%s duplicates another generated transaction", disposition)
		}
		seen[identity.String()] = true
		id, err := newID()
		if err != nil {
			return "", err
		}
		text, err := entry.render(input.ConfiguredAccounts, id)
		if err != nil {
			return "", fmt.Errorf("%s: %w", disposition, err)
		}
		sourceAmounts, err := quickBooksSourceAmounts(sourceRecords)
		if err != nil {
			return "", err
		}
		if len(sourceAmounts) == 1 && !sourceAmounts[positiveTotal] {
			return "", fmt.Errorf(
				"%s total %s is not present in its cited QuickBooks transaction amounts",
				disposition, formatCents(positiveTotal),
			)
		}
		for sourceAmount := range sourceAmounts {
			if !postingAmounts[sourceAmount] {
				return "", fmt.Errorf(
					"%s omits cited QuickBooks transaction amount %s",
					disposition,
					formatCents(sourceAmount),
				)
			}
		}
		for requiredAccount := range requiredAccounts {
			if !candidateAccounts[requiredAccount] {
				return "", fmt.Errorf(
					"%s omits human-approved mapped account %q",
					disposition,
					requiredAccount,
				)
			}
		}
		if journal.Len() != 0 {
			journal.WriteByte('\n')
		}
		journal.WriteString(text)
	}
	if len(response.Unresolved) != 0 {
		if journal.Len() != 0 {
			journal.WriteByte('\n')
		}
	}
	for index, unresolved := range response.Unresolved {
		disposition := fmt.Sprintf("unresolved item %d", index+1)
		group, err := claimGroup(unresolved.GroupID, disposition)
		if err != nil {
			return "", err
		}
		sourceRecords := group.Records
		requiredAccounts, err := quickBooksMappedAccounts(group, input.AccountMappings)
		if err != nil {
			return "", err
		}
		date, err := time.Parse("2006-01-02", unresolved.Date)
		if err != nil || date.Year() != input.Year || unresolved.Date[:7] != input.Month {
			return "", fmt.Errorf("%s has date outside %s", disposition, input.Month)
		}
		if unresolved.Date != group.Date {
			return "", fmt.Errorf(
				"%s date %s disagrees with transaction group %q date %s",
				disposition,
				unresolved.Date,
				group.ID,
				group.Date,
			)
		}
		for name, value := range map[string]string{
			"payee": unresolved.Payee, "description": unresolved.Description,
			"reference": unresolved.Reference, "missing fact": unresolved.MissingFact,
			"question": unresolved.Question,
		} {
			if !singleLine(value) {
				return "", fmt.Errorf("%s %s contains control characters", disposition, name)
			}
		}
		if strings.TrimSpace(unresolved.MissingFact) == "" ||
			strings.TrimSpace(unresolved.Question) == "" {
			return "", fmt.Errorf("%s requires a missing fact and question", disposition)
		}
		if len(unresolved.KnownPostings) == 0 {
			return "", fmt.Errorf("%s has no known postings", disposition)
		}
		knownCents := make([]int64, len(unresolved.KnownPostings))
		knownAmounts := make(map[int64]bool, len(unresolved.KnownPostings))
		knownAccounts := make(map[string]bool, len(unresolved.KnownPostings))
		var knownTotal int64
		for postingIndex, posting := range unresolved.KnownPostings {
			if !singleLine(posting.QuickBooksAccount) {
				return "", fmt.Errorf(
					"%s known posting %d QuickBooks account contains control characters",
					disposition,
					postingIndex+1,
				)
			}
			if posting.Account == "" && strings.TrimSpace(posting.QuickBooksAccount) == "" {
				return "", fmt.Errorf(
					"%s known posting %d has no account label",
					disposition,
					postingIndex+1,
				)
			}
			if posting.Account != "" && !accounts[posting.Account] {
				return "", fmt.Errorf("%s uses unconfigured known account %q",
					disposition, posting.Account)
			}
			if posting.Account != "" {
				knownAccounts[posting.Account] = true
			}
			if bankAccounts[posting.Account] && !quickBooksCashEvidence(sourceRecords) {
				return "", fmt.Errorf(
					"%s presents bank account %q without a cited cash-source row",
					disposition,
					posting.Account,
				)
			}
			cents, err := ParseCents(posting.Amount)
			if err != nil || cents == 0 {
				return "", fmt.Errorf("%s has invalid known posting amount %q",
					disposition, posting.Amount)
			}
			if (cents > 0 && knownTotal > math.MaxInt64-cents) ||
				(cents < 0 && knownTotal < math.MinInt64-cents) {
				return "", fmt.Errorf("%s known posting total is out of range", disposition)
			}
			knownTotal += cents
			knownCents[postingIndex] = cents
			absoluteKnown := cents
			if absoluteKnown < 0 {
				if absoluteKnown == math.MinInt64 {
					return "", fmt.Errorf("%s known posting amount is out of range", disposition)
				}
				absoluteKnown = -absoluteKnown
			}
			knownAmounts[absoluteKnown] = true
		}
		if knownTotal == math.MinInt64 {
			return "", fmt.Errorf("%s known posting total is out of range", disposition)
		}
		sourceAmounts, err := quickBooksSourceAmounts(sourceRecords)
		if err != nil {
			return "", err
		}
		if len(sourceRecords) != 0 {
			for knownAmount := range knownAmounts {
				if !sourceAmounts[knownAmount] {
					return "", fmt.Errorf(
						"%s amount %s is not present in its cited QuickBooks transaction amounts",
						disposition, formatCents(knownAmount),
					)
				}
			}
			for requiredAccount := range requiredAccounts {
				if !knownAccounts[requiredAccount] {
					return "", fmt.Errorf(
						"%s omits human-approved mapped account %q",
						disposition,
						requiredAccount,
					)
				}
			}
			for sourceAmount := range sourceAmounts {
				if !knownAmounts[sourceAmount] {
					return "", fmt.Errorf(
						"%s omits cited QuickBooks transaction amount %s",
						disposition,
						formatCents(sourceAmount),
					)
				}
			}
		}
		if journal.Len() != 0 && journal.String()[journal.Len()-1] != '\n' {
			journal.WriteByte('\n')
		}
		if index != 0 {
			journal.WriteByte('\n')
		}
		journal.WriteString(renderQuickBooksUnresolved(
			unresolved,
			knownCents,
			knownTotal,
			quickBooksGroupSourceIDs(group),
		))
	}
	for _, group := range input.Groups {
		if _, exists := claimed[group.ID]; !exists {
			return "", fmt.Errorf(
				"QuickBooks transaction group %q has no candidate or unresolved disposition",
				group.ID,
			)
		}
	}
	for index, warning := range response.Warnings {
		if !singleLine(warning) {
			return "", fmt.Errorf("warning %d contains control characters", index+1)
		}
	}
	return journal.String(), nil
}

func renderQuickBooksUnresolved(
	unresolved quickBooksAIUnresolved,
	knownCents []int64,
	knownTotal int64,
	sourceIDs []string,
) string {
	var output strings.Builder
	quickBooksAccounts := make([]string, 0, len(unresolved.KnownPostings))
	for _, posting := range unresolved.KnownPostings {
		if posting.QuickBooksAccount != "" {
			quickBooksAccounts = append(
				quickBooksAccounts,
				posting.QuickBooksAccount,
			)
		}
	}
	fmt.Fprintf(&output, "; Sources: %s", strings.Join(sourceIDs, ", "))
	if len(quickBooksAccounts) != 0 {
		fmt.Fprintf(
			&output,
			" | QuickBooks: %s",
			strings.Join(quickBooksAccounts, ", "),
		)
	}
	fmt.Fprintf(
		&output,
		" | Missing: %s | Question: %s\n;\n",
		unresolved.MissingFact,
		unresolved.Question,
	)
	fmt.Fprintf(&output, "%s !", unresolved.Date)
	if unresolved.Payee == "" &&
		unresolved.Description == "" &&
		unresolved.Reference == "" {
		output.WriteString(" TODO")
	}
	if unresolved.Payee != "" {
		fmt.Fprintf(&output, " %s", unresolved.Payee)
	}
	if unresolved.Description != "" {
		fmt.Fprintf(&output, " | %s", unresolved.Description)
	}
	if unresolved.Reference != "" {
		fmt.Fprintf(&output, " (%s)", unresolved.Reference)
	}
	output.WriteByte('\n')
	for index, posting := range unresolved.KnownPostings {
		account := posting.Account
		if account == "" {
			account = fmt.Sprintf("TODO:Account-%d", index+1)
		}
		fmt.Fprintf(
			&output,
			"    [%s]  $%s\n",
			account,
			formatCents(knownCents[index]),
		)
	}
	if knownTotal != 0 {
		fmt.Fprintf(
			&output,
			"    [TODO:Counterposting]  $%s\n",
			formatCents(-knownTotal),
		)
	}
	return output.String()
}

func quickBooksCashEvidence(records []quickBooksRecord) bool {
	for _, record := range records {
		if record.CashSource {
			return true
		}
	}
	return false
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
