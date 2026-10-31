// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package sessions

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/clock"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
	"go.ymotongpoo.com/pbt/fsm"
)

const ttl = 30 * time.Minute

// harness holds the store under test, the clock the test controls, and
// the state data of the machine: the session the test follows. The token
// is a Var, since it is issued when login runs; age is the time since the
// login or the last refresh, which decides when a wait expires the
// session. The state name is the machine's.
type harness struct {
	store *Store
	clock *clock.Clock
	token pbt.Var
	user  string
	age   time.Duration
}

func newHarness(opts Options) *harness {
	clk := clock.New()
	return &harness{store: New(clk.Now, ttl, opts), clock: clk}
}

func (h *harness) reset() { h.token, h.user, h.age = pbt.Var{}, "", 0 }

func (h *harness) data() any { return fmt.Sprintf("%v %s age=%v", h.token, h.user, h.age) }

var users = []string{"ann", "bo", "cy"}

type used struct {
	user string
	err  error
}

func expectErr(name string, want error) func(pbt.Env, any) error {
	return func(_ pbt.Env, got any) error {
		err, _ := got.(error)
		if !errors.Is(err, want) {
			return fmt.Errorf("%s returned %v, want %v", name, err, want)
		}
		return nil
	}
}

func (h *harness) login() pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.SampledFrom(users), func(u string) pbt.Command {
		return pbt.Command{Name: "login", Args: []any{u},
			Run:  func(pbt.Env) any { return h.store.Login(u) },
			Next: func(v pbt.Var) { h.token, h.user, h.age = v, u, 0 }}
	})
}

// useUnknown presents a token the store never issued.
func (h *harness) useUnknown() pbt.Gen[pbt.Command] {
	return pbt.Just(pbt.Command{Name: "use", Args: []any{"bogus"},
		Run:  func(pbt.Env) any { _, err := h.store.Use("bogus"); return err },
		Post: expectErr("use", ErrUnknown)})
}

func (h *harness) use(want error) pbt.Gen[pbt.Command] {
	return pbt.Just(pbt.Command{Name: "use", Args: []any{h.token},
		Run: func(env pbt.Env) any {
			user, err := h.store.Use(pbt.Resolve[Token](env, h.token))
			return used{user, err}
		},
		Post: func(_ pbt.Env, got any) error {
			r := got.(used)
			if !errors.Is(r.err, want) || r.user != h.user && want == nil {
				return fmt.Errorf("use returned (%q, %v), want (%q, %v)", r.user, r.err, h.user, want)
			}
			return nil
		}})
}

func (h *harness) refresh(want error) pbt.Gen[pbt.Command] {
	return pbt.Just(pbt.Command{Name: "refresh", Args: []any{h.token},
		Run: func(env pbt.Env) any { return h.store.Refresh(pbt.Resolve[Token](env, h.token)) },
		Next: func(pbt.Var) {
			if want == nil {
				h.age = 0
			}
		},
		Post: expectErr("refresh", want)})
}

func (h *harness) logout(want error) pbt.Gen[pbt.Command] {
	return pbt.Just(pbt.Command{Name: "logout", Args: []any{h.token},
		Run:  func(env pbt.Env) any { return h.store.Logout(pbt.Resolve[Token](env, h.token)) },
		Post: expectErr("logout", want)})
}

// wait advances the clock by a whole number of minutes between lo and hi.
func (h *harness) wait(lo, hi time.Duration) pbt.Gen[pbt.Command] {
	return pbt.Map(pbt.IntRange(int(lo/time.Minute), int(hi/time.Minute)), func(mins int) pbt.Command {
		d := time.Duration(mins) * time.Minute
		return pbt.Command{Name: "wait", Args: []any{d.String()},
			Run:  func(pbt.Env) any { h.clock.Advance(d); return nil },
			Next: func(pbt.Var) { h.age += d }}
	})
}

// transitions names the states and the transitions out of each. Time is a
// command like any other, split by where it leads: a wait that keeps the
// session within its time to live stays in active, a wait that goes past
// it moves to expired.
func (h *harness) transitions(state string) []fsm.Transition {
	switch state {
	case "anonymous":
		return []fsm.Transition{
			{Weight: 3, To: "active", Cmd: h.login()},
			{Weight: 1, To: "anonymous", Cmd: h.useUnknown()},
		}
	case "active":
		left := ttl - h.age
		ts := []fsm.Transition{
			{Weight: 3, To: "active", Cmd: h.use(nil)},
			{Weight: 2, To: "active", Cmd: h.refresh(nil)},
			{Weight: 2, To: "expired", Cmd: h.wait(left, left+ttl)},
			{Weight: 1, To: "revoked", Cmd: h.logout(nil)},
		}
		if left > time.Minute {
			ts = append(ts, fsm.Transition{Weight: 3, To: "active", Cmd: h.wait(time.Minute, left-time.Minute)})
		}
		return ts
	case "expired":
		return []fsm.Transition{
			{Weight: 2, To: "expired", Cmd: h.use(ErrExpired)},
			{Weight: 2, To: "expired", Cmd: h.refresh(ErrExpired)},
			{Weight: 1, To: "expired", Cmd: h.logout(ErrExpired)},
			{Weight: 2, To: "active", Cmd: h.login()},
		}
	default: // revoked
		return []fsm.Transition{
			{Weight: 2, To: "revoked", Cmd: h.use(ErrRevoked)},
			{Weight: 1, To: "revoked", Cmd: h.refresh(ErrRevoked)},
			{Weight: 1, To: "revoked", Cmd: h.logout(ErrRevoked)},
			{Weight: 1, To: "revoked", Cmd: h.wait(time.Minute, 2*ttl)},
			{Weight: 2, To: "active", Cmd: h.login()},
		}
	}
}

func (h *harness) program() pbt.Gen[pbt.Program] {
	return fsm.Commands("anonymous", h.reset, h.transitions, pbt.ModelState(h.data))
}

func TestSessions(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		newHarness(Options{}).program().Draw(t, "cmds").RunSequential(t)
	})
}

// A refresh that revives an expired session is caught in the expired
// state, where the machine expects ErrExpired. The shortest program is a
// login, a wait past the time to live, and the refresh.
func TestRevivedSessionIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		newHarness(Options{ReviveOnRefresh: true}).program().Draw(t, "cmds").RunSequential(t)
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "\n") > 4 || !strings.Contains(report, "refresh returned <nil>, want sessions: session expired") {
		t.Errorf("expected login, wait and refresh:\n%s", report)
	}
}
