// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// counter increments a shared value. The racy variant reads, yields and
// writes back, so two concurrent increments can return the same value; it
// uses atomics, so the race is logical rather than a data race.
type counter struct {
	n    atomic.Int64
	mu   sync.Mutex
	racy bool
}

func (c *counter) inc() int64 {
	if c.racy {
		v := c.n.Load()
		runtime.Gosched()
		c.n.Store(v + 1)
		return v + 1
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n.Add(1)
}

func counterCommands(int64) Gen[Command[int64, *counter]] {
	return Just[Command[int64, *counter]](Cmd[int64, *counter]{Name: "inc",
		Run:  func(c *counter, _ Env) any { return c.inc() },
		Next: func(n int64, _ Var) int64 { return n + 1 },
		Post: func(n int64, _ Env, res any) error {
			if res != n+1 {
				return fmt.Errorf("inc() = %v, want %d", res, n+1)
			}
			return nil
		}})
}

func counterProperty(racy bool) Property {
	return func(t *T) {
		prog := ParallelCommands(int64(0), counterCommands).Draw(t, "cmds")
		prog.RunParallel(t, &counter{racy: racy})
	}
}

func TestParallelCounterHolds(t *testing.T) {
	Check(t, counterProperty(false), NoSave())
}

func TestParallelRaceIsCaught(t *testing.T) {
	tb := &fakeTB{name: "TestRace"}
	Check(tb, counterProperty(true), Seed(1), NoSave(), Checks(300))
	if len(tb.errors) != 1 {
		t.Fatalf("the lost update was not detected in 300 runs: %q", tb.errors)
	}
	report := tb.errors[0]
	for _, want := range []string{"no interleaving of the 2 branches", "branch 1:", "branch 2:", "inc()"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}

// gate only allows open after an open-free prefix: open requires closed,
// close requires open. Two branches that both open from the same state would
// have an invalid interleaving, so the generator must trim them.
type gateModel struct{ open bool }

func gateCommands(m gateModel) Gen[Command[gateModel, *struct{}]] {
	if m.open {
		return Just[Command[gateModel, *struct{}]](Cmd[gateModel, *struct{}]{Name: "close",
			Pre:  func(m gateModel) bool { return m.open },
			Next: func(gateModel, Var) gateModel { return gateModel{false} }})
	}
	return Just[Command[gateModel, *struct{}]](Cmd[gateModel, *struct{}]{Name: "open",
		Pre:  func(m gateModel) bool { return !m.open },
		Next: func(gateModel, Var) gateModel { return gateModel{true} }})
}

func TestParallelBranchesStayInterleavable(t *testing.T) {
	Check(t, func(t *T) {
		prog := ParallelCommands(gateModel{}, gateCommands).Draw(t, "cmds")
		state := prog.init
		for _, s := range prog.prefix {
			state = s.Command.NextState(state, s.Result)
		}
		if !allInterleavingsValid(state, prog.branches, make([]int, len(prog.branches))) {
			t.Fatalf("an interleaving violates a precondition:\n%s", prog)
		}
		busy := 0
		for _, br := range prog.branches {
			if len(br) > 0 {
				busy++
			}
		}
		if busy > 1 {
			t.Fatalf("two non-empty branches cannot both be valid for the gate:\n%s", prog)
		}
		prog.RunParallel(t, &struct{}{})
	}, NoSave())
}

func TestParallelProgramString(t *testing.T) {
	prog := ParallelCommands(int64(0), counterCommands).Example(3)
	text := prog.String()
	for _, want := range []string{"ParallelProgram{", "prefix:", "branch 1:", "branch 2:"} {
		if !strings.Contains(text, want) {
			t.Errorf("String() lacks %q:\n%s", want, text)
		}
	}
}
