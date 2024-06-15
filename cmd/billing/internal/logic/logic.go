package logic

// TODO Show the service address, otherwise we get 4 near-identical statements.

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/jmacd/caspar.water/cmd/billing/internal/account"
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
	"github.com/spf13/afero"
)

var (
	// Caspar water == blue.
	cwcColor = color.Color{
		Red:   10,
		Green: 10,
		Blue:  150,
	}
)

type (
	Inputs struct {
		UsersFile     string
		BusinessFile  string
		CyclesFile    string
		PaymentsFile  string
		StatementsDir string
	}

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
		TotalCost    string // Semi-annual period
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

	UserStatement struct {
		User    user.User
		Vars    *Vars
		PdfPath string
	}

	CompanyStatement struct {
		Expenses   expense.Cycle
		Template   *template.Template
		Statements []*UserStatement
	}

	Result struct {
		Accounts *account.Accounts
		Business business.Business
		Cycles   []*CompanyStatement
	}
)

func getPayment(user user.User, charges []currency.Amount, cycle expense.Cycle) (currency.Amount, float64, int, []currency.Amount) {
	if cycle.Inactive.Contains(user) {
		return currency.Units(0), 0, 0, charges
	}

	pay := charges[0]
	charges = charges[1:]
	weight := 1

	if user.Commercial && cycle.Method == expense.NormalMethod {
		weight = 2
		pay = currency.Sum(pay, charges[0])
		charges = charges[1:]
	}

	fraction := float64(weight) / float64(cycle.EffectiveConnections)
	return pay, fraction, weight, charges
}

func Logic(inputs Inputs, fs afero.Fs) (*Result, error) {
	accts := account.NewAccounts()

	// Users
	users, err := csv.ReadFile[user.User](inputs.UsersFile, fs)
	if err != nil {
		return nil, err
	}

	// Business
	business, err := csv.ReadFile[business.Business](inputs.BusinessFile, fs)
	if err != nil {
		return nil, err
	}
	if len(business) != 1 {
		return nil, fmt.Errorf("business file should have one row: %d", len(business))
	}

	// Expense cycles
	cycles, err := csv.ReadFile[expense.Cycle](inputs.CyclesFile, fs)
	if err != nil {
		return nil, err
	}

	if err := expense.SplitAnnual(cycles); err != nil {
		return nil, err
	}

	// Payments ledger
	payments, err := csv.ReadFile[payment.Payment](inputs.PaymentsFile, fs)
	if err != nil {
		return nil, err
	}

	for _, user := range users {
		accts.Register(user)
	}

	for _, pay := range payments {
		acct := accts.Lookup(pay.AccountName)
		if acct == nil {
			return nil, fmt.Errorf("payment account not found: %s", pay.AccountName)
		}
		acct.EnterPayment(pay)
	}

	result := &Result{
		Accounts: accts,
		Business: business[0],
	}

	for _, cycle := range cycles {
		compStmt := &CompanyStatement{
			Expenses: cycle,
		}
		result.Cycles = append(result.Cycles, compStmt)

		startFullDate := cycle.PeriodStart.Starting().Date().Format(constant.FullDateLayout)
		closeFullDate := cycle.PeriodStart.Closing().Date().Format(constant.FullDateLayout)
		startMonthDate := cycle.PeriodStart.Starting().Date().Format(constant.InvoiceDateLayout)
		closeMonthDate := cycle.PeriodStart.Closing().Date().Format(constant.InvoiceDateLayout)
		issueFullDate := cycle.BillDate.Date().Format(constant.FullDateLayout)
		inputText := closeMonthDate + ".txt"
		outputPath := path.Join(inputs.StatementsDir, closeMonthDate)
		inputTextPath := path.Join(inputs.StatementsDir, inputText)

		if err := os.MkdirAll(outputPath, 0777); err != nil {
			return nil, fmt.Errorf("mkdir: %s: %w", outputPath, err)
		}

		compStmt.Template = template.New(inputText)

		if _, err = compStmt.Template.ParseFS(afero.NewIOFS(fs), inputTextPath); err != nil {
			return nil, fmt.Errorf("%s: no statement template found: %w", inputTextPath, err)
		}

		sumExpenses := currency.Sum(
			cycle.Operations,
			cycle.Utilities,
			cycle.Taxes,
			cycle.Insurance,
		)

		savingsRate := 1 + cycle.Margin
		total := sumExpenses.Scale(savingsRate)

		// Check that effective connection count is not exceeded
		realCount := 0
		for _, user := range users {
			if cycle.Inactive.Contains(user) {
				continue
			}
			realCount++
			if user.Commercial && cycle.Method == expense.NormalMethod {
				realCount++
			}
		}
		if realCount > cycle.EffectiveConnections {
			return nil, fmt.Errorf("logic error: too many connections found: %v > %v", realCount, cycle.EffectiveConnections)
		}

		fmt.Printf("Billing cycle %v..%v cycles %v savingsRate %.3f\n", startMonthDate, closeMonthDate, sumExpenses.Display(), savingsRate)

		charges := total.Split(cycle.EffectiveConnections)

		// Deterministically shuffle the $0.01 rounding
		// differences so they are shared by different users.
		rand.New(rand.NewSource(cycle.PeriodStart.Closing().Date().UnixNano())).Shuffle(len(charges), func(i, j int) {
			charges[i], charges[j] = charges[j], charges[i]
		})

		marginStr := fmt.Sprintf("%.2f", savingsRate)

		// If the bill date is prior to
		estimatedBilling := cycle.BillDate.Before(cycle.PeriodStart.Closing())

		for _, user := range users {
			if user.FirstPeriodStart.Starting().Date().After(cycle.PeriodStart.Starting().Date()) {
				continue
			}

			userStmt := &UserStatement{
				User:    user,
				PdfPath: path.Join(outputPath, user.AccountName+".pdf"),
			}
			compStmt.Statements = append(compStmt.Statements, userStmt)

			owes, fraction, weight, reduced := getPayment(user, charges, cycle)
			charges = reduced

			pctStr := fmt.Sprintf("%.2f%%", fraction*100)
			fracStr := fmt.Sprintf("%.4f", fraction)

			acct := accts.Lookup(user.AccountName)
			priorBalance := acct.Balance(cycle.BillDate)

			// @@@ Oops. Some sort of bug is creeping with the handling of
			// pennies and historical payments.  I'm seeing off-by $0.01 in
			// the invoices, which I'm going to erase temporarily:

			acct.EnterAmountDue(cycle.PeriodStart.Closing(), owes)

			if estimatedBilling {
				cycle.BillDate = cycle.PeriodStart.Closing()
			}

			totalDue := acct.Balance(cycle.BillDate)

			var lastPay string
			var lastPayDate string
			if lp := acct.LastPayment(); !lp.Amount.IsZero() {
				lastPay = lp.Amount.Display()
				lastPayDate = lp.Date.Date().Format(constant.FullDateLayout)
			}

			userStmt.Vars = &Vars{
				StartFullDate:       startFullDate,
				CloseFullDate:       closeFullDate,
				CloseMonthDate:      closeMonthDate,
				IssueFullDate:       issueFullDate,
				LastPaymentReceived: lastPayDate,

				// Share
				EffectiveUserCount: cycle.EffectiveConnections,
				UserWeight:         weight,
				Estimated:          estimatedBilling,

				// Fractions
				Percent:  pctStr,
				Fraction: fracStr,
				Margin:   marginStr,

				// Top shelf
				TotalCost:    sumExpenses.Display(),
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
		}
	}
	return result, nil
}

func Output(result *Result) error {
	for _, cycle := range result.Cycles {
		for _, stmt := range cycle.Statements {
			print, err := makeBill(result.Business, cycle.Expenses, stmt.User, stmt.Vars, cycle.Template)
			if err != nil {
				return err
			}
			if err := print.OutputFileAndClose(stmt.PdfPath); err != nil {
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

func makeBill(bus business.Business, cycle expense.Cycle, user user.User, vars *Vars, tmpl *template.Template) (pdf.Maroto, error) {
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
		//m.Row(1, func() {})
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
				vars.TotalCost,
			},
			{
				"Share",
				"× " + vars.Fraction,
			},
			{
				"Margin",
				"× " + vars.Margin,
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
