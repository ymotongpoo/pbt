// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package fsm_test

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/fsm"
)

// breaker is a circuit breaker in the spirit of the one in Chapter 11 of the
// PropEr Testing book: it trips after limit failures and stays tripped until
// it is reset. The buggy variant lets successful calls through while tripped.
type breaker struct {
	mu            sync.Mutex
	limit, errors int
	tripped       bool
	buggy         bool
}

var (
	errTripped = errors.New("tripped")
	errFailed  = errors.New("failed")
)

func (b *breaker) Success() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tripped && !b.buggy {
		return errTripped
	}
	return nil
}

func (b *breaker) Fail() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.tripped {
		return errTripped
	}
	b.errors++
	if b.errors >= b.limit {
		b.tripped = true
		return errTripped
	}
	return errFailed
}

func (b *breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.errors = 0
	b.tripped = false
}

const limit = 2

func expect(want error) func(pbt.Env, any) error {
	return func(_ pbt.Env, got any) error {
		if got != want {
			return fmt.Errorf("got %v, want %v", got, want)
		}
		return nil
	}
}

// machine builds the transitions over a breaker and the error count the
// model keeps. The commands close over both.
func machine(b **breaker) (init func(), transitions func(string) []fsm.Transition) {
	errs := 0
	success := pbt.Just(pbt.Command{Name: "success",
		Run: func(pbt.Env) any { return (*b).Success() }, Post: expect(nil)})
	successTripped := pbt.Just(pbt.Command{Name: "success",
		Run: func(pbt.Env) any { return (*b).Success() }, Post: expect(errTripped)})
	failStaying := pbt.Just(pbt.Command{Name: "fail",
		Pre:  func() bool { return errs+1 < limit },
		Run:  func(pbt.Env) any { return (*b).Fail() },
		Next: func(pbt.Var) { errs++ },
		Post: expect(errFailed)})
	failTripping := pbt.Just(pbt.Command{Name: "fail",
		Pre:  func() bool { return errs+1 >= limit },
		Run:  func(pbt.Env) any { return (*b).Fail() },
		Next: func(pbt.Var) { errs++ },
		Post: expect(errTripped)})
	reset := pbt.Just(pbt.Command{Name: "reset",
		Run:  func(pbt.Env) any { (*b).Reset(); return nil },
		Next: func(pbt.Var) { errs = 0 }})
	transitions = func(state string) []fsm.Transition {
		switch state {
		case "ok":
			return []fsm.Transition{
				{Weight: 3, To: "ok", Cmd: success},
				{Weight: 2, To: "ok", Cmd: failStaying},
				{Weight: 2, To: "tripped", Cmd: failTripping},
				{Weight: 1, To: "ok", Cmd: reset},
			}
		default:
			return []fsm.Transition{
				{Weight: 2, To: "tripped", Cmd: successTripped},
				{Weight: 1, To: "ok", Cmd: reset},
			}
		}
	}
	return func() { errs = 0 }, transitions
}

func property(buggy bool) pbt.Property {
	return func(t *pbt.T) {
		var b *breaker
		init, transitions := machine(&b)
		prog := fsm.Commands("ok", init, transitions).Draw(t, "cmds")
		b = &breaker{limit: limit, buggy: buggy}
		prog.RunSequential(t)
	}
}

type fakeTB struct {
	name   string
	logs   []string
	errors []string
}

func (f *fakeTB) Helper()      {}
func (f *fakeTB) Name() string { return f.name }
func (f *fakeTB) Logf(format string, args ...any) {
	f.logs = append(f.logs, fmt.Sprintf(format, args...))
}
func (f *fakeTB) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func TestBreakerHolds(t *testing.T) {
	tb := &fakeTB{name: "TestBreaker"}
	pbt.Check(tb, property(false), pbt.Seed(1), pbt.NoSave())
	if len(tb.errors) != 0 {
		t.Fatalf("unexpected failure: %q", tb.errors)
	}
	logs := strings.Join(tb.logs, "\n")
	for _, want := range []string{"% ok -> tripped: fail", "% tripped -> tripped: success", "% ok -> ok: success"} {
		if !strings.Contains(logs, want) {
			t.Errorf("statistics lack %q:\n%s", want, logs)
		}
	}
}

func TestBuggyBreakerIsCaughtWithThreeCommands(t *testing.T) {
	tb := &fakeTB{name: "TestBuggyBreaker"}
	pbt.Check(tb, property(true), pbt.Seed(1), pbt.NoSave(), pbt.Checks(500))
	if len(tb.errors) != 1 {
		t.Fatalf("got %d errors, want 1: %q", len(tb.errors), tb.errors)
	}
	report := tb.errors[0]
	steps := strings.Count(report, "[pbt]     var")
	if steps != 3 {
		t.Errorf("counterexample has %d commands, want 3:\n%s", steps, report)
	}
	for _, want := range []string{"ok -> tripped: fail()", "tripped -> tripped: success()", "postcondition of", "got <nil>, want tripped", "model state before step 3: tripped"} {
		if !strings.Contains(report, want) {
			t.Errorf("report lacks %q:\n%s", want, report)
		}
	}
}

func TestParallelBreakerHolds(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		var b *breaker
		init, transitions := machine(&b)
		prog := fsm.ParallelCommands("ok", init, transitions).Draw(t, "cmds")
		b = &breaker{limit: limit}
		prog.RunParallel(t)
	}, pbt.NoSave(), pbt.Checks(30))
}
