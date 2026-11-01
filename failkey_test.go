// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"testing"
)

// The shrinker keeps the failure it started from: a candidate that fails in
// another way is not a simpler version of the same bug.

// twoBugs fails by panicking for x >= 1000 and by an assertion for x in
// [1, 999]. Without failure identity the panic would shrink to the assertion
// at x = 1, which hides the panic.
func twoBugs(t *T) {
	x := IntRange(0, 2000).Draw(t, "x")
	if x >= 1000 {
		panic("too big")
	}
	if x >= 1 {
		t.Fatalf("positive: %d", x)
	}
}

func TestShrinkKeepsTheFailureIdentity(t *testing.T) {
	panics, asserts := 0, 0
	for seed := uint64(1); seed <= 40; seed++ {
		cfg, _ := newConfig([]Option{Seed(seed), Sizes(42, 42), Checks(100), NoSave()})
		r := &runner{tb: nopTB{}, prop: twoBugs, cfg: cfg}
		fail, _, err := r.search()
		if err != nil || fail == nil {
			t.Fatalf("seed %d: no failure (%v)", seed, err)
		}
		minimal, _ := r.shrink(fail)
		x := minimal.draws[0].value.(int)
		switch {
		case fail.panicVal != nil:
			panics++
			if minimal.panicVal == nil || x != 1000 {
				t.Errorf("seed %d: the panic at x=%d shrank to x=%d, panic %v", seed, fail.draws[0].value, x, minimal.panicVal)
			}
		default:
			asserts++
			if minimal.panicVal != nil || x != 1 {
				t.Errorf("seed %d: the assertion at x=%d shrank to x=%d, panic %v", seed, fail.draws[0].value, x, minimal.panicVal)
			}
		}
	}
	if panics == 0 || asserts == 0 {
		t.Fatalf("both kinds of failure must occur first for some seed: %d panics, %d assertions", panics, asserts)
	}
}

func TestFailureKeysNameTheSite(t *testing.T) {
	run := func(prop Property) *T {
		cfg, _ := newConfig([]Option{Seed(1), NoSave()})
		r := &runner{tb: nopTB{}, prop: prop, cfg: cfg}
		return r.run(nil, 1, nil)
	}
	a := run(func(t *T) { t.Errorf("a") })
	b := run(func(t *T) { t.Errorf("b") })
	if a.failKey == "" || !strings.Contains(a.failKey, "failkey_test.go") {
		t.Errorf("key does not name the test file: %q", a.failKey)
	}
	if a.failKey == b.failKey {
		t.Errorf("two assertion sites share the key %q", a.failKey)
	}
	// The first failure fixes the identity; later failures only add messages.
	var firstLine int
	c := run(func(t *T) {
		_, _, firstLine, _ = runtime.Caller(0)
		t.Errorf("first")
		t.Fatalf("second")
	})
	if want := fmt.Sprintf(":%d", firstLine+1); lineOf(c.failKey) != want || len(c.messages) != 2 {
		t.Errorf("key %q, want line %s; messages %q", c.failKey, want, c.messages)
	}
	// A panic is keyed by its type and site, not by its value.
	p1 := run(func(t *T) { panic(errors.New("one")) })
	p2 := run(func(t *T) { panic(errors.New("two")) })
	p3 := run(func(t *T) {
		for range 2 {
			panic(errors.New("three"))
		}
	})
	if !strings.HasPrefix(p1.failKey, "panic *errors.errorString at ") || !strings.Contains(p1.failKey, "failkey_test.go") {
		t.Errorf("panic key %q", p1.failKey)
	}
	if lineOf(p1.failKey) == lineOf(p2.failKey) {
		t.Errorf("panics at two sites share a line: %q %q", p1.failKey, p2.failKey)
	}
	if lineOf(p3.failKey) == lineOf(p1.failKey) || !strings.Contains(p3.failKey, "failkey_test.go") {
		t.Errorf("panic key %q", p3.failKey)
	}
}

// lineOf returns the trailing :line of a key.
func lineOf(key string) string { return key[strings.LastIndex(key, ":"):] }

// A stateful property has one key per check and command, so that a failing
// postcondition of one command cannot shrink into a panic of another.
func TestStatefulFailuresHaveTheirOwnIdentity(t *testing.T) {
	var errWrong = errors.New("wrong count")
	cfg, _ := newConfig([]Option{Seed(3), NoSave()})
	r := &runner{tb: nopTB{}, prop: func(t *T) {
		var s *miscounter
		m := 0
		inc := Just(Command{Name: "inc",
			Run:  func(Env) any { return s.inc() },
			Next: func(Var) { m++ },
			Post: func(_ Env, got any) error {
				if got.(int) != m+1 {
					return errWrong
				}
				return nil
			}})
		boom := Just(Command{Name: "boom",
			Pre: func() bool { return m >= 2 },
			Run: func(Env) any { panic("boom") }})
		// The system miscounts from the fourth increment on, and boom panics
		// once two increments ran: the panic needs a shorter prefix than the
		// postcondition failure, so a shrinker without identity would trade
		// one for the other.
		gen := func() CommandGen {
			if m >= 2 {
				return OneOf(inc, boom)
			}
			return inc
		}
		prog := Commands(func() { m = 0 }, gen).Draw(t, "cmds")
		s = &miscounter{}
		prog.RunSequential(t)
	}, cfg: cfg}
	fail, _, err := r.search()
	if err != nil || fail == nil {
		t.Fatalf("no failure (%v)", err)
	}
	if !strings.HasPrefix(fail.failKey, "panic string at ") && fail.failKey != "postcondition of inc" {
		t.Fatalf("key %q", fail.failKey)
	}
	minimal, _ := r.shrink(fail)
	if minimal.failKey != fail.failKey {
		t.Errorf("shrinking changed the failure from %q to %q", fail.failKey, minimal.failKey)
	}
	isPanic := strings.HasPrefix(fail.failKey, "panic ")
	if isPanic && !strings.Contains(formatDraws(minimal), "boom") {
		t.Errorf("the minimal case lost the panicking command:\n%s", formatDraws(minimal))
	}
	if !isPanic && strings.Contains(formatDraws(minimal), "boom") {
		t.Errorf("the minimal case traded the postcondition failure for the panic:\n%s", formatDraws(minimal))
	}
	t.Logf("first failure %q, minimal:\n%s", fail.failKey, formatDraws(minimal))
}

// miscounter miscounts from the fourth increment on.
type miscounter struct{ n int }

func (c *miscounter) inc() int {
	c.n++
	if c.n >= 4 {
		return c.n + 1
	}
	return c.n
}

func TestConjunctionKeepsTheFirstFailure(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1), NoSave()})
	run := func(prop Property) *T {
		r := &runner{tb: nopTB{}, prop: prop, cfg: cfg}
		return r.run(nil, 1, nil)
	}
	var firstLine int
	c := run(Conjunction(
		Named{"a", func(t *T) { _, _, firstLine, _ = runtime.Caller(0); t.Errorf("a") }},
		Named{"b", func(t *T) { t.Errorf("b") }},
	))
	if want := fmt.Sprintf(":%d", firstLine); lineOf(c.failKey) != want {
		t.Errorf("key %q, want line %s", c.failKey, want)
	}
	p := run(Conjunction(Named{"a", func(t *T) { panic("sub") }}))
	if !strings.HasPrefix(p.failKey, "panic string at ") || !strings.Contains(p.failKey, "failkey_test.go") {
		t.Errorf("key %q", p.failKey)
	}
}
