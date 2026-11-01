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

// harness holds the breaker under test and the state data of the model:
// the counters the breaker keeps. The state name is the machine's.
type harness struct {
	breaker          *Breaker
	errors, timeouts int
}

func (h *harness) reset() { h.errors, h.timeouts = 0, 0 }

// validErrors are the names of the errors a failing call raises; the
// breaker counts any error that is not a timeout.
var validErrors = []string{"badarg", "badmatch", "badarith"}

func expectErr(want error) func(pbt.Env, any) error {
	return func(_ pbt.Env, got any) error {
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
func (h *harness) success(want error) pbt.CommandGen {
	return pbt.Just(pbt.Command{Name: "success",
		Run:  func(pbt.Env) any { return h.breaker.Call(func() error { return nil }) },
		Next: func(pbt.Var) { h.errors, h.timeouts = max(h.errors-1, 0), max(h.timeouts-1, 0) },
		Post: expectErr(want)})
}

// failure is a call that raises the named error. A nil want means the
// call returns its own error, which is also what the call that trips the
// breaker returns; only the calls after it see ErrTripped.
func (h *harness) failure(name string, pre func() bool, next func(), want error) pbt.CommandGen {
	return pbt.Map(pbt.SampledFrom(validErrors), func(e string) pbt.Command {
		err := errors.New(e)
		return pbt.Command{Name: name, Args: []any{e},
			Pre:  pre,
			Run:  func(pbt.Env) any { return h.breaker.Call(func() error { return err }) },
			Next: func(pbt.Var) { apply(next) },
			Post: func(_ pbt.Env, got any) error {
				gotErr, _ := got.(error)
				if want == nil && gotErr != err || want != nil && !errors.Is(gotErr, want) {
					return fmt.Errorf("got %v, want %v", got, want)
				}
				return nil
			}}
	})
}

func (h *harness) timeout(name string, pre func() bool, next func(), want error) pbt.CommandGen {
	return pbt.Just(pbt.Command{Name: name,
		Pre:  pre,
		Run:  func(pbt.Env) any { return h.breaker.Call(func() error { return ErrTimeout }) },
		Next: func(pbt.Var) { apply(next) },
		Post: expectErr(want)})
}

func (h *harness) manual(name string, op func(*Breaker), next func()) pbt.CommandGen {
	return pbt.Just(pbt.Command{Name: name,
		Run:  func(pbt.Env) any { op(h.breaker); return nil },
		Next: func(pbt.Var) { apply(next) }})
}

// apply runs a model update; nil leaves the data as it is.
func apply(next func()) {
	if next != nil {
		next()
	}
}

func (h *harness) errStays() bool     { return h.errors+1 < limit }
func (h *harness) errTrips() bool     { return h.errors+1 >= limit }
func (h *harness) tmoStays() bool     { return h.timeouts+1 < limit }
func (h *harness) tmoTrips() bool     { return h.timeouts+1 >= limit }
func (h *harness) moreErrors()        { h.errors++ }
func (h *harness) moreTmos()          { h.timeouts++ }
func (h *harness) cleared()           { h.errors, h.timeouts = 0, 0 }
func (h *harness) deblock(b *Breaker) { b.Deblock(); b.Reset() }

// transitions is the model of the chapter: ok, tripped and blocked, with
// the weights the chapter settled on so that the tripped state is reached
// often enough.
func (h *harness) transitions(state string) []fsm.Transition {
	switch state {
	case "ok":
		return []fsm.Transition{
			{Weight: 1, To: "ok", Cmd: h.success(nil)},
			{Weight: 4, To: "ok", Cmd: h.failure("err", h.errStays, h.moreErrors, nil)},
			{Weight: 5, To: "tripped", Cmd: h.failure("err", h.errTrips, h.moreErrors, nil)},
			{Weight: 4, To: "ok", Cmd: h.timeout("timeout", h.tmoStays, h.moreTmos, ErrTimeout)},
			{Weight: 5, To: "tripped", Cmd: h.timeout("timeout", h.tmoTrips, h.moreTmos, ErrTimeout)},
			{Weight: 1, To: "blocked", Cmd: h.manual("manual_block", (*Breaker).Block, nil)},
			{Weight: 1, To: "ok", Cmd: h.manual("manual_reset", (*Breaker).Reset, h.cleared)},
		}
	case "tripped":
		return []fsm.Transition{
			{Weight: 1, To: "tripped", Cmd: h.success(ErrTripped)},
			{Weight: 1, To: "tripped", Cmd: h.failure("err", nil, nil, ErrTripped)},
			{Weight: 1, To: "tripped", Cmd: h.timeout("timeout", nil, nil, ErrTripped)},
			{Weight: 1, To: "blocked", Cmd: h.manual("manual_block", (*Breaker).Block, nil)},
			{Weight: 1, To: "ok", Cmd: h.manual("manual_reset", (*Breaker).Reset, h.cleared)},
		}
	default: // blocked
		return []fsm.Transition{
			{Weight: 1, To: "blocked", Cmd: h.success(ErrBlocked)},
			{Weight: 1, To: "blocked", Cmd: h.failure("err", nil, nil, ErrBlocked)},
			{Weight: 1, To: "blocked", Cmd: h.timeout("timeout", nil, nil, ErrBlocked)},
			{Weight: 1, To: "ok", Cmd: h.manual("manual_deblock", h.deblock, h.cleared)},
		}
	}
}

func (h *harness) data() any { return fmt.Sprintf("errors=%d timeouts=%d", h.errors, h.timeouts) }

func TestBreaker(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		h := &harness{breaker: New(opts)}
		prog := fsm.Commands("ok", h.reset, h.transitions, pbt.ModelState(h.data)).Draw(t, "cmds")
		prog.RunSequential(t)
	})
}

// The chapter's first model did not know that a successful call lowers the
// counters; the breaker then stayed ok where the model expected tripped,
// which the next call reveals. A model with that mistake fails within a
// handful of commands.
func (h *harness) naiveTransitions(state string) []fsm.Transition {
	trs := h.transitions(state)
	if state == "ok" {
		trs[0] = fsm.Transition{Weight: 1, To: "ok", Cmd: pbt.Just(pbt.Command{Name: "success",
			Run: func(pbt.Env) any { return h.breaker.Call(func() error { return nil }) }})}
	}
	return trs
}

func TestNaiveModelIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		h := &harness{breaker: New(opts)}
		prog := fsm.Commands("ok", h.reset, h.naiveTransitions).Draw(t, "cmds")
		prog.RunSequential(t)
	}, pbt.Seed(1), pbt.Checks(500))
	if !strings.Contains(report, "success()") || !strings.Contains(report, "want circuit breaker tripped") {
		t.Errorf("expected the drift after a success call to show as a missing trip:\n%s", report)
	}
}
