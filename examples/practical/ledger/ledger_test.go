// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"errors"
	"fmt"
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

// harness holds the ledger under test and the model: the balance the test
// expects every account to have. The commands are closures over it, and
// reset puts the model back to the opening balances whenever pbt replays
// a sequence.
type harness struct {
	ledger   *Ledger
	balances map[string]int
}

func (h *harness) reset() {
	h.balances = map[string]int{}
	for _, a := range accounts {
		h.balances[a] = opening
	}
}

// outcome is what the model says a transfer must return.
func (h *harness) outcome(from, to string, amount int) error {
	switch {
	case from == to:
		return ErrSameAccount
	case h.balances[from] < amount:
		return ErrInsufficient
	}
	return nil
}

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

func (h *harness) transfer() pbt.CommandGen {
	return pbt.Custom("transfer", func(t *pbt.T) pbt.Command {
		from, to := account.Draw(t, "from"), account.Draw(t, "to")
		n := amount.Draw(t, "amount")
		return pbt.Command{Name: "transfer", Args: []any{from, to, n},
			Run: func(pbt.Env) any { return h.ledger.Transfer(from, to, n) },
			Next: func(pbt.Var) {
				if h.outcome(from, to, n) == nil {
					h.balances[from] -= n
					h.balances[to] += n
				}
			},
			Post: func(_ pbt.Env, got any) error {
				err, _ := got.(error)
				if want := h.outcome(from, to, n); !errors.Is(err, want) {
					return fmt.Errorf("transfer returned %v, want %v", err, want)
				}
				return nil
			}}
	})
}

func (h *harness) query() pbt.CommandGen {
	return pbt.Map(account, func(name string) pbt.Command {
		return pbt.Command{Name: "balance", Args: []any{name},
			Run: func(pbt.Env) any { bal, err := h.ledger.Balance(name); return balance{bal, err} },
			Post: func(_ pbt.Env, got any) error {
				if want := (balance{h.balances[name], nil}); got != want {
					return fmt.Errorf("balance(%q) = %v, want %v", name, got, want)
				}
				return nil
			}}
	})
}

func (h *harness) commands() pbt.CommandGen {
	return pbt.Frequency(pbt.W(3, h.transfer()), pbt.W(1, h.query()))
}

func (h *harness) program() pbt.Gen[pbt.Program] {
	return pbt.Commands(h.reset, h.commands, pbt.ModelState(func() any { return h.balances }))
}

func TestLedger(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{ledger: newLedger(Options{})}
		h.program().Draw(t, "cmds").RunSequential(t)
	})
}

// The invariant-only test keeps no model. Transfers may succeed or fail,
// and the audit command checks what must hold whatever happened: the
// money is conserved and no account is overdrawn. This is the test to
// write when the exact outcome is hard to predict but the invariants are
// clear.
func (h *harness) anyTransfer() pbt.CommandGen {
	return pbt.Custom("transfer", func(t *pbt.T) pbt.Command {
		from, to := account.Draw(t, "from"), account.Draw(t, "to")
		n := amount.Draw(t, "amount")
		return pbt.Command{Name: "transfer", Args: []any{from, to, n},
			Run: func(pbt.Env) any { return h.ledger.Transfer(from, to, n) }}
	})
}

func (h *harness) audit() pbt.CommandGen {
	return pbt.Just(pbt.Command{Name: "audit",
		Run: func(pbt.Env) any { return h.ledger.Balances() },
		Post: func(_ pbt.Env, got any) error {
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
}

func (h *harness) invariants() pbt.CommandGen {
	return pbt.Frequency(pbt.W(3, h.anyTransfer()), pbt.W(1, h.audit()))
}

func TestLedgerInvariants(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{ledger: newLedger(Options{})}
		pbt.Commands(nil, h.invariants).Draw(t, "cmds").RunSequential(t)
	})
}

// A ledger that accepts a transfer from an account to itself creates
// money. The model test rejects the transfer's result at once; the
// invariant test needs the audit after it, and shrinks to exactly those
// two commands.
func TestSelfTransferIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		h := &harness{ledger: newLedger(Options{AllowSelfTransfer: true})}
		h.program().Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1))
	if !strings.Contains(report, "want ledger: cannot transfer to the same account") {
		t.Errorf("expected the model to reject the self transfer:\n%s", report)
	}

	report = expect.Failure(t, func(t *pbt.T) {
		h := &harness{ledger: newLedger(Options{AllowSelfTransfer: true})}
		pbt.Commands(nil, h.invariants).Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "= transfer(") != 1 || !strings.Contains(program, "= audit()") || !strings.Contains(report, "the ledger holds") {
		t.Errorf("expected one transfer and the audit that notices the extra money:\n%s", report)
	}
}

func TestLedgerParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{ledger: newLedger(Options{})}
		pbt.ParallelCommands(h.reset, h.commands).Draw(t, "cmds").RunParallel(t)
	})
}

// Two concurrent transfers from the same account both pass the balance
// check of the two-phase ledger. Both then succeed, which no sequential
// order of the model allows, since the second must have been refused.
func TestOverdraftRaceIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		h := &harness{}
		prog := pbt.ParallelCommands(h.reset, h.commands).Draw(t, "cmds")
		for range 5 {
			h.ledger = newLedger(Options{TwoPhase: true})
			prog.RunParallel(t)
		}
	}, pbt.Seed(1), pbt.Checks(300))
	if !strings.Contains(report, "no interleaving") {
		t.Errorf("expected a failure that no interleaving explains:\n%s", report)
	}
}
