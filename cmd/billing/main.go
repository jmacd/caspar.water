package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/johnfercher/maroto/pkg/color"
	"github.com/johnfercher/maroto/pkg/consts"
	"github.com/johnfercher/maroto/pkg/pdf"
	"github.com/johnfercher/maroto/pkg/props"
)

var (
	usersFile    = flag.String("users", "", "users file csv AcctName,User Name,Address,...")
	metadataFile = flag.String("metadata", "", "file csv")
	accountsFile = flag.String("accounts", "", "file csv")

	cwcColor = color.Color{
		Red:   10,
		Green: 10,
		Blue:  150,
	}
)

type (
	Metadata struct {
		Name    string
		Address string
		Contact string
	}

	User struct {
		AccountName    string
		UserName       string
		ServiceAddress string
		BillingAddress string
	}

	Accounts struct {
		PeriodEnd   string
		Maintenance float64
		Operations  float64
		Utilities   float64
		Insurance   float64
		Taxes       float64
	}
)

func parseAddress(in string) []string {
	out := strings.Split(in, ";")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

func main() {
	err := Main()
	if err != nil {
		log.Println(err)
	}
}

func readAll[T any](name string) ([]T, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", name, err)
	}
	read, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv %s: %w", name, err)
	}
	if len(read) < 2 {
		return nil, fmt.Errorf("not enough rows: %s", name)
	}
	legend := read[0]
	for i := range legend {
		legend[i] = strings.ReplaceAll(legend[i], " ", "")
	}
	rows := read[1:]
	var ret []T
	for _, row := range rows {
		xing := map[string]interface{}{}
		for i, v := range row {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				xing[legend[i]] = v
			} else {
				xing[legend[i]] = f
			}
		}
		data, err := json.Marshal(xing)
		if err != nil {
			return nil, fmt.Errorf("to json %w", err)

		}
		var out T
		if err := json.Unmarshal(data, &out); err != nil {
			return nil, fmt.Errorf("from json %w", err)
		}
		ret = append(ret, out)

	}
	return ret, nil
}

func Main() error {
	flag.Parse()

	// Users
	users, err := readAll[User](*usersFile)
	if err != nil {
		return err
	}
	numUsers := float64(len(users))

	// Metadata
	meta, err := readAll[Metadata](*metadataFile)
	if err != nil {
		return err
	}
	if len(meta) != 1 {
		return fmt.Errorf("metadata file should have one row: %d", len(meta))
	}

	// Accounts
	accounts, err := readAll[Accounts](*accountsFile)
	if err != nil {
		return err
	}
	acct := accounts[len(accounts)-1]
	acct.Maintenance /= numUsers
	acct.Operations /= numUsers
	acct.Utilities /= numUsers
	acct.Taxes /= numUsers
	acct.Insurance /= numUsers

	for _, user := range users {
		fmt.Println("Account", user.AccountName)

		if err := writePDF(meta[0], acct, user); err != nil {
			return fmt.Errorf("write pdf %w", err)
		}
	}
	return nil
}

type lineStyle struct {
	sz    float64
	ht    float64
	top   float64
	txt   consts.Style
	align consts.Align
	color color.Color
}

func (style lineStyle) multiLine(m pdf.Maroto, lines []string) {
	for _, line := range lines {
		m.Row(style.ht, func() {
			m.Col(0, func() {
				m.Text(line, props.Text{
					Size:  style.sz,
					Top:   style.top,
					Style: style.txt,
					Align: style.align,
					Color: style.color,
				})
			})
		})
	}
}

func fmtMoney(f float64) string {
	return fmt.Sprintf("$%.2f", f)
}

func writePDF(meta Metadata, accounts Accounts, user User) error {
	m := pdf.NewMaroto(consts.Portrait, consts.Letter)
	m.SetPageMargins(50, 50, 50)

	const layout = "1/2/2006"
	periodEnd, err := time.Parse(layout, accounts.PeriodEnd)
	if err != nil {
		return err
	}
	periodEnd = periodEnd.Add(24 * time.Hour)
	periodName := fmt.Sprint(periodEnd.Month().String()[:3], "-", periodEnd.Year())

	const bigLine = 5
	const smallLine = 4
	const sepLine = 2

	toStyle := lineStyle{
		sz:    10,
		ht:    bigLine,
		top:   3,
		txt:   consts.Bold,
		align: consts.Left,
		color: color.NewBlack(),
	}

	fromStyle := lineStyle{
		sz:    8,
		ht:    smallLine,
		top:   3,
		txt:   consts.Normal,
		align: consts.Right,
		color: cwcColor,
	}

	normText := props.Text{
		Top:   3,
		Style: consts.Bold,
		Align: consts.Left,
	}

	m.RegisterHeader(func() {
		m.Row(30, func() {
			m.Col(0, func() {
				_ = m.FileImage("assets/img/logo.jpg", props.Rect{
					Percent: 100,
					Center:  true,
				})
			})
		})
		lines := []string{meta.Name}
		lines = append(lines, parseAddress(meta.Address)...)
		lines = append(lines, parseAddress(meta.Contact)...)
		fromStyle.multiLine(m, lines)
	})

	m.Row(sepLine, func() {})

	toStyle.multiLine(m, append([]string{"To:", user.UserName}, parseAddress(user.BillingAddress)...))

	m.Row(10, func() {})

	m.Row(10, func() {
		m.Col(12, func() {
			m.Text("Invoice "+periodName, normText)
		})
	})

	sum := accounts.Maintenance + accounts.Operations + accounts.Utilities + accounts.Insurance + accounts.Taxes

	m.Row(7, func() {
		m.Col(3, func() {
			m.TableList([]string{
				"",
				"",
			}, [][]string{
				{
					"Maintenance",
					fmtMoney(accounts.Maintenance),
				},
				{
					"Operations",
					fmtMoney(accounts.Operations),
				},
				{
					"Utilities",
					fmtMoney(accounts.Utilities),
				},
				{
					"Insurance",
					fmtMoney(accounts.Insurance),
				},
				{
					"Taxes",
					fmtMoney(accounts.Taxes),
				},
			})
		})
	})

	m.Row(7, func() {
		m.Col(3, func() {
			m.Text(fmt.Sprintf("Total: $%.2f", sum), normText)
		})
		m.ColSpace(9)
	})

	return m.OutputFileAndClose(user.AccountName + ".pdf")
}
