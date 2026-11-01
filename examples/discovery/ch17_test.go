// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/discovery/internal/mutant"
)

type doubleDepositLedger struct{ Ledger }

func (l *doubleDepositLedger) Deposit(d int) int { l.bal += 2 * d; return l.bal }

type forgetWithdrawLedger struct{ Ledger }

func (l *forgetWithdrawLedger) Withdraw(d int) error {
	if d > l.bal {
		return errInsufficient
	}
	return nil // forgets to subtract
}

func TestLedgerDoubleDepositMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(func() ledger { return &doubleDepositLedger{} }, true), pbt.NoSave(), pbt.NoReplay())
}

func TestLedgerForgetWithdrawMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(func() ledger { return &forgetWithdrawLedger{} }, true), pbt.NoSave(), pbt.NoReplay())
}

// with the weak deposit Post (got < d), which command catches the double deposit?
func TestLedgerDoubleDepositWeakMutant(t *testing.T) {
	mutant.Only(t)
	pbt.Check(t, ledgerProperty(func() ledger { return &doubleDepositLedger{} }, false), pbt.NoSave(), pbt.NoReplay())
}
