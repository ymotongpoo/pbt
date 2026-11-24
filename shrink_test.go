// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"slices"
	"testing"
)

// counterexample runs prop with a fixed seed until it fails, shrinks the
// failure and returns the minimal test case.
func counterexample(t *testing.T, prop Property, opts ...Option) *T {
	t.Helper()
	cfg, err := newConfig(append([]Option{Seed(20261031), Checks(2000), NoSave()}, opts...))
	if err != nil {
		t.Fatal(err)
	}
	r := &runner{tb: t, prop: prop, cfg: cfg}
	fail, _, err := r.search()
	if err != nil {
		t.Fatal(err)
	}
	if fail == nil {
		t.Fatal("the property did not fail")
	}
	minimal, _ := r.shrink(fail)
	return minimal
}

func drawn[V any](t *testing.T, c *T, i int) V {
	t.Helper()
	if i >= len(c.draws) {
		t.Fatalf("only %d draws were recorded", len(c.draws))
	}
	v, ok := c.draws[i].value.(V)
	if !ok {
		var want V
		t.Fatalf("draw %d is %T, want %T", i, c.draws[i].value, want)
	}
	return v
}

func TestShrinkIntRangeToBoundary(t *testing.T) {
	c := counterexample(t, func(t *T) {
		if x := IntRange(0, 1000).Draw(t, "x"); x >= 500 {
			t.Fatalf("x = %d", x)
		}
	})
	if got := drawn[int](t, c, 0); got != 500 {
		t.Errorf("got %d, want 500", got)
	}
}

func TestShrinkIntPositive(t *testing.T) {
	c := counterexample(t, func(t *T) {
		if x := Int().Draw(t, "x"); x > 100 {
			t.Fatalf("x = %d", x)
		}
	})
	if got := drawn[int](t, c, 0); got != 101 {
		t.Errorf("got %d, want 101", got)
	}
}

func TestShrinkIntNegative(t *testing.T) {
	c := counterexample(t, func(t *T) {
		if x := Int().Draw(t, "x"); x < -50 {
			t.Fatalf("x = %d", x)
		}
	})
	if got := drawn[int](t, c, 0); got != -51 {
		t.Errorf("got %d, want -51", got)
	}
}

func TestShrinkSliceSum(t *testing.T) {
	c := counterexample(t, func(t *T) {
		xs := SliceOf(IntRange(0, 1000)).Draw(t, "xs")
		sum := 0
		for _, x := range xs {
			sum += x
		}
		if sum >= 100 {
			t.Fatalf("sum = %d", sum)
		}
	})
	if got := drawn[[]int](t, c, 0); !slices.Equal(got, []int{100}) {
		t.Errorf("got %v, want [100]", got)
	}
}

func TestShrinkSliceSumSigned(t *testing.T) {
	c := counterexample(t, func(t *T) {
		xs := SliceOf(Int()).Draw(t, "xs")
		sum := 0
		for _, x := range xs {
			sum += x
		}
		if sum >= 100 {
			t.Fatalf("sum = %d", sum)
		}
	})
	if got := drawn[[]int](t, c, 0); !slices.Equal(got, []int{100}) {
		t.Errorf("got %v, want [100]", got)
	}
}

func TestShrinkUnsorted(t *testing.T) {
	c := counterexample(t, func(t *T) {
		if xs := SliceOf(Int()).Draw(t, "xs"); !slices.IsSorted(xs) {
			t.Fatalf("not sorted: %v", xs)
		}
	})
	if got := drawn[[]int](t, c, 0); !slices.Equal(got, []int{1, 0}) {
		t.Errorf("got %v, want [1 0]", got)
	}
}

func TestShrinkTwoDraws(t *testing.T) {
	c := counterexample(t, func(t *T) {
		a := Int().Draw(t, "a")
		b := Int().Draw(t, "b")
		if a > b {
			t.Fatalf("%d > %d", a, b)
		}
	})
	if a, b := drawn[int](t, c, 0), drawn[int](t, c, 1); a != 1 || b != 0 {
		t.Errorf("got a=%d b=%d, want a=1 b=0", a, b)
	}
}

func TestShrinkDuplicates(t *testing.T) {
	c := counterexample(t, func(t *T) {
		xs := SliceOf(Int()).Draw(t, "xs")
		seen := map[int]bool{}
		for _, x := range xs {
			if seen[x] {
				t.Fatalf("duplicate %d in %v", x, xs)
			}
			seen[x] = true
		}
	})
	if got := drawn[[]int](t, c, 0); !slices.Equal(got, []int{0, 0}) {
		t.Errorf("got %v, want [0 0]", got)
	}
}

func TestShrinkLength(t *testing.T) {
	c := counterexample(t, func(t *T) {
		if xs := SliceOf(Int()).Draw(t, "xs"); len(xs) >= 3 {
			t.Fatalf("len = %d", len(xs))
		}
	})
	if got := drawn[[]int](t, c, 0); !slices.Equal(got, []int{0, 0, 0}) {
		t.Errorf("got %v, want [0 0 0]", got)
	}
}

func TestShrinkString(t *testing.T) {
	c := counterexample(t, func(t *T) {
		if s := String().Draw(t, "s"); len([]rune(s)) >= 2 {
			t.Fatalf("s = %q", s)
		}
	})
	if got := drawn[string](t, c, 0); got != "00" {
		t.Errorf("got %q, want \"00\"", got)
	}
}

func TestShrinkNested(t *testing.T) {
	c := counterexample(t, func(t *T) {
		xss := SliceOf(SliceOf(Int())).Draw(t, "xss")
		n := 0
		for _, xs := range xss {
			n += len(xs)
		}
		if n >= 2 {
			t.Fatalf("%d elements", n)
		}
	})
	if got := drawn[[][]int](t, c, 0); len(got) != 1 || !slices.Equal(got[0], []int{0, 0}) {
		t.Errorf("got %v, want [[0 0]]", got)
	}
}

func TestShrinkBind(t *testing.T) {
	g := Bind(IntRange(1, 10), func(n int) Gen[[]int] { return SliceOfN(Int(), n, n) })
	c := counterexample(t, func(t *T) {
		if xs := g.Draw(t, "xs"); len(xs) >= 3 {
			t.Fatalf("len = %d", len(xs))
		}
	})
	if got := drawn[[]int](t, c, 0); !slices.Equal(got, []int{0, 0, 0}) {
		t.Errorf("got %v, want [0 0 0]", got)
	}
}

func TestShrinkFilter(t *testing.T) {
	even := Filter(Int(), func(x int) bool { return x%2 == 0 })
	c := counterexample(t, func(t *T) {
		if x := even.Draw(t, "x"); x >= 10 {
			t.Fatalf("x = %d", x)
		}
	})
	if got := drawn[int](t, c, 0); got != 10 {
		t.Errorf("got %d, want 10", got)
	}
}

func TestShrinkTo(t *testing.T) {
	year := ShrinkTo(IntRange(0, 9999), Just(1970))
	c := counterexample(t, func(t *T) {
		if y := year.Draw(t, "year"); y >= 1900 {
			t.Fatalf("year = %d", y)
		}
	})
	if got := drawn[int](t, c, 0); got != 1970 {
		t.Errorf("got %d, want 1970 (the ShrinkTo alternative)", got)
	}
}

type tree struct {
	left, right *tree
}

func leaves(n *tree) int {
	if n.left == nil {
		return 1
	}
	return leaves(n.left) + leaves(n.right)
}

func depth(n *tree) int {
	if n.left == nil {
		return 0
	}
	return 1 + max(depth(n.left), depth(n.right))
}

func treeGen(depth int) Gen[*tree] {
	leaf := Just(&tree{})
	if depth == 0 {
		return leaf
	}
	sub := Lazy(func() Gen[*tree] { return treeGen(depth - 1) })
	return Frequency(
		Weighted[*tree]{Weight: 1, Gen: leaf},
		Weighted[*tree]{Weight: 2, Gen: Custom("node", func(t *T) *tree {
			return &tree{left: sub.drawSpan(t), right: sub.drawSpan(t)}
		})},
	)
}

func TestShrinkTreeToSubtree(t *testing.T) {
	gen := Sized(func(size int) Gen[*tree] { return treeGen(min(size, 6)) })
	c := counterexample(t, func(t *T) {
		if n := gen.Draw(t, "tree"); depth(n) >= 3 {
			t.Fatalf("depth %d with %d leaves", depth(n), leaves(n))
		}
	}, Sizes(10, 10))
	got := drawn[*tree](t, c, 0)
	if depth(got) != 3 || leaves(got) != 4 {
		t.Errorf("got depth %d with %d leaves, want depth 3 with 4 leaves", depth(got), leaves(got))
	}
}

func TestShrinkMap(t *testing.T) {
	c := counterexample(t, func(t *T) {
		if m := MapOf(IntRange(0, 100), Bool()).Draw(t, "m"); len(m) >= 2 {
			t.Fatalf("len = %d", len(m))
		}
	})
	got := drawn[map[int]bool](t, c, 0)
	if len(got) != 2 || got[0] || got[1] {
		t.Errorf("got %v, want map[0:false 1:false]", got)
	}
}

func TestPanicIsACounterexample(t *testing.T) {
	c := counterexample(t, func(t *T) {
		xs := SliceOf(Int()).Draw(t, "xs")
		_ = xs[1]
	})
	if got := drawn[[]int](t, c, 0); len(got) != 0 {
		t.Errorf("got %v, want an empty slice", got)
	}
	if c.panicVal == nil {
		t.Error("panic value was not recorded")
	}
}

func TestAssumeGivesUp(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1), Checks(10), NoSave()})
	r := &runner{tb: t, prop: func(t *T) { t.Assume(false) }, cfg: cfg}
	if _, _, err := r.search(); err == nil {
		t.Fatal("expected search to give up on an always-false assumption")
	}
}

func TestSizeGrows(t *testing.T) {
	var sizes []int
	cfg, _ := newConfig([]Option{Seed(1), Checks(5), Sizes(3, 42), NoSave()})
	r := &runner{tb: t, prop: func(t *T) { sizes = append(sizes, Sized(func(size int) Gen[int] { return Just(size) }).Draw(t, "")) }, cfg: cfg}
	if _, _, err := r.search(); err != nil {
		t.Fatal(err)
	}
	if want := []int{3, 4, 5, 6, 7}; !slices.Equal(sizes, want) {
		t.Errorf("sizes = %v, want %v", sizes, want)
	}
}

func TestSeedIsDeterministic(t *testing.T) {
	draw := func() []int {
		var xs []int
		cfg, _ := newConfig([]Option{Seed(7), Checks(20), NoSave()})
		r := &runner{tb: t, prop: func(t *T) { xs = append(xs, Int().Draw(t, "")) }, cfg: cfg}
		r.search()
		return xs
	}
	if a, b := draw(), draw(); !slices.Equal(a, b) {
		t.Errorf("two runs with the same seed drew %v and %v", a, b)
	}
}

func TestZigzagLayout(t *testing.T) {
	for _, r := range []struct{ c, lo, hi int64 }{{0, -3, 7}, {0, -7, 3}, {0, 0, 10}, {0, -10, 0}, {5, 5, 9}, {-2, -9, -2}, {0, -4, 4}} {
		m := zigzagMeta(r.c, r.lo, r.hi)
		seen := map[int64]bool{}
		for u := range uint64(r.hi - r.lo + 1) {
			v := zigzagDecode(u, r.c, r.lo, r.hi)
			if v < r.lo || v > r.hi || seen[v] {
				t.Fatalf("zigzagDecode(%d) in [%d, %d] = %d: out of range or repeated", u, r.lo, r.hi, v)
			}
			seen[v] = true
			if got := r.c + offsetOf(u, m); got != v {
				t.Errorf("offsetOf(%d) in [%d, %d] gives %d, zigzagDecode gives %d", u, r.lo, r.hi, got, v)
			}
			if got := choiceOf(v-r.c, m); got != u {
				t.Errorf("choiceOf(%d) in [%d, %d] = %d, want %d", v-r.c, r.lo, r.hi, got, u)
			}
		}
	}
}

func TestShrinkSwitchAlternatives(t *testing.T) {
	short := SliceOfN(IntRange(0, 9), 1, 1)
	long := SliceOfN(IntRange(0, 9), 8, 8)
	for _, weighted := range []bool{false, true} {
		for _, seed := range []uint64{1, 3, 5} {
			for _, nested := range []bool{false, true} {
				for _, two := range []bool{false, true} {
					alt := OneOf(short, long)
					if weighted {
						alt = Frequency(W(1, short), W(3, long))
					}
					prop := func(pt *T) {
						if nested {
							SliceOfN(alt, 3, 3).Draw(pt, "a")
						} else {
							alt.Draw(pt, "a")
						}
						p := IntRange(0, 100000).Draw(pt, "p")
						q := 1001
						if two {
							q = IntRange(0, 100000).Draw(pt, "q")
						}
						if p > 1000 && q > 1000 {
							pt.Fatalf("%d %d", p, q)
						}
					}
					c := counterexample(t, prop, Seed(seed))
					if nested {
						for _, a := range drawn[[][]int](t, c, 0) {
							if !slices.Equal(a, []int{0}) {
								t.Fatalf("seed %d weighted %v nested %v two %v: %v", seed, weighted, nested, two, a)
							}
						}
					} else if a := drawn[[]int](t, c, 0); !slices.Equal(a, []int{0}) {
						t.Fatalf("seed %d weighted %v two %v: %v", seed, weighted, two, a)
					}
					if drawn[int](t, c, 1) != 1001 || (two && drawn[int](t, c, 2) != 1001) {
						t.Fatalf("wrong tail: %s", drawsLine(c))
					}
					again := counterexample(t, prop, Seed(seed))
					if !slices.Equal(c.choices, again.choices) {
						t.Fatal("shrinking is not deterministic")
					}
				}
			}
		}
	}
}

// The realigned span is named by its sequence number: leaf spans that open
// and close before it, such as those of the Frequency draws here, sit for a
// moment at the index the span had in the old trace.
func TestRealignAfterDroppedLeafSpans(t *testing.T) {
	atom := Custom("atom", func(t *T) string {
		b := make([]byte, IntRange(1, 20).Draw(t, ""))
		for i := range b {
			b[i] = SampledFrom([]byte("abc")).Draw(t, "")
		}
		return string(b)
	})
	g := Custom("outer", func(t *T) string {
		form := Frequency(W(4, Just("")), W(1, Just("!"))).Draw(t, "")
		mode := Frequency(W(15, Just("")), W(1, Just("?"))).Draw(t, "")
		a := atom.Draw(t, "")
		n := IntRange(0, 100).Draw(t, "")
		return form + mode + a + "@" + string(rune('0'+n%10))
	})
	for seed := uint64(1); seed <= 5; seed++ {
		c := counterexample(t, func(pt *T) {
			if s := g.Draw(pt, "s"); s[len(s)-1] != '0' {
				pt.Fail()
			}
		}, Seed(seed))
		if got := drawn[string](t, c, 0); got != "a@1" {
			t.Errorf("seed %d: %q, want %q", seed, got, "a@1")
		}
	}
}
