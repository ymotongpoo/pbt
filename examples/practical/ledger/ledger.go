// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package ledger is a practical example of a stateful test with an
// invariant. A ledger moves money between accounts; whatever sequence of
// transfers runs, the total must not change and no balance may go
// negative. The tests show three ways to state that: a model that predicts
// every result, an audit command that checks the invariants without a
// model, and a parallel test that catches a transfer which checks the
// balance and moves the money in two separate critical sections.
package ledger

import (
	"errors"
	"runtime"
	"sync"
)

var (
	// ErrUnknownAccount is returned for an account that was never opened.
	ErrUnknownAccount = errors.New("ledger: unknown account")
	// ErrSameAccount is returned for a transfer from an account to itself.
	ErrSameAccount = errors.New("ledger: cannot transfer to the same account")
	// ErrBadAmount is returned for a zero or negative amount.
	ErrBadAmount = errors.New("ledger: amount must be positive")
	// ErrInsufficient is returned when the source account cannot cover the
	// amount.
	ErrInsufficient = errors.New("ledger: insufficient funds")
)

// Options select the planted bugs of the example. The zero value is the
// correct ledger.
type Options struct {
	// AllowSelfTransfer skips the same-account check. The transfer reads
	// both balances before it writes both, so the credit overwrites the
	// debit and the account gains the amount.
	AllowSelfTransfer bool
	// TwoPhase checks the balance and moves the money in two separate
	// critical sections. Two concurrent transfers from one account can then
	// both pass the check, and the account goes negative.
	TwoPhase bool
}

// Ledger holds the balances of a fixed set of accounts, in cents.
type Ledger struct {
	mu       sync.Mutex
	balances map[string]int
	opts     Options
}

// New returns a ledger with the given opening balances.
func New(opening map[string]int, opts Options) *Ledger {
	l := &Ledger{balances: make(map[string]int, len(opening)), opts: opts}
	for name, bal := range opening {
		l.balances[name] = bal
	}
	return l
}

// Transfer moves amount from one account to another. It fails without
// changing anything when the amount is not positive, an account is
// unknown, both accounts are the same, or the source cannot cover it.
func (l *Ledger) Transfer(from, to string, amount int) error {
	if amount <= 0 {
		return ErrBadAmount
	}
	if from == to && !l.opts.AllowSelfTransfer {
		return ErrSameAccount
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	fromBal, fromOK := l.balances[from]
	toBal, toOK := l.balances[to]
	if !fromOK || !toOK {
		return ErrUnknownAccount
	}
	if fromBal < amount {
		return ErrInsufficient
	}
	if l.opts.TwoPhase {
		// The planted race: the check is done, the lock is released, and a
		// concurrent transfer from the same account can pass its own check
		// before this one moves the money.
		l.mu.Unlock()
		runtime.Gosched()
		l.mu.Lock()
		fromBal, toBal = l.balances[from], l.balances[to]
	}
	l.balances[from] = fromBal - amount
	l.balances[to] = toBal + amount
	return nil
}

// Balance returns the balance of an account.
func (l *Ledger) Balance(name string) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	bal, ok := l.balances[name]
	if !ok {
		return 0, ErrUnknownAccount
	}
	return bal, nil
}

// Balances returns a copy of every balance.
func (l *Ledger) Balances() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string]int, len(l.balances))
	for name, bal := range l.balances {
		out[name] = bal
	}
	return out
}
