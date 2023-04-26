package account

import (
	"github.com/jmacd/caspar.water/cmd/billing/internal/payment"
)

type Account struct {
	payments []payment.Payment
}

type Accounts struct {
	balances map[string]*Account
}

func NewAccounts() *Accounts {
	return &Accounts{
		balances: map[string]*Account{},
	}
}

func (a *Accounts) Register(name string) {
	a.balances[name] = &Account{}
}

func (a *Accounts) Lookup(name string) *Account {
	return a.balances[name]
}

func (a *Account) EnterPayment(pay payment.Payment) {
	a.payments = append(a.payments, pay)
}
