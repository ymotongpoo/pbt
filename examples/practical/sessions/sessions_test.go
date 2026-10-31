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

// System is the store together with the clock the test controls.
type System struct {
	store *Store
	clock *clock.Clock
}

func newSystem(opts Options) *System {
	clk := clock.New()
	return &System{store: New(clk.Now, ttl, opts), clock: clk}
}

// data is the state data of the machine: the session the test follows.
// The token is a Var, since it is issued when login runs; age is the time
// since the login or the last refresh, which decides when a wait expires
// the session.
type data struct {
	token pbt.Var
	user  string
	age   time.Duration
}

type cmd = pbt.Cmd[data, *System]
type command = pbt.Command[data, *System]
type transition = fsm.Transition[string, data, *System]

var users = []string{"ann", "bo", "cy"}

type used struct {
	user string
	err  error
}

func expectErr(name string, want error) func(data, pbt.Env, any) error {
	return func(_ data, _ pbt.Env, got any) error {
		err, _ := got.(error)
		if !errors.Is(err, want) {
			return fmt.Errorf("%s returned %v, want %v", name, err, want)
		}
		return nil
	}
}

var login = pbt.Map(pbt.SampledFrom(users), func(u string) command {
	return cmd{Name: "login", Args: []any{u},
		Run:  func(s *System, _ pbt.Env) any { return s.store.Login(u) },
		Next: func(_ data, v pbt.Var) data { return data{token: v, user: u} }}
})

// useUnknown presents a token the store never issued.
var useUnknown = pbt.Just[command](cmd{Name: "use", Args: []any{"bogus"},
	Run:  func(s *System, _ pbt.Env) any { _, err := s.store.Use("bogus"); return err },
	Post: expectErr("use", ErrUnknown)})

func use(d data, want error) pbt.Gen[command] {
	return pbt.Just[command](cmd{Name: "use", Args: []any{d.token},
		Run: func(s *System, env pbt.Env) any {
			user, err := s.store.Use(pbt.Resolve[Token](env, d.token))
			return used{user, err}
		},
		Post: func(d data, _ pbt.Env, got any) error {
			r := got.(used)
			if !errors.Is(r.err, want) || r.user != d.user && want == nil {
				return fmt.Errorf("use returned (%q, %v), want (%q, %v)", r.user, r.err, d.user, want)
			}
			return nil
		}})
}

func refresh(d data, want error) pbt.Gen[command] {
	return pbt.Just[command](cmd{Name: "refresh", Args: []any{d.token},
		Run: func(s *System, env pbt.Env) any { return s.store.Refresh(pbt.Resolve[Token](env, d.token)) },
		Next: func(d data, _ pbt.Var) data {
			if want == nil {
				d.age = 0
			}
			return d
		},
		Post: expectErr("refresh", want)})
}

func logout(d data, want error) pbt.Gen[command] {
	return pbt.Just[command](cmd{Name: "logout", Args: []any{d.token},
		Run:  func(s *System, env pbt.Env) any { return s.store.Logout(pbt.Resolve[Token](env, d.token)) },
		Post: expectErr("logout", want)})
}

// wait advances the clock by a whole number of minutes between lo and hi.
func wait(lo, hi time.Duration) pbt.Gen[command] {
	return pbt.Map(pbt.IntRange(int(lo/time.Minute), int(hi/time.Minute)), func(mins int) command {
		d := time.Duration(mins) * time.Minute
		return cmd{Name: "wait", Args: []any{d.String()},
			Run:  func(s *System, _ pbt.Env) any { s.clock.Advance(d); return nil },
			Next: func(st data, _ pbt.Var) data { st.age += d; return st }}
	})
}

// machine names the states and the transitions out of each. Time is a
// command like any other, split by where it leads: a wait that keeps the
// session within its time to live stays in active, a wait that goes past
// it moves to expired.
type machine struct{}

func (machine) Initial() (string, data) { return "anonymous", data{} }

func (machine) Transitions(state string, d data) []transition {
	switch state {
	case "anonymous":
		return []transition{
			{Weight: 3, To: "active", Cmd: login},
			{Weight: 1, To: "anonymous", Cmd: useUnknown},
		}
	case "active":
		left := ttl - d.age
		ts := []transition{
			{Weight: 3, To: "active", Cmd: use(d, nil)},
			{Weight: 2, To: "active", Cmd: refresh(d, nil)},
			{Weight: 2, To: "expired", Cmd: wait(left, left+ttl)},
			{Weight: 1, To: "revoked", Cmd: logout(d, nil)},
		}
		if left > time.Minute {
			ts = append(ts, transition{Weight: 3, To: "active", Cmd: wait(time.Minute, left-time.Minute)})
		}
		return ts
	case "expired":
		return []transition{
			{Weight: 2, To: "expired", Cmd: use(d, ErrExpired)},
			{Weight: 2, To: "expired", Cmd: refresh(d, ErrExpired)},
			{Weight: 1, To: "expired", Cmd: logout(d, ErrExpired)},
			{Weight: 2, To: "active", Cmd: login},
		}
	default: // revoked
		return []transition{
			{Weight: 2, To: "revoked", Cmd: use(d, ErrRevoked)},
			{Weight: 1, To: "revoked", Cmd: refresh(d, ErrRevoked)},
			{Weight: 1, To: "revoked", Cmd: logout(d, ErrRevoked)},
			{Weight: 1, To: "revoked", Cmd: wait(time.Minute, 2*ttl)},
			{Weight: 2, To: "active", Cmd: login},
		}
	}
}

func TestSessions(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		prog := fsm.Commands[string, data, *System](machine{}).Draw(t, "cmds")
		prog.RunSequential(t, newSystem(Options{}))
	})
}

// A refresh that revives an expired session is caught in the expired
// state, where the machine expects ErrExpired. The shortest program is a
// login, a wait past the time to live, and the refresh.
func TestRevivedSessionIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		prog := fsm.Commands[string, data, *System](machine{}).Draw(t, "cmds")
		prog.RunSequential(t, newSystem(Options{ReviveOnRefresh: true}))
	}, pbt.Seed(1))
	program := report[strings.Index(report, "Program{"):strings.Index(report, "}")]
	if strings.Count(program, "\n") > 4 || !strings.Contains(report, "refresh returned <nil>, want sessions: session expired") {
		t.Errorf("expected login, wait and refresh:\n%s", report)
	}
}
