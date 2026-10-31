// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ratelimit

import (
	"fmt"
	"maps"
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

// System is the limiter together with the clock the test controls.
type System struct {
	limiter *Limiter
	clock   *clock.Clock
}

func newSystem(opts Options) *System {
	clk := clock.New()
	return &System{limiter: New(clk.Now, limit, window, opts), clock: clk}
}

// model is the time elapsed since the start and every request that was
// allowed, per client. It never prunes: the sequences are short, and the
// rule is easier to read over the whole history.
type model struct {
	now    time.Duration
	grants map[string][]time.Duration
}

// recent counts the allowed requests of client inside the current window.
func (m model) recent(client string) int {
	n := 0
	for _, at := range m.grants[client] {
		if m.now-at < window {
			n++
		}
	}
	return n
}

// idle lists the clients the limiter should forget: those it remembers
// whose last allowed request is a whole window old.
func (m model) idle() []string {
	var idle []string
	for client, log := range m.grants {
		if m.now-log[len(log)-1] >= window {
			idle = append(idle, client)
		}
	}
	slices.Sort(idle)
	return idle
}

func (m model) grant(client string) model {
	grants := maps.Clone(m.grants)
	grants[client] = append(slices.Clone(grants[client]), m.now)
	return model{m.now, grants}
}

func (m model) forget(clients []string) model {
	grants := maps.Clone(m.grants)
	for _, c := range clients {
		delete(grants, c)
	}
	return model{m.now, grants}
}

type cmd = pbt.Cmd[model, *System]
type command = pbt.Command[model, *System]

var allow = pbt.Map(pbt.SampledFrom(clients), func(client string) command {
	return cmd{Name: "allow", Args: []any{client},
		Run: func(s *System, _ pbt.Env) any { return s.limiter.Allow(client) },
		Next: func(m model, _ pbt.Var) model {
			if m.recent(client) < limit {
				return m.grant(client)
			}
			return m
		},
		Post: func(m model, _ pbt.Env, got any) error {
			recent := m.recent(client)
			if want := recent < limit; got != want {
				return fmt.Errorf("allow(%q) = %v, want %v with %d requests in the last %v", client, got, want, recent, window)
			}
			return nil
		}}
})

var advance = pbt.Map(pbt.SampledFrom(steps), func(d time.Duration) command {
	return cmd{Name: "advance", Args: []any{d.String()},
		Run:  func(s *System, _ pbt.Env) any { s.clock.Advance(d); return nil },
		Next: func(m model, _ pbt.Var) model { m.now += d; return m }}
})

var sweep = pbt.Just[command](cmd{Name: "sweep",
	Run:  func(s *System, _ pbt.Env) any { return s.limiter.Sweep() },
	Next: func(m model, _ pbt.Var) model { return m.forget(m.idle()) },
	Post: func(m model, _ pbt.Env, got any) error {
		if idle := m.idle(); got != len(idle) {
			return fmt.Errorf("sweep() forgot %v clients, want %d %v", got, len(idle), idle)
		}
		return nil
	}})

func commands(model) pbt.Gen[command] {
	return pbt.Frequency(
		pbt.Weighted[command]{Weight: 5, Gen: allow},
		pbt.Weighted[command]{Weight: 2, Gen: advance},
		pbt.Weighted[command]{Weight: 1, Gen: sweep},
	)
}

func TestLimiter(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := pbt.Commands(model{grants: map[string][]time.Duration{}}, commands).Draw(t, "cmds")
		prog.RunSequential(t, newSystem(Options{}))
	})
}

// The eager sweep forgets a client whose requests are still inside the
// window. The model notices at the sweep itself, which forgets a client
// it should not, and the counterexample is a request, half a window, and
// the sweep.
func TestEagerSweepIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := pbt.Commands(model{grants: map[string][]time.Duration{}}, commands).Draw(t, "cmds")
		prog.RunSequential(t, newSystem(Options{EagerSweep: true}))
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "\n") > 4 || !strings.Contains(report, "sweep() forgot 1 clients, want 0") {
		t.Errorf("expected one allow, one advance and the sweep:\n%s", report)
	}
}
