// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch14

import (
	"errors"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

var errInsufficient = errors.New("insufficient balance")

// Ledger は第11章の台帳である。
type Ledger struct{ bal int }

func NewLedger(start int) *Ledger { return &Ledger{bal: start} }

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

// 悪い例：入金と出金は順序によらないと決めつけている
func ledgerSwapProperty(start int) pbt.Property {
	return func(t *pbt.T) {
		d := pbt.IntRange(1, 10).Draw(t, "d")
		w := pbt.IntRange(1, 10).Draw(t, "w")

		a := NewLedger(start)
		a.Deposit(d)
		a.Withdraw(w)

		b := NewLedger(start)
		b.Withdraw(w)
		b.Deposit(d)

		if a.Balance() != b.Balance() {
			t.Fatalf("deposit(%d), withdraw(%d) order changed the balance: %d, %d", d, w, a.Balance(), b.Balance())
		}
	}
}

func TestLedgerSwapRichSurvives(t *testing.T) {
	pbt.Check(t, ledgerSwapProperty(1000), append(opts, pbt.Checks(2000))...)
}
func TestLedgerSwapFromZeroMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerSwapProperty(0), opts...)
}

// 前提を主張に含めた版：出金が先でも通る残高に限る
func TestLedgerSwapWithFunds(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		start := pbt.IntRange(0, 100).Draw(t, "start")
		w := pbt.IntRange(0, start).Draw(t, "w") // 出金が先でも足りる額に限る
		d := pbt.IntRange(1, 10).Draw(t, "d")

		a := NewLedger(start)
		a.Deposit(d)
		errA := a.Withdraw(w)

		b := NewLedger(start)
		errB := b.Withdraw(w)
		b.Deposit(d)

		if errA != nil || errB != nil || a.Balance() != b.Balance() {
			t.Fatalf("start=%d d=%d w=%d: %v, %v, %d, %d", start, d, w, errA, errB, a.Balance(), b.Balance())
		}
	}, opts...)
}
