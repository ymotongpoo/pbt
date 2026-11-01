// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"errors"
	"fmt"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

type Ledger struct{ bal int }

var errInsufficient = errors.New("insufficient funds")

func (l *Ledger) Deposit(d int) int { l.bal += d; return l.bal }
func (l *Ledger) Withdraw(d int) error {
	if d > l.bal {
		return errInsufficient
	}
	l.bal -= d
	return nil
}
func (l *Ledger) Balance() int { return l.bal }

type ledger interface {
	Deposit(d int) int
	Withdraw(d int) error
	Balance() int
}

func ledgerProperty(newLedger func() ledger, exactDeposit bool) pbt.Property {
	return func(t *pbt.T) {
		l := newLedger()
		bal := 0

		deposit := pbt.Map(pbt.IntRange(1, 100), func(d int) pbt.Command {
			return pbt.Command{
				Name: "deposit", Args: []any{d},
				Run:  func(pbt.Env) any { return l.Deposit(d) },
				Next: func(pbt.Var) { bal += d },
				Post: func(_ pbt.Env, got any) error {
					if exactDeposit {
						if got.(int) != bal+d {
							return fmt.Errorf("deposit(%d) returned balance %v, want %d", d, got, bal+d)
						}
						return nil
					}
					if got.(int) < d {
						return fmt.Errorf("deposit(%d) returned balance %v", d, got)
					}
					return nil
				},
			}
		})
		withdraw := func() pbt.CommandGen {
			return pbt.Map(pbt.IntRange(1, bal), func(d int) pbt.Command {
				return pbt.Command{
					Name: "withdraw", Args: []any{d},
					Pre:  func() bool { return d <= bal },
					Run:  func(pbt.Env) any { return l.Withdraw(d) },
					Next: func(pbt.Var) { bal -= d },
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("withdraw(%d) failed with balance %d: %v", d, bal, got)
						}
						return nil
					},
				}
			})
		}
		overdraft := func() pbt.CommandGen {
			return pbt.Map(pbt.IntRange(1, 50), func(e int) pbt.Command {
				amt := bal + e
				return pbt.Command{
					Name: "overdraft", Args: []any{amt},
					Run: func(pbt.Env) any { return l.Withdraw(amt) },
					Post: func(_ pbt.Env, got any) error {
						if got == nil {
							return fmt.Errorf("withdraw(%d) succeeded, want error", amt)
						}
						return nil
					},
				}
			})
		}
		balance := pbt.Just(pbt.Command{
			Name: "balance",
			Run:  func(pbt.Env) any { return l.Balance() },
			Post: func(_ pbt.Env, got any) error {
				if got != bal {
					return fmt.Errorf("balance() = %v, model says %d", got, bal)
				}
				return nil
			},
		})
		commands := func() pbt.CommandGen {
			if bal == 0 {
				return pbt.Frequency(pbt.W(3, deposit), pbt.W(1, overdraft()), pbt.W(1, balance))
			}
			return pbt.Frequency(pbt.W(3, deposit), pbt.W(2, withdraw()), pbt.W(1, overdraft()), pbt.W(1, balance))
		}
		init := func() { bal = 0 }
		prog := pbt.Commands(init, commands, pbt.ModelState(func() any { return bal })).Draw(t, "cmds")
		prog.RunSequential(t)
	}
}

func TestLedger(t *testing.T) {
	pbt.Check(t, ledgerProperty(func() ledger { return &Ledger{} }, true), pbt.Verbose())
}

func TestLedgerWeakDeposit(t *testing.T) {
	pbt.Check(t, ledgerProperty(func() ledger { return &Ledger{} }, false))
}

// mutant: allows overdraft
type lenientLedger struct{ Ledger }

func (l *lenientLedger) Withdraw(d int) error { l.bal -= d; return nil }

func TestLedgerMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(func() ledger { return &lenientLedger{} }, true), pbt.NoSave(), pbt.NoReplay())
}
