// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package jobqueue

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

// handoff follows two workers contending for one job. The generated wait
// decides whether the first worker's lease expires before the second polls.
// The first worker then acknowledges with its original lease.
func (h *harness) handoff() pbt.Gen[pbt.Program] {
	step := 0
	init := func() { h.reset(); step = 0 }
	commands := func() pbt.CommandGen {
		var gen pbt.CommandGen
		switch step {
		case 0:
			gen = h.enqueue()
		case 1, 3:
			gen = h.lease()
		case 2:
			gen = h.advance()
		default:
			first := h.leases[0]
			gen = pbt.Just(pbt.Command{Name: "ack", Args: []any{first.v},
				Run: func(env pbt.Env) any { return h.queue.Ack(pbt.Resolve[taken](env, first.v).lease) },
				Next: func(pbt.Var) {
					if h.current(first) {
						h.jobs[first.job].phase = done
					}
				},
				Post: func(_ pbt.Env, got any) error {
					err, _ := got.(error)
					var want error
					if !h.current(first) {
						want = ErrStaleLease
					}
					if !errors.Is(err, want) {
						return fmt.Errorf("ack(%v) returned %v, want %v", first.v, err, want)
					}
					return nil
				}})
		}
		return pbt.Map(gen, func(cmd pbt.Command) pbt.Command {
			next := cmd.Next
			cmd.Next = func(v pbt.Var) {
				if next != nil {
					next(v)
				}
				step++
			}
			return cmd
		})
	}
	return pbt.Commands(init, commands, pbt.Length(5, 5))
}

func TestLeaseHandoff(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		newHarness(Options{}).handoff().Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1))
}

func TestStaleAckCausalityIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		// Reset both the queue and clock on every continuation and replay.
		newHarness(Options{IgnoreToken: true}).handoff().Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1), pbt.Causality(8))
	if !strings.Contains(report, "ack(var2) returned <nil>, want jobqueue: lease is not current") ||
		!strings.Contains(report, "causality (8 continuations per prefix, seed 0x1):") {
		t.Fatalf("expected the stale ack and its causality report:\n%s", report)
	}
	// The rise refers to the original program, before shrinking. A
	// timeout makes a lease stale; leasing again exposes the missing token
	// check. Sampled rates suggest a cause, rather than proving one.
	if !strings.Contains(report, "largest measured rise at step 3: advance(") &&
		!strings.Contains(report, "largest measured rise at step 4: lease()") {
		t.Fatalf("expected the largest rise at a lease expiry or the second lease:\n%s", report)
	}
}
