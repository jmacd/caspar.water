package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/johnfercher/maroto/pkg/color"
	"github.com/johnfercher/maroto/pkg/consts"
	"github.com/johnfercher/maroto/pkg/pdf"
	"github.com/johnfercher/maroto/pkg/props"
)

var (
	// These three files contain private data, are not kept in
	// this repository.
	usersFile    = flag.String("users", "users.csv", "users file csv AcctName,User Name,Address,...")
	metadataFile = flag.String("metadata", "metadata.csv", "file csv")
	accountsFile = flag.String("accounts", "accounts.csv", "file csv")
	outputDir    = flag.String("output", "output", "output directory")

	// Caspar water is blue!
	cwcColor = color.Color{
		Red:   10,
		Green: 10,
		Blue:  150,
	}

	dollarsAndCentsRe = regexp.MustCompile(`\$(\d+(?:,\d\d\d)*)\.(\d\d)`)
)

const (
	csvLayout = "1/2/2006"

	// maxConnections is how many connections we can reach,
	// excluding the one that is not viable (so that with that
	// connection maxConnections would be 14).  Limit is 14.
	maxConnections = 13

	// communityCenterAdjustedUserCount is the target effective
	// user count for the CC used for billing after the initial
	// adjustment, which gives the CC double weight.
	communityCenterAdjustedUserCount = 2

	// communityCenterAdjustment is how many effective
	// users will be added after the CC adjustment is applied.
	communityCenterAdjustment = (communityCenterAdjustedUserCount - 1)

	// communityCenterAccount is the account name for the
	// community center used to carry out the adjustment.
	communityCenterAccount = "CommCtr"

	initialMargin       = 0.0
	targetMargin        = 0.2
	marginIncreaseYears = 2
	statementsPerYear   = 2
)

type (
	// Metadata describes the billing entity and other static
	// information.
	Metadata struct {
		// Name is how to make the payment.
		Name string

		// Address is where to send the payment.
		Address string

		// Contact is how and with whom to discuss the payment.
		Contact string
	}

	// User describes one account for payment.
	User struct {
		// AccountName is an internal identifier, descriptive
		// for the company but does not meaningful on the
		// bill.
		AccountName string

		// UserName is the responsible party's name.
		UserName string

		// ServiceAddress is the location of water service.
		ServiceAddress string

		// BillingAddress is where the user receives mail.
		BillingAddress string
	}

	// Accounts describes the cost of doing business.
	Accounts struct {
		// PeriodEnd is the end of the billing cycle.
		PeriodEnd string

		// Operations includes treatment, chemicals, and lab analysis.
		Operations money.Money

		// Utilities includes electricity.
		Utilities money.Money

		// Insurance is general liability for the water company.
		Insurance money.Money

		// Taxes are property taxes, business licensing, and
		// certification costs.
		Taxes money.Money

		// Method describes the billing method, values include:
		// - Baseline: the initial condition has no reserve.
		// - FirstAdjustment: a billing cycle where the CommCtr
		//   doubles in weight and the first cost-of-living
		//   adjustment is applied.
		Method string

		// endDate is the parsed PeriodEnd.
		periodEnd time.Time

		// periodName is computed from PeriodEnd.
		periodName string

		// dirPath is the directory where PDFs are written.
		dirPath string
	}

	// Billing is the billing state that evolves from one period
	// to the next, including cumulative cost-of-living adjustments
	// and introductory reweighting.
	Billing struct {
		// effectiveUserCount is maxConnections at the baseline.
		effectiveUserCount int

		// communityCenterCount is 1 at the baseline.
		communityCenterCount int

		// savingsRate is 1 + margin.
		savingsRate float64

		// adjustments counts the number of adjustments.
		adjustments int
	}
)

func newBilling() *Billing {
	return &Billing{
		effectiveUserCount:   maxConnections,
		communityCenterCount: 1,
		savingsRate:          1 + initialMargin,
	}
}

// parseAddress splits a semicolon-delimited multiline address.
func parseAddress(in string) []string {
	out := strings.Split(in, ";")
	for i := range out {
		out[i] = strings.TrimSpace(out[i])
	}
	return out
}

func (acct *Accounts) parsePeriod() (err error) {
	acct.periodEnd, err = time.Parse(csvLayout, acct.PeriodEnd)
	if err != nil {
		return err
	}
	billDate := acct.periodEnd.Add(24 * time.Hour)
	acct.periodName = fmt.Sprint(billDate.Month().String()[:3], "-", billDate.Year())

	acct.dirPath = path.Join(*outputDir, acct.periodName)
	if err := os.MkdirAll(acct.dirPath, 0777); err != nil {
		return fmt.Errorf("mkdir: %s: %w", acct.dirPath, err)
	}
	return nil
}

// sumMoney computes a money sum.
func sumMoney(inputs ...money.Money) money.Money {
	total := money.New(0, money.USD)
	for i := range inputs {
		total, _ = total.Add(&inputs[i])
	}
	return *total
}

func main() {
	b := newBilling()
	err := b.Main()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// readAll converts a CSV file into a list of T structs (all defined
// above), where the first CSV row matches field names.  This is done
// via an intermediate JSON representation.  All fields are either
// strings or $Dollars.Cents.
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
			// Note: go-money parses money fields, we
			// format the generic JSON struct they expect.
			parts := dollarsAndCentsRe.FindStringSubmatch(v)
			var value interface{}
			if parts == nil {
				value = v
			} else {
				dollars, err := strconv.Atoi(strings.ReplaceAll(parts[1], ",", ""))
				if err != nil {
					return nil, err
				}
				cents, err := strconv.Atoi(parts[2])
				if err != nil {
					return nil, err
				}
				// go-money expects two fields, one float64, one string.
				value = map[string]interface{}{
					"amount":   float64(dollars*100 + cents),
					"currency": "USD",
				}
			}
			xing[legend[i]] = value
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

func (b *Billing) getPayment(name string, payments []*money.Money) (money.Money, float64, int, []*money.Money) {
	pay := *payments[0]
	payments = payments[1:]
	weight := 1

	if name == communityCenterAccount {
		weight = b.communityCenterCount
	}

	fraction := float64(weight) / float64(b.effectiveUserCount)
	return pay, fraction, weight, payments
}

func (b *Billing) Main() error {
	flag.Parse()

	// Users
	users, err := readAll[User](*usersFile)
	if err != nil {
		return err
	}

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

	// Every other period we split the taxes and insurance, which
	// are yearly expenses paid during the October-March period.
	for acctNo := 0; acctNo < len(accounts); acctNo += 2 {
		yearlyTax, _ := accounts[acctNo].Taxes.Split(2)
		yearlyIns, _ := accounts[acctNo].Insurance.Split(2)

		accounts[acctNo].Taxes = *yearlyTax[0]
		accounts[acctNo].Insurance = *yearlyIns[0]

		// The final period will be missing every other cycle.
		if acctNo+1 < len(accounts) {
			accounts[acctNo+1].Taxes = *yearlyTax[1]
			accounts[acctNo+1].Insurance = *yearlyIns[1]
		}
	}

	for cycleNo := range accounts {
		acct := &accounts[cycleNo]
		if err := acct.parsePeriod(); err != nil {
			return err
		}

		switch acct.Method {
		case "Baseline":
			// No adjustments

		case "InitialAdjustment":
			b.communityCenterCount = communityCenterAdjustedUserCount
			b.effectiveUserCount += communityCenterAdjustment
			// In a different universe, this fallthrough
			// might matter.  It doesn't in our universe
			// because Caspar's InitialAdjustment happens
			// in an odd cycle.
			fallthrough

		case "NormalAdjustment":
			b.adjustments++

			// The margin updates every other period, up
			// until the number of statements required to
			// reach the target margin.
			if cycleNo%2 == 1 && b.adjustments < marginIncreaseYears*statementsPerYear {
				ratio := float64(b.adjustments) / (marginIncreaseYears * statementsPerYear)
				b.savingsRate = 1 + ratio*targetMargin
			}
		default:
			panic(fmt.Sprintf("Unknown accounting method for %s: %s", acct.periodName, acct.Method))
		}

		expenses := sumMoney(
			acct.Operations,
			acct.Utilities,
			acct.Taxes,
			acct.Insurance,
		)

		total := money.New(int64(float64(expenses.Amount())*b.savingsRate), money.USD)

		fmt.Printf("Billing cycle %v expenses %v savingsRate %.3f\n", acct.periodName, expenses.Display(), b.savingsRate)

		payments, err := total.Split(b.effectiveUserCount)
		if err != nil {
			return err
		}

		// Deterministically shuffle the $0.01 rounding
		// differences so they are shared by different users.
		rand.New(rand.NewSource(acct.periodEnd.UnixNano())).Shuffle(len(payments), func(i, j int) {
			payments[i], payments[j] = payments[j], payments[i]
		})

		for userNo := range users {
			user := &users[userNo]

			pay, fraction, weight, reduced := b.getPayment(user.AccountName, payments)
			payments = reduced

			pdfPath := path.Join(acct.dirPath, user.AccountName+".pdf")

			if err := b.writePDF(&meta[0], acct, user, fraction, weight, *total, pay, pdfPath); err != nil {
				return fmt.Errorf("write pdf %w", err)
			}
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

func (b *Billing) writePDF(meta *Metadata, acct *Accounts, user *User, fraction float64, weight int, total, pay money.Money, outputPath string) error {
	m := pdf.NewMaroto(consts.Portrait, consts.Letter)
	m.SetPageMargins(50, 50, 50)

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

	boldText := props.Text{
		Top:   3,
		Style: consts.Bold,
		Align: consts.Left,
	}

	normText := props.Text{
		Top:   3,
		Align: consts.Left,
	}

	tableStyle := props.TableList{
		Align: consts.Right,
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
			m.Text("Invoice "+acct.periodName, boldText)
		})
	})

	pctStr := fmt.Sprintf("%.2f%%", fraction*100)
	fracStr := fmt.Sprintf("%.4f", fraction)

	m.Row(10, func() {
		m.Text(
			fmt.Sprintf(
				"Your bill represents %v of the semi-annual cost of operating the Caspar water system.",
				pctStr,
			),
			normText,
		)
	})
	if b.savingsRate != 1 {
		m.Row(10, func() {
			m.Text(
				fmt.Sprintf(
					"An additional operating margin of %.0f%% has been applied to cover maintenance costs.",
					(b.savingsRate-1)*100,
				),
				normText,
			)
		})
	}

	m.Row(10, func() {})

	m.Row(7, func() {
		m.TableList([]string{
			"Expense",
			"Cost",
			"",
		}, [][]string{
			{
				"Operations",
				acct.Operations.Display(),
			},
			{
				"Utilities",
				acct.Utilities.Display(),
			},
			{
				"Insurance",
				acct.Insurance.Display(),
			},
			{
				"Taxes",
				acct.Taxes.Display(),
			},
			{},
			{
				"Subtotal",
				total.Display(),
			},
			{
				"Fraction",
				"× " + fracStr,
			},
			{
				"",
				"",
			},
			{
				"Your payment",
				pay.Display(),
			},
		}, tableStyle)
	})

	return m.OutputFileAndClose(outputPath)
}
