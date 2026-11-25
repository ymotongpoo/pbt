// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"fmt"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: causality
var delayedWriteProperty = func(t *pbt.T) {
	step, want, stored := 0, 0, 0
	init := func() { step, want = 0, 0 }
	next := func(pbt.Var) { step++ }
	gen := func() pbt.CommandGen {
		switch step {
		case 0:
			return pbt.Map(pbt.IntRange(0, 15), func(value int) pbt.Command {
				return pbt.Command{
					Name: "write", Args: []any{value},
					Run: func(pbt.Env) any {
						stored = value
						if value == 7 {
							stored = 0 // planted bug: this write loses its value
						}
						return nil
					},
					Next: func(pbt.Var) { want = value; step++ },
				}
			})
		case 5:
			return pbt.Just(pbt.Command{
				Name: "read", Next: next,
				Run: func(pbt.Env) any { return stored },
				Post: func(_ pbt.Env, result any) error {
					if result.(int) != want {
						return fmt.Errorf("read %d, want %d", result, want)
					}
					return nil
				},
			})
		default:
			return pbt.Just(pbt.Command{Name: "wait", Next: next})
		}
	}
	pbt.Commands(init, gen, pbt.Length(6, 6)).Draw(t, "program").RunSequential(t)
}

// snippet: end

func TestCausalityFindsEarlyWrite(t *testing.T) {
	report := failureReport(t, "TestDelayedWrite", delayedWriteProperty,
		pbt.Seed(1), pbt.Causality(32))
	mustContain(t, report, "step 6: postcondition of read() failed: read 0, want 7",
		"property failed after 38 passed checks and 16 shrink steps (size 39)",
		"causality (32 continuations per prefix, seed 0x1):",
		"after 0 of 6 steps: 0/32 fail with the same identity",
		"after 1 of 6 steps: 32/32 fail with the same identity <- largest measured rise at step 1: write(7)")
}
