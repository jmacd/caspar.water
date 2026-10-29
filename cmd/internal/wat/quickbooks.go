package wat

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
)

type quickBooksRecord struct {
	ID      string            `json:"id"`
	Source  string            `json:"source"`
	Row     int               `json:"row"`
	Section string            `json:"section,omitempty"`
	Date    string            `json:"date"`
	Fields  map[string]string `json:"fields"`
}

type quickBooksJournalExample struct {
	Date        string   `json:"date"`
	Description string   `json:"description"`
	Accounts    []string `json:"accounts"`
}

type quickBooksUndatedRow struct {
	Row    int
	Fields []string
}

type quickBooksParseResult struct {
	Records              []quickBooksRecord
	Headers              []string
	TotalRows            int
	NonemptyRows         int
	HeaderRow            int
	RowsBeforeHeader     int
	NonemptyBeforeHeader int
	DataRows             int
	DatedRows            int
	SectionRows          int
	BlankRows            int
	UndatedRows          []quickBooksUndatedRow
	DatesByYear          map[int]int
	EarliestDate         string
	LatestDate           string
}

var quickBooksKnownHeaders = map[string]bool{
	"account": true, "account name": true, "amount": true, "balance": true,
	"cleared": true, "credit": true, "date": true, "debit": true,
	"deposit": true, "description": true,
	"distribution account": true, "memo": true, "name": true, "num": true,
	"number": true, "payee": true, "payment": true, "transaction date": true,
	"transaction type": true, "type": true,
}

func (b *Books) quickBooksRecords(
	ctx context.Context,
	paths []string,
	year int,
	progress io.Writer,
) ([]quickBooksRecord, error) {
	if year < 1900 || year > 9999 {
		return nil, fmt.Errorf("QuickBooks reconstruction year must be a four-digit year")
	}
	var records []quickBooksRecord
	for _, path := range paths {
		data, err := quickBooksInputCSV(ctx, path, progress)
		if err != nil {
			return nil, err
		}
		parsed, err := parseQuickBooksCSVRows(filepath.Base(path), data, year)
		if err != nil {
			return nil, fmt.Errorf("parse QuickBooks input %q: %w", path, err)
		}
		fmt.Fprintf(progress, "QuickBooks input %s: %d transactions dated %d.\n",
			filepath.Base(path), len(parsed.Records), year)
		records = append(records, parsed.Records...)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("QuickBooks inputs contain no transactions dated %d", year)
	}
	slices.SortStableFunc(records, func(a, b quickBooksRecord) int {
		if order := strings.Compare(a.Date, b.Date); order != 0 {
			return order
		}
		return strings.Compare(a.ID, b.ID)
	})
	return records, nil
}

func quickBooksInputCSV(ctx context.Context, path string, progress io.Writer) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect QuickBooks input %q: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("QuickBooks input is not a regular file: %s", path)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read QuickBooks CSV %q: %w", path, err)
		}
		return data, nil
	case ".xls", ".xlsx":
		return quickBooksSpreadsheetCSV(ctx, path, progress)
	default:
		return nil, fmt.Errorf(
			"QuickBooks input must be .xls, .xlsx, or .csv: %s",
			path,
		)
	}
}

func quickBooksSpreadsheetCSV(ctx context.Context, path string, progress io.Writer) ([]byte, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve QuickBooks spreadsheet %q: %w", path, err)
	}
	pond := strings.TrimSpace(os.Getenv("WAT_POND"))
	if pond == "" {
		pond = "pond"
	}
	if _, err := exec.LookPath(pond); err != nil {
		return nil, fmt.Errorf(
			"read QuickBooks spreadsheet: pond is not available (set WAT_POND): %w",
			err,
		)
	}
	escaped := (&url.URL{Path: absolute}).EscapedPath()
	source := "host+xls://" + escaped
	cmd := exec.CommandContext(ctx, pond, "cat", source, "--format=csv")
	cmd.Stderr = progress
	data, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, fmt.Errorf("read QuickBooks spreadsheet %q: %w", path, ctx.Err())
	}
	if err != nil {
		return nil, fmt.Errorf("read QuickBooks spreadsheet %q with pond: %w", path, err)
	}
	return data, nil
}

func parseQuickBooksCSV(source string, data []byte, year int) ([]quickBooksRecord, error) {
	parsed, err := parseQuickBooksCSVRows(source, data, year)
	if err != nil {
		return nil, err
	}
	return parsed.Records, nil
}

func parseQuickBooksCSVRows(
	source string,
	data []byte,
	year int,
) (quickBooksParseResult, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return quickBooksParseResult{}, fmt.Errorf("decode CSV: %w", err)
	}
	headerIndex, err := quickBooksHeaderRow(rows)
	if err != nil {
		return quickBooksParseResult{}, err
	}
	headers := make([]string, len(rows[headerIndex]))
	normalized := make(map[string]bool, len(headers))
	dateColumn := -1
	for index, value := range rows[headerIndex] {
		name := strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
		if name == "" {
			name = "column_" + strconv.Itoa(index+1)
		}
		key := normalizeQuickBooksHeader(name)
		if normalized[key] {
			return quickBooksParseResult{}, fmt.Errorf("header row has duplicate column %q", name)
		}
		normalized[key] = true
		headers[index] = name
		if key == "transaction date" || (key == "date" && dateColumn < 0) {
			dateColumn = index
		}
	}
	if dateColumn < 0 {
		return quickBooksParseResult{}, fmt.Errorf("header row has no transaction date column")
	}
	result := quickBooksParseResult{
		Headers:          headers,
		TotalRows:        len(rows),
		HeaderRow:        headerIndex + 1,
		RowsBeforeHeader: headerIndex,
		DataRows:         len(rows) - headerIndex - 1,
		DatesByYear:      make(map[int]int),
	}
	for index, row := range rows {
		if quickBooksNonemptyCells(row) == 0 {
			continue
		}
		result.NonemptyRows++
		if index < headerIndex {
			result.NonemptyBeforeHeader++
		}
	}
	section := ""
	var earliest, latest time.Time
	for rowIndex, row := range rows[headerIndex+1:] {
		rowNumber := headerIndex + rowIndex + 2
		if len(row) > len(headers) {
			for _, value := range row[len(headers):] {
				if strings.TrimSpace(value) != "" {
					return quickBooksParseResult{}, fmt.Errorf(
						"row %d has a value beyond the %d header columns",
						rowNumber, len(headers),
					)
				}
			}
		}
		values := make([]string, len(headers))
		for index := range values {
			if index < len(row) {
				values[index] = strings.TrimSpace(row[index])
			}
		}
		dateText := values[dateColumn]
		if dateText == "" {
			nonempty := quickBooksNonemptyCells(values)
			if nonempty == 0 {
				result.BlankRows++
				continue
			}
			if value, ok := singleQuickBooksValue(values); ok {
				section = value
				result.SectionRows++
			} else {
				fields := make([]string, 0, nonempty)
				for index, value := range values {
					if value != "" {
						fields = append(fields, headers[index])
					}
				}
				result.UndatedRows = append(result.UndatedRows, quickBooksUndatedRow{
					Row: rowNumber, Fields: fields,
				})
			}
			continue
		}
		date, err := parseQuickBooksDate(dateText)
		if err != nil {
			return quickBooksParseResult{}, fmt.Errorf(
				"row %d transaction date: %w", rowNumber, err,
			)
		}
		result.DatedRows++
		result.DatesByYear[date.Year()]++
		if earliest.IsZero() || date.Before(earliest) {
			earliest = date
		}
		if latest.IsZero() || date.After(latest) {
			latest = date
		}
		if date.Year() != year {
			continue
		}
		fields := make(map[string]string)
		for index, value := range values {
			if value != "" {
				fields[headers[index]] = value
			}
		}
		result.Records = append(result.Records, quickBooksRecord{
			ID:      fmt.Sprintf("%s:%d", source, rowNumber),
			Source:  source,
			Row:     rowNumber,
			Section: section,
			Date:    date.Format("2006-01-02"),
			Fields:  fields,
		})
	}
	if !earliest.IsZero() {
		result.EarliestDate = earliest.Format("2006-01-02")
		result.LatestDate = latest.Format("2006-01-02")
	}
	return result, nil
}

func quickBooksNonemptyCells(row []string) int {
	count := 0
	for _, value := range row {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

func (b *Books) InspectQuickBooks(
	ctx context.Context,
	paths []string,
	year int,
	output io.Writer,
	progress io.Writer,
) error {
	if year < 1900 || year > 9999 {
		return fmt.Errorf("QuickBooks inspection year must be a four-digit year")
	}
	if output == nil {
		output = io.Discard
	}
	if progress == nil {
		progress = io.Discard
	}
	for fileIndex, path := range paths {
		data, err := quickBooksInputCSV(ctx, path, progress)
		if err != nil {
			return err
		}
		parsed, err := parseQuickBooksCSVRows(filepath.Base(path), data, year)
		if err != nil {
			return fmt.Errorf("inspect QuickBooks input %q: %w", path, err)
		}
		if fileIndex != 0 {
			fmt.Fprintln(output)
		}
		fmt.Fprintf(output, "QuickBooks input: %s\n", filepath.Base(path))
		fmt.Fprintf(output, "  normalized CSV: %d rows, %d nonempty, %d columns\n",
			parsed.TotalRows, parsed.NonemptyRows, len(parsed.Headers))
		fmt.Fprintf(output, "  detected header: row %d (%s)\n",
			parsed.HeaderRow, strings.Join(parsed.Headers, " | "))
		fmt.Fprintf(output, "  before header: %d rows, %d nonempty\n",
			parsed.RowsBeforeHeader, parsed.NonemptyBeforeHeader)
		fmt.Fprintf(
			output,
			"  data rows: %d total, %d dated, %d section, %d undated nonempty, %d blank\n",
			parsed.DataRows,
			parsed.DatedRows,
			parsed.SectionRows,
			len(parsed.UndatedRows),
			parsed.BlankRows,
		)
		if parsed.EarliestDate != "" {
			fmt.Fprintf(output, "  date span: %s through %s\n",
				parsed.EarliestDate, parsed.LatestDate)
		}
		years := make([]int, 0, len(parsed.DatesByYear))
		for dateYear := range parsed.DatesByYear {
			years = append(years, dateYear)
		}
		slices.Sort(years)
		fmt.Fprint(output, "  dated rows by year:")
		if len(years) == 0 {
			fmt.Fprint(output, " none")
		}
		for _, dateYear := range years {
			fmt.Fprintf(output, " %d=%d", dateYear, parsed.DatesByYear[dateYear])
		}
		fmt.Fprintln(output)
		fmt.Fprintf(output, "  selected year %d: %d transaction rows\n",
			year, len(parsed.Records))
		for _, row := range parsed.UndatedRows {
			fmt.Fprintf(output, "  undated nonempty row %d: %s\n",
				row.Row, strings.Join(row.Fields, " | "))
		}
	}
	return nil
}

func quickBooksHeaderRow(rows [][]string) (int, error) {
	bestIndex, bestScore := -1, -1
	for index, row := range rows {
		headers := make(map[string]bool)
		for _, value := range row {
			headers[normalizeQuickBooksHeader(value)] = true
		}
		hasDate := headers["transaction date"] || headers["date"]
		hasMoney := headers["amount"] || headers["balance"] ||
			headers["deposit"] || headers["payment"] ||
			headers["debit"] || headers["credit"]
		if !hasDate || !hasMoney {
			continue
		}
		score := 0
		for header := range headers {
			if quickBooksKnownHeaders[header] {
				score++
			}
		}
		if score > bestScore {
			bestIndex, bestScore = index, score
		}
	}
	if bestIndex < 0 {
		return 0, fmt.Errorf(
			"could not find a QuickBooks header containing date and amount/balance columns",
		)
	}
	return bestIndex, nil
}

func normalizeQuickBooksHeader(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimPrefix(value, "\ufeff")), " "))
}

func singleQuickBooksValue(values []string) (string, bool) {
	found := ""
	for _, value := range values {
		if value == "" {
			continue
		}
		if found != "" {
			return "", false
		}
		found = value
	}
	return found, found != ""
}

func parseQuickBooksDate(value string) (time.Time, error) {
	for _, layout := range []string{
		"2006-01-02", "2006-01-02T15:04:05.000", "2006-01-02T15:04:05",
		"1/2/2006", "01/02/2006", "1/2/06", "Jan 2, 2006", "January 2, 2006",
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unsupported date %q", value)
}

func (b *Books) quickBooksJournalExamples(year int) ([]quickBooksJournalExample, error) {
	transactions, err := b.transactions(b.Config.Journal, "--strict")
	if err != nil {
		return nil, err
	}
	var examples []quickBooksJournalExample
	for _, tx := range transactions {
		date, err := time.Parse("2006-01-02", tx.Date)
		if err != nil || date.Year() <= year {
			continue
		}
		accounts := make([]string, 0, len(tx.Postings))
		for _, posting := range tx.Postings {
			if !slices.Contains(accounts, posting.Account) {
				accounts = append(accounts, posting.Account)
			}
		}
		examples = append(examples, quickBooksJournalExample{
			Date: tx.Date, Description: tx.Description, Accounts: accounts,
		})
	}
	if len(examples) > 500 {
		examples = examples[len(examples)-500:]
	}
	return examples, nil
}
