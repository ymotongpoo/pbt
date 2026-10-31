// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestStatsAreLogged(t *testing.T) {
	tb := &fakeTB{name: "TestStats"}
	Check(tb, func(t *T) {
		x := IntRange(0, 9).Draw(t, "x")
		t.Classify(x < 5, "small")
		t.Collect(x % 2)
		t.Measure("x", float64(x))
	}, Seed(1), NoSave())
	if len(tb.errors) != 0 {
		t.Fatalf("unexpected errors: %q", tb.errors)
	}
	logs := strings.Join(tb.logs, "\n")
	for _, want := range []string{"OK, passed 100 checks", "% small", "% 0", "% 1", "x: min 0, avg "} {
		if !strings.Contains(logs, want) {
			t.Errorf("statistics lack %q:\n%s", want, logs)
		}
	}
}

func TestCoverWarnsOrFails(t *testing.T) {
	impossible := func(t *T) {
		x := IntRange(0, 9).Draw(t, "x")
		t.Cover(x > 100, 50, "huge")
	}
	tb := &fakeTB{name: "TestCover"}
	Check(tb, impossible, Seed(1), NoSave())
	if len(tb.errors) != 0 || !strings.Contains(strings.Join(tb.logs, "\n"), "insufficient coverage") {
		t.Errorf("without CheckCoverage: errors %q, logs %q", tb.errors, tb.logs)
	}
	tb2 := &fakeTB{name: "TestCover"}
	Check(tb2, impossible, Seed(1), NoSave(), CheckCoverage())
	if len(tb2.errors) != 1 || !strings.Contains(tb2.errors[0], "huge: 0.0% of 100 cases, required 50.0%") {
		t.Errorf("with CheckCoverage: errors %q", tb2.errors)
	}
	tb3 := &fakeTB{name: "TestCover"}
	Check(tb3, func(t *T) {
		x := IntRange(0, 9).Draw(t, "x")
		t.Cover(x < 5, 20, "small")
	}, Seed(1), NoSave(), CheckCoverage())
	if len(tb3.errors) != 0 {
		t.Errorf("a satisfied requirement failed: %q", tb3.errors)
	}
}

func TestConjunctionNamesFailures(t *testing.T) {
	c := counterexample(t, Conjunction(
		Named{"always", func(t *T) { Int().Draw(t, "x") }},
		Named{"never", func(t *T) { t.Errorf("nope") }},
		Named{"fatal", func(t *T) { t.Fatalf("stop") }},
	))
	msgs := strings.Join(c.messages, "\n")
	for _, want := range []string{"never: nope", "fatal: stop", "conjunction: [never fatal] failed"} {
		if !strings.Contains(msgs, want) {
			t.Errorf("messages lack %q:\n%s", want, msgs)
		}
	}
	Check(t, Conjunction(
		Named{"a", func(t *T) { Int().Draw(t, "x") }},
		Named{"b", func(t *T) {}},
	), NoSave(), Checks(10))
}

func TestTimeout(t *testing.T) {
	c := counterexample(t, Timeout(20*time.Millisecond, func(t *T) {
		ms := IntRange(0, 60).Draw(t, "ms")
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}), Checks(100))
	if ms := drawn[int](t, c, 0); ms < 10 {
		t.Errorf("timed out at %d ms", ms)
	}
	if !strings.Contains(strings.Join(c.messages, "\n"), "longer than the limit of 20ms") {
		t.Errorf("messages: %q", c.messages)
	}
	Check(t, Timeout(time.Second, func(t *T) { Int().Draw(t, "x") }), NoSave(), Checks(10))
}

func TestFuncIsPureAndShrinks(t *testing.T) {
	c := counterexample(t, func(t *T) {
		f := Func[int, int](Int()).Draw(t, "f")
		if f.Call(3) != f.Call(3) {
			t.Fatalf("impure")
		}
		if f.Call(3) > 10 {
			t.Fatalf("f(3) = %d", f.Call(3))
		}
	})
	f := drawn[*Fn[int, int]](t, c, 0)
	if f.Call(3) != 11 {
		t.Errorf("f(3) = %d, want 11", f.Call(3))
	}
	if got := f.String(); got != "{3 -> 11}" {
		t.Errorf("String() = %q", got)
	}
}

func TestFunc2(t *testing.T) {
	Check(t, func(t *T) {
		f := Func2[int, string](Bool()).Draw(t, "f")
		if f.Call(1, "a") != f.Call(1, "a") {
			t.Fatalf("impure")
		}
		f.Call(2, "b")
		if n := strings.Count(f.String(), "->"); n != 2 {
			t.Fatalf("String() = %q", f.String())
		}
	}, NoSave(), Checks(20))
}

func TestNoShrinkKeepsValue(t *testing.T) {
	first := -1
	c := counterexample(t, func(t *T) {
		x := NoShrink(IntRange(0, 1000)).Draw(t, "x")
		if x >= 10 {
			if first < 0 {
				first = x
			}
			t.Fatalf("x = %d", x)
		}
	})
	if got := drawn[int](t, c, 0); got != first {
		t.Errorf("NoShrink value changed from %d to %d", first, got)
	}
}

func TestLetShrinkToPart(t *testing.T) {
	part := SliceOf(Int())
	g := LetShrink([]Gen[[]int]{part, part, part}, func(ps [][]int) []int { return slices.Concat(ps...) })
	c := counterexample(t, func(t *T) {
		if xs := g.Draw(t, "xs"); len(xs) >= 2 {
			t.Fatalf("len %d", len(xs))
		}
	})
	if got := drawn[[]int](t, c, 0); !slices.Equal(got, []int{0, 0}) {
		t.Errorf("got %v, want [0 0]", got)
	}
	if c.choices[0] >= 3 {
		t.Errorf("the result was combined (choice %d) instead of shrinking to a part", c.choices[0])
	}
}

func TestSampleShrink(t *testing.T) {
	var vals []int
	for v := range SampleShrink(IntRange(-100, 100), 1) {
		vals = append(vals, v)
	}
	if len(vals) == 0 || vals[len(vals)-1] != 0 {
		t.Errorf("shrink sequence %v does not end at 0", vals)
	}
	n := 0
	for range SampleShrink(SliceOf(Int()), 2) {
		n++
		break
	}
	if n != 1 {
		t.Errorf("stopping early yielded %d values", n)
	}
}

func TestPermutation(t *testing.T) {
	Check(t, func(t *T) {
		p := Permutation([]int{1, 2, 3, 4}).Draw(t, "p")
		sorted := slices.Clone(p)
		slices.Sort(sorted)
		if !slices.Equal(sorted, []int{1, 2, 3, 4}) {
			t.Fatalf("%v is not a permutation", p)
		}
	}, NoSave())
	c := counterexample(t, func(t *T) {
		if p := Permutation([]int{1, 2, 3}).Draw(t, "p"); !slices.Equal(p, []int{1, 2, 3}) {
			t.Fatalf("%v", p)
		}
	})
	if got := drawn[[]int](t, c, 0); !slices.Equal(got, []int{2, 1, 3}) {
		t.Errorf("got %v, want [2 1 3]", got)
	}
}

func TestPtr(t *testing.T) {
	c := counterexample(t, func(t *T) {
		Ptr(Int(), true).Draw(t, "p")
		t.Fail()
	})
	if p := drawn[*int](t, c, 0); p != nil {
		t.Errorf("got %v, want nil", *p)
	}
	Check(t, func(t *T) {
		if Ptr(Int(), false).Draw(t, "p") == nil {
			t.Fatalf("nil pointer")
		}
	}, NoSave())
}

func TestAnyShrinksToZero(t *testing.T) {
	c := counterexample(t, func(t *T) {
		Any().Draw(t, "v")
		t.Fail()
	})
	if v := drawn[any](t, c, 0); v != 0 {
		t.Errorf("got %#v, want 0", v)
	}
	Check(t, func(t *T) { Any().Draw(t, "v") }, NoSave())
}

func TestSetupAndTeardown(t *testing.T) {
	setups, teardowns := 0, 0
	Check(t, func(t *T) {
		Int().Draw(t, "x")
		if teardowns != setups-1 {
			t.Fatalf("teardown of the previous case did not run")
		}
	}, Checks(5), NoSave(), Setup(func(t *T) func() {
		setups++
		return func() { teardowns++ }
	}))
	if setups != 5 || teardowns != 5 {
		t.Errorf("setups %d, teardowns %d", setups, teardowns)
	}
}

func TestRecover(t *testing.T) {
	if got := Recover(func() { panic("boom") }); got != "boom" {
		t.Errorf("got %v", got)
	}
	if got := Recover(func() {}); got != nil {
		t.Errorf("got %v", got)
	}
}

func TestIntRangeToward(t *testing.T) {
	c := counterexample(t, func(t *T) {
		if y := IntRangeToward(0, 9999, 1970).Draw(t, "y"); y >= 1900 {
			t.Fatalf("y = %d", y)
		}
	})
	if got := drawn[int](t, c, 0); got != 1970 {
		t.Errorf("got %d, want 1970", got)
	}
}

func TestOnFailRunsForTheReport(t *testing.T) {
	tb := &fakeTB{name: "TestOnFail"}
	calls := 0
	Check(tb, func(t *T) {
		x := IntRange(0, 100).Draw(t, "x")
		t.OnFail(func() {
			calls++
			t.Logf("onfail x=%d", x)
		})
		if x >= 50 {
			t.Fatalf("big")
		}
	}, Seed(1), NoSave())
	if calls != 1 {
		t.Errorf("OnFail ran %d times, want once", calls)
	}
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "log: onfail x=50") {
		t.Errorf("report: %q", tb.errors)
	}
}

func TestBytesN(t *testing.T) {
	Check(t, func(t *T) {
		if b := BytesN(2, 4).Draw(t, "b"); len(b) < 2 || len(b) > 4 {
			t.Fatalf("len %d", len(b))
		}
	}, NoSave())
}
