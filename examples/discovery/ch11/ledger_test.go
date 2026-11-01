// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch11

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

// Ledger は残高を持つ台帳である。
type Ledger struct{ bal int }

var errInsufficient = errors.New("insufficient funds")

func (l *Ledger) Deposit(d int) int {
	l.bal += d
	return l.bal
}

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

func ledgerProperty(newLedger func() ledger) pbt.Property {
	return func(t *pbt.T) {
		l := newLedger()
		bal := 0 // モデル：残高

		deposit := pbt.Map(pbt.IntRange(1, 100), func(d int) pbt.Command {
			return pbt.Command{
				Name: "deposit", Args: []any{d},
				Run:  func(pbt.Env) any { return l.Deposit(d) },
				Next: func(pbt.Var) { bal += d },
				Post: func(_ pbt.Env, got any) error {
					if got.(int) != bal+d {
						return fmt.Errorf("deposit(%d) returned balance %v, want %d", d, got, bal+d)
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
				amt := bal + e // モデルの残高より大きい額を作る
				return pbt.Command{
					Name: "overdraft", Args: []any{amt},
					Pre: func() bool { return amt > bal },
					Run: func(pbt.Env) any { return l.Withdraw(amt) },
					// Next はない：モデルは変わらない、が問い2への答えである
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
	pbt.Check(t, ledgerProperty(func() ledger { return &Ledger{} }), pbt.Verbose())
}

var noSave = []pbt.Option{pbt.NoSave(), pbt.NoReplay()}

// ---- 変異

// lenientLedger は残高不足の出金を許す。
type lenientLedger struct{ Ledger }

func (l *lenientLedger) Withdraw(d int) error {
	l.bal -= d
	return nil
}

type doubleDepositLedger struct{ Ledger }

func (l *doubleDepositLedger) Deposit(d int) int {
	l.bal += 2 * d
	return l.bal
}

func TestLenientMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(func() ledger { return &lenientLedger{} }), noSave...)
}
func TestDoubleDepositMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(func() ledger { return &doubleDepositLedger{} }), noSave...)
}

// ---- 失敗の解剖1：Post はいつ動くか
func postMistakeProperty(newLedger func() ledger) pbt.Property {
	return func(t *pbt.T) {
		l := newLedger()
		bal := 0
		deposit := pbt.Map(pbt.IntRange(1, 100), func(d int) pbt.Command {
			return pbt.Command{
				Name: "deposit", Args: []any{d},
				Run:  func(pbt.Env) any { return l.Deposit(d) },
				Next: func(pbt.Var) { bal += d },
				// 悪い例：Post の中の bal を、更新後の残高だと思っている
				Post: func(_ pbt.Env, got any) error {
					if got != bal { // 誤り：bal はまだ更新前の値である
						return fmt.Errorf("deposit(%d) returned balance %v, want %d", d, got, bal)
					}
					return nil
				},
			}
		})
		commands := func() pbt.CommandGen { return deposit }
		pbt.Commands(func() { bal = 0 }, commands, pbt.ModelState(func() any { return bal })).Draw(t, "cmds").RunSequential(t)
	}
}

func TestPostMistakeMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, postMistakeProperty(func() ledger { return &Ledger{} }), noSave...)
}

// ---- 失敗の解剖2：Pre
var preFalse atomic.Int64

// fromModel は額をモデルから選ぶか、モデルを見ずに選ぶかを切り替える。
func preProperty(newLedger func() ledger, fromModel, withPre bool) pbt.Property {
	return func(t *pbt.T) {
		l := newLedger()
		bal := 0
		deposit := pbt.Map(pbt.IntRange(1, 100), func(d int) pbt.Command {
			return pbt.Command{
				Name: "deposit", Args: []any{d},
				Run:  func(pbt.Env) any { return l.Deposit(d) },
				Next: func(pbt.Var) { bal += d },
			}
		})
		mkWithdraw := func(d int) pbt.Command {
			c := pbt.Command{
				Name: "withdraw", Args: []any{d},
				Run:  func(pbt.Env) any { return l.Withdraw(d) },
				Next: func(pbt.Var) { bal -= d },
				Post: func(_ pbt.Env, got any) error {
					if got != nil {
						return fmt.Errorf("withdraw(%d) failed with balance %d: %v", d, bal, got)
					}
					return nil
				},
			}
			if withPre {
				c.Pre = func() bool {
					ok := d <= bal
					if !ok {
						preFalse.Add(1)
					}
					return ok
				}
			}
			return c
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
			if fromModel {
				if bal == 0 {
					return pbt.OneOf(deposit, balance)
				}
				return pbt.Frequency(pbt.W(3, deposit), pbt.W(2, pbt.Map(pbt.IntRange(1, bal), mkWithdraw)), pbt.W(1, balance))
			}
			// 悪い例：額がモデルを見ておらず、Pre もない
			blind := pbt.W(2, pbt.Map(pbt.IntRange(1, 100), mkWithdraw))
			return pbt.Frequency(pbt.W(3, deposit), blind, pbt.W(1, balance))
		}
		pbt.Commands(func() { bal = 0 }, commands).Draw(t, "cmds").RunSequential(t)
	}
}

// forgetWithdrawLedger は出金を残高に反映し忘れる。
type forgetWithdrawLedger struct{ Ledger }

func (l *forgetWithdrawLedger) Withdraw(d int) error {
	if d > l.bal {
		return errInsufficient
	}
	return nil
}

func TestFromModelWithPre(t *testing.T) {
	preFalse.Store(0)
	pbt.Check(t, preProperty(func() ledger { return &Ledger{} }, true, true), pbt.Checks(2000))
	if n := preFalse.Load(); n != 0 {
		t.Fatalf("Pre was false %d times", n)
	}
}
func TestFromModelNoPre(t *testing.T) {
	pbt.Check(t, preProperty(func() ledger { return &Ledger{} }, true, false), pbt.Checks(2000))
}
func TestBlindWithPre(t *testing.T) {
	preFalse.Store(0)
	pbt.Check(t, preProperty(func() ledger { return &Ledger{} }, false, true), pbt.Checks(2000), pbt.Seed(1))
	t.Logf("Pre false: %d", preFalse.Load())
}

// モデルを見ない生成器で Pre もないと、正しい台帳が落ちる。
func TestBlindNoPreMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, preProperty(func() ledger { return &Ledger{} }, false, false), noSave...)
}
func TestFromModelWithPreShrinkMutant(t *testing.T) {
	mutant.Only(t)
	preFalse.Store(0)
	defer func() { t.Logf("Pre false: %d", preFalse.Load()) }()
	pbt.Check(t, preProperty(func() ledger { return &forgetWithdrawLedger{} }, true, true), noSave...)
}
func TestFromModelNoPreShrinkMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, preProperty(func() ledger { return &forgetWithdrawLedger{} }, true, false), noSave...)
}
func TestBlindWithPreShrinkMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, preProperty(func() ledger { return &forgetWithdrawLedger{} }, false, true), noSave...)
}

// ---- 並行では枝が互いの残高を読めないので Pre が要る
type lockedLedger struct {
	mu sync.Mutex
	Ledger
}

func (l *lockedLedger) Deposit(d int) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Ledger.Deposit(d)
}

func (l *lockedLedger) Withdraw(d int) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.Ledger.Withdraw(d)
}

func parallelProperty(withPre bool) pbt.Property {
	return func(t *pbt.T) {
		l := &lockedLedger{}
		bal := 0
		deposit := pbt.Map(pbt.IntRange(1, 5), func(d int) pbt.Command {
			return pbt.Command{
				Name: "deposit", Args: []any{d},
				Run:  func(pbt.Env) any { return l.Deposit(d) },
				Next: func(pbt.Var) { bal += d },
				Post: func(_ pbt.Env, got any) error {
					if got != bal+d {
						return fmt.Errorf("deposit(%d) returned %v, want %d", d, got, bal+d)
					}
					return nil
				},
			}
		})
		withdraw := func() pbt.CommandGen {
			return pbt.Map(pbt.IntRange(1, bal), func(d int) pbt.Command {
				c := pbt.Command{
					Name: "withdraw", Args: []any{d},
					Run:  func(pbt.Env) any { return l.Withdraw(d) },
					Next: func(pbt.Var) { bal -= d },
					Post: func(_ pbt.Env, got any) error {
						if got != nil {
							return fmt.Errorf("withdraw(%d) failed with balance %d", d, bal)
						}
						return nil
					},
				}
				if withPre {
					c.Pre = func() bool { return d <= bal }
				}
				return c
			})
		}
		commands := func() pbt.CommandGen {
			if bal == 0 {
				return deposit
			}
			return pbt.OneOf(deposit, withdraw())
		}
		pbt.ParallelCommands(func() { bal = 0 }, commands).Draw(t, "cmds").RunParallel(t)
	}
}

func TestParallelWithPre(t *testing.T) { pbt.Check(t, parallelProperty(true)) }
func TestParallelNoPreMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, parallelProperty(false), noSave...)
}
