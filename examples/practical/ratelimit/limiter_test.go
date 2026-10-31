// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/clock"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

const (
	limit  = 3
	window = time.Second
)

var (
	clients = []string{"alice", "bob", "carol"}
	// The steps the clock takes: within a window, exactly a window, which
	// tests the boundary, and past it.
	steps = []time.Duration{100 * time.Millisecond, 250 * time.Millisecond, 500 * time.Millisecond, window, 2 * window}
)

// harness holds the limiter under test, the clock the test controls, and
// the model: the time elapsed since the start and every request that was
// allowed, per client. The model never prunes: the sequences are short,
// and the rule is easier to read over the whole history.
type harness struct {
	limiter *Limiter
	clock   *clock.Clock
	now     time.Duration
	grants  map[string][]time.Duration
}

func newHarness(opts Options) *harness {
	clk := clock.New()
	return &harness{limiter: New(clk.Now, limit, window, opts), clock: clk}
}

func (h *harness) reset() {
	h.now = 0
	h.grants = map[string][]time.Duration{}
}

// recent counts the allowed requests of client inside the current window.
func (h *harness) recent(client string) int {
	n := 0
	for _, at := range h.grants[client] {
		if h.now-at < window {
			n++
		}
	}
	return n
}

// idle lists the clients the limiter should forget: those it remembers
// whose last allowed request is a whole window old.
func (h *harness) idle() []string {
	var idle []string
	for client, log := range h.grants {
		if h.now-log[len(log)-1] >= window {
			idle = append(idle, client)
		}
	}
	slices.Sort(idle)
	return idle
}

func (h *harness) allow() pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.SampledFrom(clients), func(client string) pbt.Command {
		return pbt.Command{Name: "allow", Args: []any{client},
			Run: func(pbt.Env) any { return h.limiter.Allow(client) },
			Next: func(pbt.Var) {
				if h.recent(client) < limit {
					h.grants[client] = append(h.grants[client], h.now)
				}
			},
			Post: func(_ pbt.Env, got any) error {
				recent := h.recent(client)
				if want := recent < limit; got != want {
					return fmt.Errorf("allow(%q) = %v, want %v with %d requests in the last %v", client, got, want, recent, window)
				}
				return nil
			}}
	})
}

func (h *harness) advance() pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.SampledFrom(steps), func(d time.Duration) pbt.Command {
		return pbt.Command{Name: "advance", Args: []any{d.String()},
			Run:  func(pbt.Env) any { h.clock.Advance(d); return nil },
			Next: func(pbt.Var) { h.now += d }}
	})
}

func (h *harness) sweep() pbt.Gen[pbt.Command] {
	return pbt.Just(pbt.Command{Name: "sweep",
		Run: func(pbt.Env) any { return h.limiter.Sweep() },
		Next: func(pbt.Var) {
			for _, c := range h.idle() {
				delete(h.grants, c)
			}
		},
		Post: func(_ pbt.Env, got any) error {
			if idle := h.idle(); got != len(idle) {
				return fmt.Errorf("sweep() forgot %v clients, want %d %v", got, len(idle), idle)
			}
			return nil
		}})
}

func (h *harness) commands() pbt.Gen[pbt.Command] {
	return pbt.Frequency(pbt.W(5, h.allow()), pbt.W(2, h.advance()), pbt.W(1, h.sweep()))
}

func (h *harness) program() pbt.Gen[pbt.Program] {
	return pbt.Commands(h.reset, h.commands, pbt.ModelState(func() any { return h.grants }))
}

func TestLimiter(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		newHarness(Options{}).program().Draw(t, "cmds").RunSequential(t)
	})
}

// The eager sweep forgets a client whose requests are still inside the
// window. The model notices at the sweep itself, which forgets a client
// it should not, and the counterexample is a request, half a window, and
// the sweep.
func TestEagerSweepIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		newHarness(Options{EagerSweep: true}).program().Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "\n") > 4 || !strings.Contains(report, "sweep() forgot 1 clients, want 0") {
		t.Errorf("expected one allow, one advance and the sweep:\n%s", report)
	}
}
