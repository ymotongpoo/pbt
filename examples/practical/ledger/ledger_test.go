// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"errors"
	"fmt"
	"maps"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

var accounts = []string{"alice", "bob", "carol"}

const opening = 100

func newLedger(opts Options) *Ledger {
	bal := map[string]int{}
	for _, a := range accounts {
		bal[a] = opening
	}
	return New(bal, opts)
}

// model is what the test expects every balance to be. It is copied before
// every change: the shrinker regenerates sequences from earlier states, and
// a model changed in place would carry later results back into them.
type model struct{ balances map[string]int }

func initial() model {
	m := model{balances: map[string]int{}}
	for _, a := range accounts {
		m.balances[a] = opening
	}
	return m
}

func (m model) moved(from, to string, amount int) model {
	balances := maps.Clone(m.balances)
	balances[from] -= amount
	balances[to] += amount
	return model{balances}
}

// outcome is what the model says a transfer must return.
func (m model) outcome(from, to string, amount int) error {
	switch {
	case from == to:
		return ErrSameAccount
	case m.balances[from] < amount:
		return ErrInsufficient
	}
	return nil
}

type cmd = pbt.Cmd[model, *Ledger]

type balance struct {
	bal int
	err error
}

var (
	account = pbt.SampledFrom(accounts)
	// Amounts go past the opening balance, so that transfers are refused
	// about as often as they succeed.
	amount = pbt.IntRange(1, opening+opening/2)
)

var transfer = pbt.Custom("transfer", func(t *pbt.T) pbt.Command[model, *Ledger] {
	from, to := account.Draw(t, "from"), account.Draw(t, "to")
	n := amount.Draw(t, "amount")
	return cmd{Name: "transfer", Args: []any{from, to, n},
		Run: func(l *Ledger, _ pbt.Env) any { return l.Transfer(from, to, n) },
		Next: func(m model, _ pbt.Var) model {
			if m.outcome(from, to, n) != nil {
				return m
			}
			return m.moved(from, to, n)
		},
		Post: func(m model, _ pbt.Env, got any) error {
			err, _ := got.(error)
			if want := m.outcome(from, to, n); !errors.Is(err, want) {
				return fmt.Errorf("transfer returned %v, want %v", err, want)
			}
			return nil
		}}
})

var query = pbt.Map(account, func(name string) pbt.Command[model, *Ledger] {
	return cmd{Name: "balance", Args: []any{name},
		Run: func(l *Ledger, _ pbt.Env) any { bal, err := l.Balance(name); return balance{bal, err} },
		Post: func(m model, _ pbt.Env, got any) error {
			if want := (balance{m.balances[name], nil}); got != want {
				return fmt.Errorf("balance(%q) = %v, want %v", name, got, want)
			}
			return nil
		}}
})

func commands(model) pbt.Gen[pbt.Command[model, *Ledger]] {
	return pbt.Frequency(
		pbt.Weighted[pbt.Command[model, *Ledger]]{Weight: 3, Gen: transfer},
		pbt.Weighted[pbt.Command[model, *Ledger]]{Weight: 1, Gen: query},
	)
}

func TestLedger(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(initial(), commands).Draw(t, "cmds")
		prog.RunSequential(t, newLedger(Options{}))
	})
}

// The invariant-only test keeps no model. Transfers may succeed or fail,
// and the audit command checks what must hold whatever happened: the
// money is conserved and no account is overdrawn. This is the test to
// write when the exact outcome is hard to predict but the invariants are
// clear.
type noModel struct{}

var anyTransfer = pbt.Custom("transfer", func(t *pbt.T) pbt.Command[noModel, *Ledger] {
	from, to := account.Draw(t, "from"), account.Draw(t, "to")
	n := amount.Draw(t, "amount")
	return pbt.Cmd[noModel, *Ledger]{Name: "transfer", Args: []any{from, to, n},
		Run: func(l *Ledger, _ pbt.Env) any { return l.Transfer(from, to, n) }}
})

var audit = pbt.Just[pbt.Command[noModel, *Ledger]](pbt.Cmd[noModel, *Ledger]{Name: "audit",
	Run: func(l *Ledger, _ pbt.Env) any { return l.Balances() },
	Post: func(_ noModel, _ pbt.Env, got any) error {
		total := 0
		for name, bal := range got.(map[string]int) {
			if bal < 0 {
				return fmt.Errorf("%s is overdrawn: %d", name, bal)
			}
			total += bal
		}
		if want := opening * len(accounts); total != want {
			return fmt.Errorf("the ledger holds %d, want %d", total, want)
		}
		return nil
	}})

func invariants(noModel) pbt.Gen[pbt.Command[noModel, *Ledger]] {
	return pbt.Frequency(
		pbt.Weighted[pbt.Command[noModel, *Ledger]]{Weight: 3, Gen: anyTransfer},
		pbt.Weighted[pbt.Command[noModel, *Ledger]]{Weight: 1, Gen: audit},
	)
}

func TestLedgerInvariants(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(noModel{}, invariants).Draw(t, "cmds")
		prog.RunSequential(t, newLedger(Options{}))
	})
}

// A ledger that accepts a transfer from an account to itself creates
// money. The model test rejects the transfer's result at once; the
// invariant test needs the audit after it, and shrinks to exactly those
// two commands.
func TestSelfTransferIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := pbt.Commands(initial(), commands).Draw(t, "cmds")
		prog.RunSequential(t, newLedger(Options{AllowSelfTransfer: true}))
	}, pbt.Seed(1))
	if !strings.Contains(report, "want ledger: cannot transfer to the same account") {
		t.Errorf("expected the model to reject the self transfer:\n%s", report)
	}

	report = expect.Failure(t, func(t *pbt.T) {
		prog := pbt.Commands(noModel{}, invariants).Draw(t, "cmds")
		prog.RunSequential(t, newLedger(Options{AllowSelfTransfer: true}))
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "= transfer(") != 1 || !strings.Contains(program, "= audit()") || !strings.Contains(report, "the ledger holds") {
		t.Errorf("expected one transfer and the audit that notices the extra money:\n%s", report)
	}
}

func TestLedgerParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.ParallelCommands(initial(), commands).Draw(t, "cmds")
		prog.RunParallel(t, newLedger(Options{}))
	})
}

// Two concurrent transfers from the same account both pass the balance
// check of the two-phase ledger. Both then succeed, which no sequential
// order of the model allows, since the second must have been refused.
func TestOverdraftRaceIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := pbt.ParallelCommands(initial(), commands).Draw(t, "cmds")
		for range 5 {
			prog.RunParallel(t, newLedger(Options{TwoPhase: true}))
		}
	}, pbt.Seed(1), pbt.Checks(300))
	if !strings.Contains(report, "no interleaving") {
		t.Errorf("expected a failure that no interleaving explains:\n%s", report)
	}
}
