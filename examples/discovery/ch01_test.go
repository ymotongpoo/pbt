// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// largest returns the largest element of a non-empty slice.
func largest(xs []int) int {
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func TestLargestExample(t *testing.T) {
	if got := largest([]int{3, 1, 4, 1, 5}); got != 5 {
		t.Fatalf("largest = %d, want 5", got)
	}
}

func TestLargestQ1(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		if got := largest(xs); !slices.Contains(xs, got) {
			t.Fatalf("largest(%v) = %d, which is not an element", xs, got)
		}
	})
}

func TestLargestQ2(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		got := largest(xs)
		for _, x := range xs {
			if got < x {
				t.Fatalf("largest(%v) = %d, but %d is larger", xs, got, x)
			}
		}
	})
}

func TestLargestQ3(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		ys := slices.Clone(xs)
		slices.Reverse(ys)
		if largest(xs) != largest(ys) {
			t.Fatalf("largest(%v) != largest(%v)", xs, ys)
		}
	})
}

func TestLargestQ4(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		if got, want := largest(xs), slices.Max(xs); got != want {
			t.Fatalf("largest(%v) = %d, want %d", xs, got, want)
		}
	})
}

func TestLargestCopy(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		want := xs[0]
		for _, x := range xs[1:] {
			if x > want {
				want = x
			}
		}
		if got := largest(xs); got != want {
			t.Fatalf("largest(%v) = %d, want %d", xs, got, want)
		}
	})
}

func TestLargestSortOracle(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		sorted := slices.Clone(xs)
		slices.Sort(sorted)
		if got, want := largest(xs), sorted[len(sorted)-1]; got != want {
			t.Fatalf("largest(%v) = %d, want %d", xs, got, want)
		}
	})
}
