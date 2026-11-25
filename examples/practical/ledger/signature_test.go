// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ledger

import (
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

// namedAudit checks the same invariants as audit. Both causes share the
// audit postcondition's automatic identity, so name the violated rule.
// Names omit account names and balances to keep shrinking on that rule.
func (h *harness) namedAudit(t *pbt.T) pbt.CommandGen {
	return pbt.Just(pbt.Command{Name: "audit",
		Run: func(pbt.Env) any { return h.ledger.Balances() },
		Post: func(_ pbt.Env, got any) error {
			balances := got.(map[string]int)
			total := 0
			// A fixed account order also makes the first overdraft stable.
			for _, name := range accounts {
				bal := balances[name]
				if bal < 0 {
					t.Signature("overdrawn account")
					return fmt.Errorf("%s is overdrawn: %d", name, bal)
				}
				total += bal
			}
			if want := opening * len(accounts); total != want {
				t.Signature("money not conserved")
				return fmt.Errorf("the ledger holds %d, want %d", total, want)
			}
			return nil
		}})
}

func namedInvariants(opts Options) pbt.Property {
	return func(t *pbt.T) {
		h := &harness{ledger: newLedger(opts)}
		commands := func() pbt.CommandGen {
			return pbt.Frequency(pbt.W(3, h.anyTransfer()), pbt.W(1, h.namedAudit(t)))
		}
		pbt.Commands(nil, commands, pbt.Length(0, 20)).Draw(t, "cmds").RunSequential(t)
	}
}

func TestLedgerNamedInvariants(t *testing.T) {
	pbt.Check(t, namedInvariants(Options{}), pbt.Seed(1))
}

func TestAuditCausesAreCaught(t *testing.T) {
	reports := expect.Failures(t, namedInvariants(Options{
		AllowSelfTransfer: true,
		AllowOverdraft:    true,
	}), pbt.Seed(1), pbt.MaxFailures(2))
	if len(reports) != 2 {
		t.Fatalf("got %d reports, want two audit causes: %v", len(reports), reports)
	}
	report := strings.Join(reports, "\n")
	for _, want := range []string{
		"failure 1 of 2:", "failure 2 of 2:",
		`failure was named "overdrawn account"`,
		`failure was named "money not conserved"`,
		"postcondition of audit() failed:", "is overdrawn:", "the ledger holds",
	} {
		if !strings.Contains(report, want) {
			t.Errorf("expected %q in the audit report:\n%s", want, report)
		}
	}
}
