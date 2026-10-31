// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"math"
	"strings"
	"sync"
	"testing"
)

// Regression tests for the defects found in the pre-release review.

func TestFloatRangesStayWithinBounds(t *testing.T) {
	ranges := [][2]float64{
		{1 << 54, 1 << 54}, {0.1, 0.2}, {-1e300, -1e299}, {1e308, math.MaxFloat64},
		{-0.5, 0.5}, {3, 3}, {-1 << 60, 1 << 60}, {1e-310, 1e-300}, {-7, -2.5},
	}
	for _, r := range ranges {
		g := Float64Range(r[0], r[1])
		Check(t, func(t *T) {
			if v := g.Draw(t, "v"); v < r[0] || v > r[1] || math.IsNaN(v) {
				t.Fatalf("%v is outside [%v, %v]", v, r[0], r[1])
			}
		}, NoSave(), Checks(300))
	}
}

func TestParallelBranchesResolveTheirOwnVars(t *testing.T) {
	Check(t, func(t *T) {
		var mu sync.Mutex
		ids := 0
		var vars []Var
		register := Just(Command{Name: "register",
			Run: func(Env) any {
				mu.Lock()
				defer mu.Unlock()
				ids++
				return ids
			},
			Next: func(v Var) { vars = append(vars, v) }})
		gen := func() Gen[Command] {
			if len(vars) == 0 {
				return register
			}
			lookup := Map(SampledFrom(vars), func(v Var) Command {
				return Command{Name: "lookup", Args: []any{v},
					Run: func(env Env) any { return Resolve[int](env, v) > 0 },
					Post: func(_ Env, got any) error {
						if got != true {
							return errNotPositive
						}
						return nil
					}}
			})
			return OneOf(register, lookup)
		}
		prog := ParallelCommands(func() { vars = nil }, gen).Draw(t, "cmds")
		prog.RunParallel(t)
	}, NoSave(), Checks(200))
}

var errNotPositive = &parallelError{"id is not positive"}

type parallelError struct{ s string }

func (e *parallelError) Error() string { return e.s }

func TestParallelDiscardIsRaisedOnTheMainGoroutine(t *testing.T) {
	skip := Just(Command{Name: "skip",
		Run: func(Env) any { panic(discardTest{reason: "not applicable"}) }})
	cfg, _ := newConfig([]Option{Seed(1), Checks(5), NoSave()})
	r := &runner{tb: t, prop: func(t *T) {
		prog := ParallelCommands(nil, func() Gen[Command] { return skip }, Branches(2, 2)).Draw(t, "cmds")
		prog.RunParallel(t)
	}, cfg: cfg}
	if _, _, err := r.search(); err == nil || !strings.Contains(err.Error(), "discarded") {
		t.Errorf("expected the run to give up on discards, got %v", err)
	}
}

func TestParallelBranchCountIsCapped(t *testing.T) {
	defer func() {
		if rec := recover(); rec == nil {
			t.Error("four branches were accepted")
		}
	}()
	ParallelCommands(nil, NoCommands, Branches(4, 1))
}

func TestFailureBeatsDiscard(t *testing.T) {
	tb := &fakeTB{name: "TestFailureBeatsDiscard"}
	Check(tb, func(t *T) {
		Int().Draw(t, "x")
		t.Error("broken")
		t.Assume(false)
	}, Seed(1), NoSave(), Checks(10))
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "error: broken") {
		t.Errorf("the failure was not reported: %q", tb.errors)
	}
}

func TestCleanupsRunToCompletion(t *testing.T) {
	ran := 0
	tb := &fakeTB{name: "TestCleanups"}
	Check(tb, func(t *T) {
		t.Cleanup(func() { ran++ })
		t.Cleanup(func() { t.Cleanup(func() { ran++ }) }) // registered during cleanup
		t.Cleanup(func() { panic("first cleanup panics") })
	}, Seed(1), NoSave(), Checks(1))
	if ran != 2 {
		t.Errorf("later cleanups ran %d times, want 2", ran)
	}
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "first cleanup panics") {
		t.Errorf("the cleanup panic was not reported: %q", tb.errors)
	}
}

func TestExampleRunsCleanups(t *testing.T) {
	cleaned := false
	g := Custom("resource", func(t *T) int {
		t.Cleanup(func() { cleaned = true })
		return Int().Draw(t, "")
	})
	g.Example(1)
	if !cleaned {
		t.Error("Example did not run the Cleanup callback")
	}
}

func TestMakeTagBoundsAreExact(t *testing.T) {
	type rec struct {
		ID int64 `pbt:"min=9007199254740993,max=9007199254740993"`
	}
	if v := Make[rec]().Example(1); v.ID != 9007199254740993 {
		t.Errorf("got %d, want 9007199254740993", v.ID)
	}
	defer func() {
		if rec := recover(); rec == nil || !strings.Contains(rec.(string), "does not fit") {
			t.Errorf("an overflowing bound was accepted: %v", rec)
		}
	}()
	Make[struct {
		N uint8 `pbt:"max=300"`
	}]()
}

func TestMakeHandlesNilInterfaceValues(t *testing.T) {
	if v := Make[any](WithType(Just[any](nil))).Example(1); v != nil {
		t.Errorf("got %v, want nil", v)
	}
	type holder struct{ Err error }
	h := Make[holder](WithType(Just[error](nil))).Example(1)
	if h.Err != nil {
		t.Errorf("got %v, want nil", h.Err)
	}
}

func TestMakeRejectsUnknownFieldOverrides(t *testing.T) {
	defer func() {
		if rec := recover(); rec == nil || !strings.Contains(rec.(string), "does not exist") {
			t.Errorf("an unknown field override was accepted: %v", rec)
		}
	}()
	Make[record](WithField[record]("Nope", Int()))
}

func TestMakeRecursiveGeneratorIsSafeToShare(t *testing.T) {
	g := Make[*node]()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.Example(uint64(i))
		}()
	}
	wg.Wait()
}

func TestZeroGenPanicsClearly(t *testing.T) {
	defer func() {
		if rec := recover(); rec == nil || !strings.Contains(rec.(string), "uninitialized generator") {
			t.Errorf("got %v", rec)
		}
	}()
	var g Gen[int]
	g.Example(1)
}

func TestUninitializedCommandGeneratorIsRejected(t *testing.T) {
	defer func() {
		if rec := recover(); rec == nil || !strings.Contains(rec.(string), "NoCommands") {
			t.Errorf("got %v", rec)
		}
	}()
	Commands(nil, func() Gen[Command] { return Gen[Command]{} }).Example(1)
}

func TestAsAnyWidens(t *testing.T) {
	g := OneOf(AsAny(Int()), AsAny(String()))
	ints, strs := 0, 0
	Check(t, func(t *T) {
		switch g.Draw(t, "v").(type) {
		case int:
			ints++
		case string:
			strs++
		default:
			t.Fatalf("unexpected type")
		}
	}, NoSave())
	if ints == 0 || strs == 0 {
		t.Errorf("ints %d, strings %d", ints, strs)
	}
}

func TestSizeZeroMeansEmptyCollections(t *testing.T) {
	Check(t, func(t *T) {
		if xs := Resize(0, SliceOf(Int())).Draw(t, "xs"); len(xs) != 0 {
			t.Fatalf("slice of %d elements at size 0", len(xs))
		}
		if m := Resize(0, MapOf(Int(), Int())).Draw(t, "m"); len(m) != 0 {
			t.Fatalf("map of %d entries at size 0", len(m))
		}
		if s := Resize(0, String()).Draw(t, "s"); s != "" {
			t.Fatalf("string %q at size 0", s)
		}
		// A minimum length still forces elements.
		if ys := Resize(0, SliceOfN(Int(), 2, 5)).Draw(t, "ys"); len(ys) != 2 {
			t.Fatalf("slice of %d elements at size 0 with min 2", len(ys))
		}
	}, NoSave(), Checks(100))

	// Sizes may start at 0, and the first cases are then empty.
	tb := &fakeTB{name: "TestSizes"}
	Check(tb, func(t *T) {
		xs := SliceOf(Int()).Draw(t, "xs")
		if t.Size() == 0 && len(xs) != 0 {
			t.Fatalf("size %d gave %d elements", t.Size(), len(xs))
		}
	}, NoSave(), Sizes(0, 5), Checks(50))
	if len(tb.errors) != 0 {
		t.Fatalf("%q", tb.errors)
	}
	if _, err := newConfig([]Option{Sizes(-1, 5)}); err == nil {
		t.Error("Sizes(-1, 5) was accepted")
	}
}

func TestAnyFloat64CoversTheSpecialValues(t *testing.T) {
	var nan, posInf, negInf, finite int
	Check(t, func(t *T) {
		switch f := AnyFloat64().Draw(t, "f"); {
		case math.IsNaN(f):
			nan++
		case math.IsInf(f, 1):
			posInf++
		case math.IsInf(f, -1):
			negInf++
		default:
			finite++
		}
	}, NoSave(), Checks(600))
	if nan == 0 || posInf == 0 || negInf == 0 || finite < 300 {
		t.Fatalf("nan %d, +inf %d, -inf %d, finite %d", nan, posInf, negInf, finite)
	}
	// Special values shrink to a finite one, which is the first alternative.
	tb := &fakeTB{name: "TestAnyFloat64"}
	Check(tb, func(t *T) {
		if f := AnyFloat64().Draw(t, "f"); f != f || f < -10 {
			t.Fatalf("f = %v", f)
		}
	}, NoSave(), Seed(1), Checks(600))
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "f (AnyFloat64()) = ") {
		t.Fatalf("%q", tb.errors)
	}
}
