package main

// TODO Show the service address, otherwise we get 4 near-identical statements.

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strings"
	"text/template"
	"time"

	"github.com/jmacd/caspar.water/cmd/billing/internal/billing"
	"github.com/jmacd/caspar.water/cmd/billing/internal/csv"
	"github.com/jmacd/caspar.water/cmd/billing/internal/currency"
	"github.com/jmacd/caspar.water/cmd/billing/internal/expense"
	"github.com/jmacd/caspar.water/cmd/billing/internal/metadata"
	"github.com/jmacd/caspar.water/cmd/billing/internal/payment"
	"github.com/jmacd/caspar.water/cmd/billing/internal/user"
	"github.com/jmacd/maroto/pkg/color"
	"github.com/jmacd/maroto/pkg/consts"
	"github.com/jmacd/maroto/pkg/pdf"
	"github.com/jmacd/maroto/pkg/props"
)

var (
	// These files contain private data, are not kept in
	// this repository.
	usersFile    = flag.String("users", "users.csv", "csv")
	metadataFile = flag.String("metadata", "metadata.csv", "csv")
	expensesFile = flag.String("expenses", "expenses.csv", "csv")
	paymentsFile = flag.String("payments", "paymets.csv", "csv")

	statementsDir = flag.String("statements", "statements", "input directory")

	// Caspar water == blue.
	cwcColor = color.Color{
		Red:   10,
		Green: 10,
		Blue:  150,
	}
)

type (
	Vars struct {
		// Timestamps
		StartDate           string
		EndDate             string
		LastPaymentReceived string

		// How the fraction/percent are computed.
		EffectiveUserCount int
		UserWeight         int

		// Display strings
		Percent  string
		Fraction string
		Margin   string

		// Money top shelf
		Total        string // Semi-annual period
		Pay          string // Share of period total
		PriorBalance string // Unpaid balance
		TotalDue     string // Pay + PriorBalance
		LastPayment  string // Amount of last payment

		// Money breakdown
		Operations string
		Utilities  string
		Taxes      string
		Insurance  string
	}
)

// func parseDate(date string) (time.Time, error) {
// 	return time.Parse(constant.CsvLayout, date)
// }

// func (acct *Accounts) advanceBillingPeriod() (err error) {
// 	acct.invoiceName = acct.PeriodStart.Billing().Format(invoiceDateLayout)
// 	acct.dirPath = path.Join(*statementsDir, acct.invoiceName)
// 	if err := os.MkdirAll(acct.dirPath, 0777); err != nil {
// 		return fmt.Errorf("mkdir: %s: %w", acct.dirPath, err)
// 	}
// 	return nil
// }

// func (a *Accounts) prepareStatement(currentTmplPtr **template.Template) (err error) {
// 	base := a.invoiceName + ".txt"
// 	nt := template.New(base)
// 	in := path.Join(*statementsDir, base)

// 	if _, err = nt.ParseFiles(in); err == nil {
// 		a.statementTmpl = nt
// 		*currentTmplPtr = nt
// 	} else if errors.Is(err, os.ErrNotExist) && *currentTmplPtr != nil {
// 		a.statementTmpl = *currentTmplPtr
// 	} else {
// 		return fmt.Errorf("no statement template found: %w", err)
// 	}

// 	return nil
// }

func main() {
	flag.Parse()

	if err := Main(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// func (b *Billing) getPayment(user user.User, payments []currency.Amount) (currency.Amount, float64, int, []currency.Amount) {
// 	if !user.Active {
// 		return currency.Amount{}, 0, 0, payments
// 	}

// 	pay := payments[0]
// 	payments = payments[1:]
// 	weight := 1

// 	if user.AccountName == communityCenterAccount {
// 		weight = b.communityCenterCount

// 		pay = currency.Sum(pay, payments[0])
// 		payments = payments[1:]
// 	}

// 	fraction := float64(weight) / float64(b.effectiveUserCount)
// 	return pay, fraction, weight, payments
// }

// func parsePayments(payments []payment.Payment, users []user.User) (*account.Accounts, error) {
// 	ac := account.NewAccounts()

// 	for _, u := range users {
// 		ac.Register(u.AccountName)
// 	}

// 	for _, pay := range payments {
// 		ac := ac.Lookup(pay.AccountName)

// 		if ac == nil {
// 			return nil, fmt.Errorf("payment user not found: %v", pay.AccountName)
// 		}

// 		when, err := time.Parse(csvLayout, pay.Date)
// 		if err != nil {
// 			return nil, err
// 		}

// 		ac.EnterPayment(when, pay.Amount)
// 	}

// 	return ac, nil
// }

func Main() error {
	bill := billing.New()

	// Users
	users, err := csv.ReadAll[user.User](*usersFile)
	if err != nil {
		return err
	}

	// Metadata
	meta, err := csv.ReadAll[metadata.Metadata](*metadataFile)
	if err != nil {
		return err
	}
	if len(meta) != 1 {
		return fmt.Errorf("metadata file should have one row: %d", len(meta))
	}

	// Expenses
	expenses, err := csv.ReadAll[expense.Expenses](*expensesFile)
	if err != nil {
		return err
	}

	if err := expense.SplitAnnual(expenses); err != nil {
		return err
	}

	// Payments ledger
	payments, err := csv.ReadAll[payment.Payment](*paymentsFile)
	if err != nil {
		return err
	}

	// err := parsePayments(payments, users)
	// if err != nil {
	// 	return err
	// }

	var currentTmpl *template.Template

	for cycleNo := range accounts {
		acct := &accounts[cycleNo]
		if err := acct.advanceBillingPeriod(); err != nil {
			return err
		}

		switch acct.Method {
		case "Baseline":
			// No adjustments

		case "FirstAdjustment":
			b.communityCenterCount = communityCenterAdjustedUserCount
			b.effectiveUserCount += communityCenterAdjustment
			// In a different universe, this fallthrough
			// might matter.  It doesn't in our universe
			// because Caspar's FirstAdjustment happens
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
			panic(fmt.Sprintf("Unknown accounting method for %s: %s", acct.invoiceName, acct.Method))
		}

		err := acct.prepareStatement(&currentTmpl)
		if err != nil {
			fmt.Println("UMM")
			return err
		}

		expenses := currency.Sum(
			acct.Operations,
			acct.Utilities,
			acct.Taxes,
			acct.Insurance,
		)

		total := expenses.Scale(b.savingsRate)

		fmt.Printf("Billing cycle %v expenses %v savingsRate %.3f\n", acct.invoiceName, expenses.Display(), b.savingsRate)

		payments := total.Split(b.effectiveUserCount)

		// Deterministically shuffle the $0.01 rounding
		// differences so they are shared by different users.
		rand.New(rand.NewSource(acct.PeriodStart.Ending().UnixNano())).Shuffle(len(payments), func(i, j int) {
			payments[i], payments[j] = payments[j], payments[i]
		})

		marginStr := fmt.Sprintf("%.2f%%", b.savingsRate-1)
		startDate := acct.PeriodStart.Starting().Format(fullDateLayout)
		endDate := acct.PeriodStart.Ending().Format(fullDateLayout)

		for userNo := range users {
			user := &users[userNo]
			if !user.Active {
				continue
			}

			pdfPath := path.Join(acct.dirPath, user.AccountName+".pdf")

			pay, fraction, weight, reduced := b.getPayment(*user, payments)
			payments = reduced

			pctStr := fmt.Sprintf("%.2f%%", fraction*100)
			fracStr := fmt.Sprintf("%.4f", fraction)

			totalDue, _ := user.accountBalance.Add(&pay)

			var lastPay string
			var lastPayDate string
			if user.lastPayment.Amount() != 0 {
				lastPay = user.lastPayment.Display()
				lastPayDate = user.lastPaymentDate.Format(fullDateLayout)
			}

			vars := &Vars{
				StartDate:           startDate,
				EndDate:             endDate,
				LastPaymentReceived: lastPayDate,

				// Share
				EffectiveUserCount: b.effectiveUserCount,
				UserWeight:         weight,

				// Fractions
				Percent:  pctStr,
				Fraction: fracStr,
				Margin:   marginStr,

				// Top shelf
				Total:        total.Display(),
				Pay:          pay.Display(),
				TotalDue:     totalDue.Display(),
				PriorBalance: user.accountBalance.Display(),
				LastPayment:  lastPay,

				// Breakdown
				Operations: acct.Operations.Display(),
				Utilities:  acct.Utilities.Display(),
				Taxes:      acct.Taxes.Display(),
				Insurance:  acct.Insurance.Display(),
			}

			bill, err := b.makeBill(&meta[0], acct, user, vars)
			if err != nil {
				return err
			}
			if err := bill.OutputFileAndClose(pdfPath); err != nil {
				return err
			}

			// update the account balance
			newBal, _ := user.accountBalance.Add(&pay)
			user.accountBalance = *newBal
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

func (b *Billing) makeBill(meta *Metadata, acct *Accounts, user *user.User, vars *Vars) (pdf.Maroto, error) {
	m := pdf.NewMaroto(consts.Portrait, consts.Letter)
	m.SetPageMargins(30, 25, 30)

	const bigLine = 5
	const smallLine = 4
	const sepLine = 2

	toStyle := lineStyle{
		sz:    10,
		ht:    bigLine,
		top:   4,
		txt:   consts.Bold,
		align: consts.Left,
		color: color.NewBlack(),
	}

	paymentStyle := lineStyle{
		sz:    10,
		ht:    bigLine,
		top:   4,
		align: consts.Left,
		color: color.NewBlack(),
	}

	boldText := props.Text{
		Top:    3,
		Style:  consts.Bold,
		Align:  consts.Left,
		Family: consts.Helvetica,
		Size:   10,
	}

	normText := props.Text{
		Align:           consts.Left,
		Family:          consts.Helvetica,
		Size:            10,
		VerticalPadding: 1,
	}

	centerText := props.Text{
		Align:           consts.Center,
		Family:          consts.Helvetica,
		Size:            10,
		VerticalPadding: 1,
	}

	rightText := props.Text{
		Align:           consts.Right,
		Family:          consts.Helvetica,
		Size:            10,
		VerticalPadding: 1,
	}

	tableStyle := props.TableList{
		Align: consts.Right,
		ContentProp: props.TableListContent{
			Family: consts.Helvetica,
			Size:   9,
		},
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
	})

	m.RegisterFooter(func() {
		m.Row(3, func() {
			m.Col(0, func() {
				m.Text(meta.Contact, centerText)

			})
		})
	})

	m.Row(4, func() {})
	m.Row(6, func() {
		m.Col(4, func() {
			m.Text("To:", normText)
		})
		m.ColSpace(4)
		m.Col(4, func() {
			m.Text(time.Now().Format(fullDateLayout), rightText)
		})
	})

	toStyle.multiLine(m, append([]string{user.UserName}, parseAddress(user.BillingAddress)...))

	m.Row(4, func() {})
	m.Row(12, func() {
		m.Col(4, func() {
			m.Text("Invoice "+acct.invoiceName, boldText)
		})
	})

	var textBuf bytes.Buffer
	err := acct.statementTmpl.Execute(&textBuf, vars)
	if err != nil {
		return nil, err
	}

	for _, para := range strings.Split(textBuf.String(), "\n\n") {
		para = strings.TrimSpace(para)
		para = strings.ReplaceAll(para, "\n", " ")

		plines := m.GetLinesHeight(para, normText, 115)
		m.Row(float64(plines), func() {
			m.Col(0, func() {
				m.Text(para, normText)
			})
		})
		m.Row(1, func() {})
	}
	m.Row(1, func() {})

	m.Row(2, func() {
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
				"Subtotal (Semi-annual)",
				vars.Total,
			},
			{
				"Share",
				"× " + vars.Fraction,
			},
			{
				"Margin",
				"+ " + vars.Margin,
			},
			{},
			{
				"New balance",
				vars.Pay,
			},
			{
				"Prior balance",
				vars.PriorBalance,
			},
			{
				"Amount due",
				vars.TotalDue,
			},
		}, tableStyle)
	})

	m.Row(2, func() {})
	m.Row(4, func() {
		m.Col(4, func() {
			m.Text("Please send payment to:", normText)
		})
	})

	paymentStyle.multiLine(m, append([]string{meta.Name}, parseAddress(meta.Address)...))

	m.Row(10, func() {})
	m.Row(4, func() {
		m.Col(4, func() {
			m.Text("Thank you!", normText)
		})
	})
	return m, nil
}
