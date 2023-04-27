package account

import (
	"github.com/jmacd/caspar.water/cmd/billing/internal/csv"
	"github.com/jmacd/caspar.water/cmd/billing/internal/currency"
	"github.com/jmacd/caspar.water/cmd/billing/internal/payment"
	"github.com/jmacd/caspar.water/cmd/billing/internal/user"
)

type Account struct {
	payments []payment.Payment
	charges  []payment.Payment
	user     user.User
}

type Accounts struct {
	balances map[string]*Account
}

func NewAccounts() *Accounts {
	return &Accounts{
		balances: map[string]*Account{},
	}
}

func (a *Accounts) Register(u user.User) {
	a.balances[u.AccountName] = &Account{
		user: u,
	}
}

func (a *Accounts) Lookup(name string) *Account {
	return a.balances[name]
}

func (a *Account) EnterPayment(pay payment.Payment) {
	a.payments = append(a.payments, pay)
}

func (a *Account) EnterAmountDue(date csv.Date, due currency.Amount) {
	a.charges = append(a.charges, payment.Payment{
		Date:        date,
		AccountName: a.user.AccountName,
		Amount:      due,
	})
}

func (a *Account) Balance(on csv.Date) currency.Amount {
	var total currency.Amount
	for _, charge := range a.charges {
		if !charge.Date.Date().After(on.Date()) {
			total = currency.Sum(total, charge.Amount)
		}
	}
	for _, pay := range a.payments {
		if !pay.Date.Date().After(on.Date()) {
			total = currency.Difference(total, pay.Amount)
		}
	}
	return total
}

func (a *Account) LastPayment() payment.Payment {
	if a.payments == nil {
		return payment.Payment{}
	}

	return a.payments[len(a.payments)-1]
}
