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

// counterProgram generates parallel programs over a counter and a model
// count; the counter is created by the caller before the program runs.
func counterProgram(c **counter) Gen[ParallelProgram] {
	var n int64
	inc := Just(Command{Name: "inc",
		Run:  func(Env) any { return (*c).inc() },
		Next: func(Var) { n++ },
		Post: func(_ Env, res any) error {
			if res != n+1 {
				return fmt.Errorf("inc() = %v, want %d", res, n+1)
			}
			return nil
		}})
	return ParallelCommands(func() { n = 0 }, func() Gen[Command] { return inc })
}

func counterProperty(racy bool) Property {
	return func(t *T) {
		var c *counter
		prog := counterProgram(&c).Draw(t, "cmds")
		c = &counter{racy: racy}
		prog.RunParallel(t)
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
func TestParallelBranchesStayInterleavable(t *testing.T) {
	Check(t, func(t *T) {
		open := false
		gen := func() Gen[Command] {
			if open {
				return Just(Command{Name: "close",
					Pre:  func() bool { return open },
					Next: func(Var) { open = false }})
			}
			return Just(Command{Name: "open",
				Pre:  func() bool { return !open },
				Next: func(Var) { open = true }})
		}
		prog := ParallelCommands(func() { open = false }, gen).Draw(t, "cmds")
		if !prog.model.allInterleavingsValid(prog.prefix, prog.branches) {
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
		prog.RunParallel(t)
	}, NoSave())
}

func TestBranchesOptionShapesTheProgram(t *testing.T) {
	step := Just(Command{Name: "step"})
	Check(t, func(t *T) {
		prog := ParallelCommands(nil, func() Gen[Command] { return step }, Branches(3, 2), Length(0, 1)).Draw(t, "cmds")
		if len(prog.Branches()) != 3 || len(prog.Prefix()) > 1 {
			t.Fatalf("got %d branches after a prefix of %d", len(prog.Branches()), len(prog.Prefix()))
		}
		for _, br := range prog.Branches() {
			if len(br) > 2 {
				t.Fatalf("a branch has %d commands", len(br))
			}
		}
	}, NoSave())
	defer func() {
		msg, _ := recover().(string)
		if !strings.Contains(msg, "Branches(4, 1)") {
			t.Errorf("message %q", msg)
		}
	}()
	ParallelCommands(nil, func() Gen[Command] { return step }, Branches(4, 1))
}

func TestParallelProgramString(t *testing.T) {
	var c *counter
	prog := counterProgram(&c).Example(3)
	text := prog.String()
	for _, want := range []string{"ParallelProgram{", "prefix:", "branch 1:", "branch 2:"} {
		if !strings.Contains(text, want) {
			t.Errorf("String() lacks %q:\n%s", want, text)
		}
	}
}
