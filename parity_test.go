// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"math"
	"slices"
	"testing"
)

// These properties mirror the shrink quality checks that rapid, which
// shares the choice-sequence method, applies to its own engine. The
// expected counterexamples are the ones this library's ordering produces:
// integers shrink to the nearest bound, floats to integral values first,
// strings to the first rune of their alphabet.

func TestParityIntComparisons(t *testing.T) {
	cases := []struct {
		name string
		fail func(int) bool
		want int
	}{
		{"gt million", func(i int) bool { return i > 1000000 }, 1000001},
		{"gt minus million", func(i int) bool { return i > -1000000 }, 0},
		{"ge zero", func(i int) bool { return i >= 0 }, 0},
		{"lt million", func(i int) bool { return i < 1000000 }, 0},
		{"lt minus million", func(i int) bool { return i < -1000000 }, -1000001},
		{"le zero", func(i int) bool { return i <= 0 }, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := counterexample(t, func(t *T) {
				if i := Int().Draw(t, "i"); c.fail(i) {
					t.Fatalf("%d", i)
				}
			})
			if got := drawn[int](t, r, 0); got != c.want {
				t.Errorf("got %d, want %d", got, c.want)
			}
		})
	}
}

func TestParityFloatComparisons(t *testing.T) {
	cases := []struct {
		name string
		fail func(float64) bool
		want float64
	}{
		{"gt million", func(f float64) bool { return f > 1000000 }, 1000001},
		{"gt pi", func(f float64) bool { return f > math.Pi }, 4},
		{"ge one", func(f float64) bool { return f >= 1 }, 1},
		{"gt minus million", func(f float64) bool { return f > -1000000 }, 0},
		{"lt minus million", func(f float64) bool { return f < -1000000 }, -1000001},
		{"lt minus e", func(f float64) bool { return f < -math.E }, -3},
		{"le zero", func(f float64) bool { return f <= 0 }, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := counterexample(t, func(t *T) {
				if f := Float64().Draw(t, "f"); c.fail(f) {
					t.Fatalf("%v", f)
				}
			})
			if got := drawn[float64](t, r, 0); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestParityFloatFractions(t *testing.T) {
	// A property that only fails on non-integral values shrinks to one half.
	r := counterexample(t, func(t *T) {
		if f := Float64Range(0, 10).Draw(t, "f"); f != math.Trunc(f) {
			t.Fatalf("%v", f)
		}
	})
	if got := drawn[float64](t, r, 0); got != 0.5 {
		t.Errorf("got %v, want 0.5", got)
	}
	// The extremes are reachable.
	Check(t, func(t *T) {
		f := Float64().Draw(t, "f")
		t.Classify(math.Abs(f) > 1e100, "huge")
		t.Classify(f != 0 && math.Abs(f) < 1e-100, "tiny")
		t.Classify(f == math.Trunc(f), "integral")
		if math.IsNaN(f) || math.IsInf(f, 0) {
			t.Fatalf("%v is not finite", f)
		}
	}, NoSave(), Checks(300))
}

func TestParityTwoLargeElements(t *testing.T) {
	r := counterexample(t, func(t *T) {
		xs := SliceOf(Int()).Draw(t, "xs")
		n := 0
		for _, x := range xs {
			if x > 1000000 {
				n++
			}
		}
		if n > 1 {
			t.Fatalf("%v", xs)
		}
	})
	if got := drawn[[]int](t, r, 0); !slices.Equal(got, []int{1000001, 1000001}) {
		t.Errorf("got %v, want [1000001 1000001]", got)
	}
}

func TestParityIndexedElement(t *testing.T) {
	r := counterexample(t, func(t *T) {
		xs := SliceOfN(Int(), 1, -1).Draw(t, "xs")
		i := IntRange(0, len(xs)-1).Draw(t, "i")
		if xs[i] >= 100 {
			t.Fatalf("%v[%d]", xs, i)
		}
	})
	if xs, i := drawn[[]int](t, r, 0), drawn[int](t, r, 1); !slices.Equal(xs, []int{100}) || i != 0 {
		t.Errorf("got %v[%d], want [100][0]", xs, i)
	}
}

func TestParityLengthModulo(t *testing.T) {
	r := counterexample(t, func(t *T) {
		xs := SliceOfN(Int(), 4, -1).Draw(t, "xs")
		if len(xs)%3 == 1 && xs[len(xs)-1] >= 100 {
			t.Fatalf("%v", xs)
		}
	})
	if got := drawn[[]int](t, r, 0); !slices.Equal(got, []int{0, 0, 0, 100}) {
		t.Errorf("got %v, want [0 0 0 100]", got)
	}
}

func TestParityDistinctSorted(t *testing.T) {
	r := counterexample(t, func(t *T) {
		xs := SliceOfN(IntMin(1), 5, -1).Draw(t, "xs")
		slices.Sort(xs)
		for i := 1; i < len(xs); i++ {
			if xs[i-1] == xs[i] {
				return
			}
		}
		t.Fatalf("%v has no duplicates", xs)
	})
	got := drawn[[]int](t, r, 0)
	slices.Sort(got)
	if !slices.Equal(got, []int{1, 2, 3, 4, 5}) {
		t.Errorf("got %v, want [1 2 3 4 5]", got)
	}
}

func TestParityStringLengths(t *testing.T) {
	r := counterexample(t, func(t *T) {
		s1 := String().Draw(t, "s1")
		s2 := String().Draw(t, "s2")
		if len(s1) > len(s2) {
			t.Fatalf("%q %q", s1, s2)
		}
	})
	if s1, s2 := drawn[string](t, r, 0), drawn[string](t, r, 1); s1 != "0" || s2 != "" {
		t.Errorf("got %q and %q, want \"0\" and \"\"", s1, s2)
	}
	r = counterexample(t, func(t *T) {
		s1 := StringOf(SampledFrom([]rune("XYZ"))).Draw(t, "s1")
		s2 := StringOf(SampledFrom([]rune("UVW"))).Draw(t, "s2")
		if len(s1) > len(s2) {
			t.Fatalf("%q %q", s1, s2)
		}
	})
	if s1, s2 := drawn[string](t, r, 0), drawn[string](t, r, 1); s1 != "X" || s2 != "" {
		t.Errorf("got %q and %q, want \"X\" and \"\"", s1, s2)
	}
}
