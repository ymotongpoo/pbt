// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"slices"
	"strings"
	"testing"
)

// The shrinker tested by its own properties: whatever the generator and the
// failure condition, the result still fails, is no more complex than the
// first failure, replays to the same values, cannot be improved by any
// single pass again, and shrinking it once more changes nothing.

// shrinkCase is one combination of generator and failure condition that the
// meta-tests draw from.
type shrinkCase struct {
	name string
	prop Property
}

var shrinkCases = []shrinkCase{
	{"int bound", func(t *T) {
		if Int().Draw(t, "x") > 1000 {
			t.Fail()
		}
	}},
	{"slice sum", func(t *T) {
		s := 0
		for _, x := range SliceOf(IntRange(0, 100)).Draw(t, "xs") {
			s += x
		}
		if s >= 50 {
			t.Fail()
		}
	}},
	{"string with a vowel", func(t *T) {
		if strings.ContainsAny(String().Draw(t, "s"), "aeiou") {
			t.Fail()
		}
	}},
	{"nested slices", func(t *T) {
		n := 0
		for _, xs := range SliceOf(SliceOf(Int())).Draw(t, "xss") {
			n += len(xs)
		}
		if n >= 3 {
			t.Fail()
		}
	}},
	{"map and filter", func(t *T) {
		m := MapOf(StringN(1, 3), Filter(Int(), func(x int) bool { return x%3 == 0 })).Draw(t, "m")
		if len(m) >= 2 {
			t.Fail()
		}
	}},
	{"bind", func(t *T) {
		xs := Bind(IntRange(1, 5), func(n int) Gen[[]int] { return SliceOfN(IntRange(-10, 10), n, n) }).Draw(t, "xs")
		if slices.Max(xs)-slices.Min(xs) >= 5 {
			t.Fail()
		}
	}},
	{"struct", func(t *T) {
		p := Make[struct {
			A, B int
			S    []string
		}]().Draw(t, "p")
		if p.A > p.B && len(p.S) > 0 {
			t.Fail()
		}
	}},
	{"commands", func(t *T) {
		type sys struct{}
		prog := Commands(0, func(n int) Gen[Command[int, sys]] {
			return Map(IntRange(-5, 5), func(d int) Command[int, sys] {
				return Cmd[int, sys]{Name: "add", Args: []any{d}, Next: func(n int, _ Var) int { return n + d }}
			})
		}).Draw(t, "cmds")
		n := 0
		for _, s := range prog.Steps() {
			n += s.Command.(Cmd[int, sys]).Args[0].(int)
		}
		if n >= 7 {
			t.Fail()
		}
	}},
}

// firstFailure finds a failing case for a property with a given seed.
func firstFailure(t *testing.T, c shrinkCase, seed uint64) (*runner, *T) {
	t.Helper()
	cfg, err := newConfig([]Option{Seed(seed), Checks(2000), NoSave()})
	if err != nil {
		t.Fatal(err)
	}
	r := &runner{tb: nopTB{}, prop: c.prop, cfg: cfg}
	fail, _, err := r.search()
	if err != nil {
		t.Fatal(err)
	}
	if fail == nil {
		t.Fatalf("%s: no failure found with seed %d", c.name, seed)
	}
	return r, fail
}

func TestShrinkerProperties(t *testing.T) {
	for _, c := range shrinkCases {
		for seed := uint64(1); seed <= 5; seed++ {
			r, fail := firstFailure(t, c, seed)
			minimal, steps := r.shrink(fail)
			name := c.name
			if !minimal.failed || minimal.discarded {
				t.Errorf("%s/%d: the result does not fail", name, seed)
			}
			if simpler(fail.choices, minimal.choices) {
				t.Errorf("%s/%d: the result is more complex than the first failure", name, seed)
			}
			if steps >= maxShrinkSteps {
				t.Errorf("%s/%d: shrinking hit the step limit", name, seed)
			}
			// Replaying the result's choices reproduces the same values.
			replay := r.run(minimal.choices, minimal.size, nil)
			if !slices.Equal(replay.choices, minimal.choices) || formatDraws(replay) != formatDraws(minimal) {
				t.Errorf("%s/%d: replay differs:\n%s\n%s", name, seed, formatDraws(minimal), formatDraws(replay))
			}
			// Shrinking the result again changes nothing.
			again, _ := r.shrink(minimal)
			if !slices.Equal(again.choices, minimal.choices) {
				t.Errorf("%s/%d: shrinking twice gave %v then %v", name, seed, minimal.choices, again.choices)
			}
			// No single deletion of a span or single lowering of a choice
			// to 0 leaves a simpler failing case: the result is a local
			// minimum of the basic moves.
			for _, sp := range minimal.spans {
				cand := slices.Concat(minimal.choices[:sp.start], minimal.choices[sp.end:])
				if u := r.run(cand, minimal.size, nil); u.failed && !u.discarded && simpler(u.choices, minimal.choices) {
					t.Errorf("%s/%d: deleting span %q still fails with a simpler case %v", name, seed, sp.label, u.choices)
				}
			}
			for i, v := range minimal.choices {
				if v == 0 {
					continue
				}
				cand := slices.Clone(minimal.choices)
				cand[i] = 0
				if u := r.run(cand, minimal.size, nil); u.failed && !u.discarded && simpler(u.choices, minimal.choices) {
					t.Errorf("%s/%d: zeroing choice %d still fails with a simpler case %v", name, seed, i, u.choices)
				}
			}
		}
	}
}

func formatDraws(t *T) string {
	var b strings.Builder
	for _, d := range t.draws {
		b.WriteString(formatDraw(d))
		b.WriteByte('\n')
	}
	return b.String()
}

func TestReportLimitCutsLongValues(t *testing.T) {
	tb := &fakeTB{name: "TestLimit"}
	Check(tb, func(t *T) {
		s := StringOfN(Just('x'), 5000, 5000).Draw(t, "s")
		if len(s) > 0 {
			t.Fatalf("long")
		}
	}, Seed(1), NoSave(), ReportLimit(100))
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "more characters; raise the limit with ReportLimit") {
		t.Errorf("report was not cut: %q", tb.errors)
	}
	if strings.Count(tb.errors[0], "x") > 200 {
		t.Errorf("report still holds the whole value")
	}
}
