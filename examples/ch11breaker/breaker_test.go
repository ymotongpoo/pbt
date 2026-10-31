// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch11breaker

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
	"go.ymotongpoo.com/pbt/fsm"
)

const limit = 3

var opts = Options{NErrors: limit, NTimeouts: limit}

// data is the state data of the model: the counters the breaker keeps.
type data struct {
	errors, timeouts int
}

// validErrors are the names of the errors a failing call raises; the
// breaker counts any error that is not a timeout.
var validErrors = []string{"badarg", "badmatch", "badarith"}

type cmd = pbt.Cmd[data, *Breaker]

func expectErr(want error) func(data, pbt.Env, any) error {
	return func(_ data, _ pbt.Env, got any) error {
		gotErr, _ := got.(error)
		if !errors.Is(gotErr, want) && !(want == nil && gotErr == nil) {
			return fmt.Errorf("got %v, want %v", got, want)
		}
		return nil
	}
}

// The commands, as the shim module of the chapter exposes them: a
// successful call, a failing call with a given error, a timeout, and the
// manual operations.
func success(want error) pbt.Gen[pbt.Command[data, *Breaker]] {
	return pbt.Just[pbt.Command[data, *Breaker]](cmd{Name: "success",
		Run:  func(b *Breaker, _ pbt.Env) any { return b.Call(func() error { return nil }) },
		Next: func(d data, _ pbt.Var) data { return data{max(d.errors-1, 0), max(d.timeouts-1, 0)} },
		Post: expectErr(want)})
}

// failure is a call that raises the named error. A nil want means the
// call returns its own error, which is also what the call that trips the
// breaker returns; only the calls after it see ErrTripped.
func failure(name string, pre func(data) bool, next func(data) data, want error) pbt.Gen[pbt.Command[data, *Breaker]] {
	return pbt.Map(pbt.SampledFrom(validErrors), func(e string) pbt.Command[data, *Breaker] {
		err := errors.New(e)
		return cmd{Name: name, Args: []any{e},
			Pre:  pre,
			Run:  func(b *Breaker, _ pbt.Env) any { return b.Call(func() error { return err }) },
			Next: func(d data, _ pbt.Var) data { return next(d) },
			Post: func(_ data, _ pbt.Env, got any) error {
				gotErr, _ := got.(error)
				if want == nil && gotErr != err || want != nil && !errors.Is(gotErr, want) {
					return fmt.Errorf("got %v, want %v", got, want)
				}
				return nil
			}}
	})
}

func timeout(name string, pre func(data) bool, next func(data) data, want error) pbt.Gen[pbt.Command[data, *Breaker]] {
	return pbt.Just[pbt.Command[data, *Breaker]](cmd{Name: name,
		Pre:  pre,
		Run:  func(b *Breaker, _ pbt.Env) any { return b.Call(func() error { return ErrTimeout }) },
		Next: func(d data, _ pbt.Var) data { return next(d) },
		Post: expectErr(want)})
}

func manual(name string, op func(*Breaker), next func(data) data) pbt.Gen[pbt.Command[data, *Breaker]] {
	return pbt.Just[pbt.Command[data, *Breaker]](cmd{Name: name,
		Run:  func(b *Breaker, _ pbt.Env) any { op(b); return nil },
		Next: func(d data, _ pbt.Var) data { return next(d) }})
}

var (
	errStays   = func(d data) bool { return d.errors+1 < limit }
	errTrips   = func(d data) bool { return d.errors+1 >= limit }
	tmoStays   = func(d data) bool { return d.timeouts+1 < limit }
	tmoTrips   = func(d data) bool { return d.timeouts+1 >= limit }
	moreErrors = func(d data) data { return data{d.errors + 1, d.timeouts} }
	moreTmos   = func(d data) data { return data{d.errors, d.timeouts + 1} }
	same       = func(d data) data { return d }
	cleared    = func(data) data { return data{} }
)

// machine is the model of the chapter: ok, tripped and blocked, with the
// weights the chapter settled on so that the tripped state is reached
// often enough.
type machine struct{}

func (machine) Initial() (string, data) { return "ok", data{} }

func (machine) Transitions(state string, _ data) []fsm.Transition[string, data, *Breaker] {
	type tr = fsm.Transition[string, data, *Breaker]
	switch state {
	case "ok":
		return []tr{
			{Weight: 1, To: "ok", Cmd: success(nil)},
			{Weight: 4, To: "ok", Cmd: failure("err", errStays, moreErrors, nil)},
			{Weight: 5, To: "tripped", Cmd: failure("err", errTrips, moreErrors, nil)},
			{Weight: 4, To: "ok", Cmd: timeout("timeout", tmoStays, moreTmos, ErrTimeout)},
			{Weight: 5, To: "tripped", Cmd: timeout("timeout", tmoTrips, moreTmos, ErrTimeout)},
			{Weight: 1, To: "blocked", Cmd: manual("manual_block", (*Breaker).Block, same)},
			{Weight: 1, To: "ok", Cmd: manual("manual_reset", (*Breaker).Reset, cleared)},
		}
	case "tripped":
		return []tr{
			{Weight: 1, To: "tripped", Cmd: success(ErrTripped)},
			{Weight: 1, To: "tripped", Cmd: failure("err", nil, same, ErrTripped)},
			{Weight: 1, To: "tripped", Cmd: timeout("timeout", nil, same, ErrTripped)},
			{Weight: 1, To: "blocked", Cmd: manual("manual_block", (*Breaker).Block, same)},
			{Weight: 1, To: "ok", Cmd: manual("manual_reset", (*Breaker).Reset, cleared)},
		}
	default: // blocked
		return []tr{
			{Weight: 1, To: "blocked", Cmd: success(ErrBlocked)},
			{Weight: 1, To: "blocked", Cmd: failure("err", nil, same, ErrBlocked)},
			{Weight: 1, To: "blocked", Cmd: timeout("timeout", nil, same, ErrBlocked)},
			{Weight: 1, To: "ok", Cmd: manual("manual_deblock", func(b *Breaker) { b.Deblock(); b.Reset() }, cleared)},
		}
	}
}

func TestBreaker(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := fsm.Commands[string, data, *Breaker](machine{}).Draw(t, "cmds")
		prog.RunSequential(t, New(opts))
	})
}

// The chapter's first model did not know that a successful call lowers the
// counters; the breaker then stayed ok where the model expected tripped,
// which the next call reveals. A model with that mistake fails within a
// handful of commands.
type naiveMachine struct{ machine }

func (naiveMachine) Transitions(state string, d data) []fsm.Transition[string, data, *Breaker] {
	trs := machine{}.Transitions(state, d)
	if state == "ok" {
		trs[0] = fsm.Transition[string, data, *Breaker]{Weight: 1, To: "ok", Cmd: pbt.Just[pbt.Command[data, *Breaker]](cmd{Name: "success",
			Run: func(b *Breaker, _ pbt.Env) any { return b.Call(func() error { return nil }) }})}
	}
	return trs
}

func TestNaiveModelIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := fsm.Commands[string, data, *Breaker](naiveMachine{}).Draw(t, "cmds")
		prog.RunSequential(t, New(opts))
	}, pbt.Seed(1), pbt.Checks(500))
	if !strings.Contains(report, "success()") || !strings.Contains(report, "want circuit breaker tripped") {
		t.Errorf("expected the drift after a success call to show as a missing trip:\n%s", report)
	}
}
