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

	"github.com/jmacd/caspar.water/cmd/billing/internal/billing"
	"github.com/jmacd/caspar.water/cmd/billing/internal/business"
	"github.com/jmacd/caspar.water/cmd/billing/internal/constant"
	"github.com/jmacd/caspar.water/cmd/billing/internal/csv"
	"github.com/jmacd/caspar.water/cmd/billing/internal/currency"
	"github.com/jmacd/caspar.water/cmd/billing/internal/expense"
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
	businessFile = flag.String("business", "business.csv", "csv")
	cyclesFile   = flag.String("cycles", "cycles.csv", "csv")
	paymentsFile = flag.String("payments", "payments.csv", "csv")

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
		StartFullDate       string
		StartMonthDate      string
		CloseFullDate       string
		CloseMonthDate      string
		IssueFullDate       string
		LastPaymentReceived string

		// How the fraction/percent are computed.
		EffectiveUserCount int
		UserWeight         int
		Estimated          bool

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

func main() {
	flag.Parse()

	if err := Main(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func getPayment(user user.User, charges []currency.Amount, b *billing.Billing) (currency.Amount, float64, int, []currency.Amount) {
	if !user.Active {
		panic("impossible")
	}

	pay := charges[0]
	charges = charges[1:]
	weight := 1

	if user.AccountName == constant.CommunityCenterAccount {
		weight = b.CommunityCenterCount()

		for i := 1; i < b.CommunityCenterCount(); i++ {
			pay = currency.Sum(pay, charges[0])
			charges = charges[1:]
		}
	}

	fraction := float64(weight) / float64(b.EffectiveUserCount())
	return pay, fraction, weight, charges
}

func Main() error {
	bill := billing.New()

	// Users
	users, err := csv.ReadFile[user.User](*usersFile)
	if err != nil {
		return err
	}

	// Business
	meta, err := csv.ReadFile[business.Business](*businessFile)
	if err != nil {
		return err
	}
	if len(meta) != 1 {
		return fmt.Errorf("business file should have one row: %d", len(meta))
	}

	// Expense cycles
	cycles, err := csv.ReadFile[expense.Cycle](*cyclesFile)
	if err != nil {
		return err
	}

	if err := expense.SplitAnnual(cycles); err != nil {
		return err
	}

	// Payments ledger
	payments, err := csv.ReadFile[payment.Payment](*paymentsFile)
	if err != nil {
		return err
	}

	if err := bill.EnterUsers(users); err != nil {
		return err
	}
	if err := bill.EnterPayments(payments); err != nil {
		return err
	}

	for _, cycle := range cycles {

		startFullDate := cycle.PeriodStart.Starting().Date().Format(constant.FullDateLayout)
		closeFullDate := cycle.PeriodStart.Closing().Date().Format(constant.FullDateLayout)
		startMonthDate := cycle.PeriodStart.Starting().Date().Format(constant.InvoiceDateLayout)
		closeMonthDate := cycle.PeriodStart.Closing().Date().Format(constant.InvoiceDateLayout)
		issueFullDate := cycle.BillDate.Date().Format(constant.FullDateLayout)
		inputText := closeMonthDate + ".txt"
		outputPath := path.Join(*statementsDir, closeMonthDate)
		inputTextPath := path.Join(*statementsDir, inputText)

		if err := os.MkdirAll(outputPath, 0777); err != nil {
			return fmt.Errorf("mkdir: %s: %w", outputPath, err)
		}

		bill.StartCycle(cycle)

		newTmpl := template.New(inputText)

		if _, err = newTmpl.ParseFiles(inputTextPath); err != nil {
			return fmt.Errorf("no statement template found: %w", err)
		}

		sumExpenses := currency.Sum(
			cycle.Operations,
			cycle.Utilities,
			cycle.Taxes,
			cycle.Insurance,
		)

		total := sumExpenses.Scale(bill.SavingsRate())

		fmt.Printf("Billing cycle %v..%v cycles %v savingsRate %.3f\n", startMonthDate, closeMonthDate, sumExpenses.Display(), bill.SavingsRate())

		charges := total.Split(bill.EffectiveUserCount())

		// Deterministically shuffle the $0.01 rounding
		// differences so they are shared by different users.
		rand.New(rand.NewSource(cycle.PeriodStart.Closing().Date().UnixNano())).Shuffle(len(charges), func(i, j int) {
			charges[i], charges[j] = charges[j], charges[i]
		})

		marginStr := fmt.Sprintf("%.2f%%", bill.SavingsRate()-1)

		// If the bill date is prior to
		estimatedBilling := cycle.BillDate.Before(cycle.PeriodStart.Closing())

		for _, user := range users {
			if !user.Active {
				continue
			}
			if user.FirstPeriodStart.Starting().Date().After(cycle.PeriodStart.Starting().Date()) {
				continue
			}

			pdfPath := path.Join(outputPath, user.AccountName+".pdf")

			owes, fraction, weight, reduced := getPayment(user, charges, bill)
			charges = reduced

			pctStr := fmt.Sprintf("%.2f%%", fraction*100)
			fracStr := fmt.Sprintf("%.4f", fraction)

			priorBalance := bill.Balance(user, cycle.BillDate)

			bill.EnterAmountDue(user, cycle.PeriodStart.Closing(), owes)

			if estimatedBilling {
				cycle.BillDate = cycle.PeriodStart.Closing()
			}

			totalDue := bill.Balance(user, cycle.BillDate)

			var lastPay string
			var lastPayDate string
			if lp := bill.LastPayment(user); !lp.Amount.IsZero() {
				lastPay = lp.Amount.Display()
				lastPayDate = lp.Date.Date().Format(constant.FullDateLayout)
			}

			vars := &Vars{
				StartFullDate:       startFullDate,
				CloseFullDate:       closeFullDate,
				CloseMonthDate:      closeMonthDate,
				IssueFullDate:       issueFullDate,
				LastPaymentReceived: lastPayDate,

				// Share
				EffectiveUserCount: bill.EffectiveUserCount(),
				UserWeight:         weight,
				Estimated:          estimatedBilling,

				// Fractions
				Percent:  pctStr,
				Fraction: fracStr,
				Margin:   marginStr,

				// Top shelf
				Total:        total.Display(),
				Pay:          owes.Display(),
				TotalDue:     totalDue.Display(),
				PriorBalance: priorBalance.Display(),
				LastPayment:  lastPay,

				// Breakdown
				Operations: cycle.Operations.Display(),
				Utilities:  cycle.Utilities.Display(),
				Taxes:      cycle.Taxes.Display(),
				Insurance:  cycle.Insurance.Display(),
			}

			stmt, err := makeBill(meta[0], cycle, user, vars, newTmpl, bill)
			if err != nil {
				return err
			}
			if err := stmt.OutputFileAndClose(pdfPath); err != nil {
				return err
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

func makeBill(bus business.Business, cycle expense.Cycle, user user.User, vars *Vars, tmpl *template.Template, b *billing.Billing) (pdf.Maroto, error) {
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
				m.Text(bus.Contact, centerText)

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
			m.Text(vars.IssueFullDate, rightText)
		})
	})

	toStyle.multiLine(m, append([]string{user.UserName}, user.BillingAddress.Split()...))
	m.Row(4, func() {})

	invoiceName := vars.CloseMonthDate
	if vars.Estimated {
		invoiceName += "-Estimate"
	}

	m.Row(8, func() {
		m.Col(8, func() {
			m.Text("Invoice: "+invoiceName, boldText)
		})
		m.Row(4, func() {})
	})

	m.Row(8, func() {
		m.Col(12, func() {
			m.Text("Service address: "+user.ServiceAddress.OneLine(), boldText)
		})
	})
	m.Row(4, func() {})

	var textBuf bytes.Buffer
	err := tmpl.Execute(&textBuf, vars)
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
				cycle.Operations.Display(),
			},
			{
				"Utilities",
				cycle.Utilities.Display(),
			},
			{
				"Insurance",
				cycle.Insurance.Display(),
			},
			{
				"Taxes",
				cycle.Taxes.Display(),
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

	paymentStyle.multiLine(m, append([]string{bus.Name}, bus.Address.Split()...))

	m.Row(10, func() {})
	m.Row(4, func() {
		m.Col(4, func() {
			m.Text("Thank you!", normText)
		})
	})
	return m, nil
}
