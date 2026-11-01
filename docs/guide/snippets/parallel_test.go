// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"fmt"
	"sync"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: parallel
// Counter is safe for concurrent use. Add returns the new value, which
// gives the interleaving check a result to explain for every call.
type Counter struct {
	mu sync.Mutex
	n  int
}

func (c *Counter) Add(d int) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n += d
	return c.n
}

func TestCounterParallel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		c := &Counter{}
		n := 0 // the model: what the counter should hold
		add := pbt.Map(pbt.IntRange(1, 5), func(d int) pbt.Command {
			return pbt.Command{
				Name: "add", Args: []any{d},
				Run:  func(pbt.Env) any { return c.Add(d) },
				Next: func(pbt.Var) { n += d },
				Post: func(_ pbt.Env, got any) error {
					if got != n+d {
						return fmt.Errorf("add(%d) returned %v, want %d", d, got, n+d)
					}
					return nil
				},
			}
		})
		commands := func() pbt.CommandGen { return add }
		prog := pbt.ParallelCommands(func() { n = 0 }, commands).Draw(t, "cmds")
		prog.RunParallel(t)
	})
}

// snippet: end
