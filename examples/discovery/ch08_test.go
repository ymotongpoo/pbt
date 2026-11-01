// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// Median: lower median for even length.
func Median(xs []int) int {
	s := slices.Clone(xs)
	slices.Sort(s)
	return s[(len(s)-1)/2]
}

var nonEmpty = pbt.SliceOfN(pbt.IntRange(-100, 100), 1, -1)

func TestMedianShift(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := nonEmpty.Draw(t, "xs")
		c := pbt.IntRange(-50, 50).Draw(t, "c")
		shifted := make([]int, len(xs))
		for i, x := range xs {
			shifted[i] = x + c
		}
		if got, want := Median(shifted), Median(xs)+c; got != want {
			t.Fatalf("Median(shifted) = %d, want %d (base %v, shift %d)", got, want, xs, c)
		}
	})
}

// Claim in draft: doubling the list "may shift" under the lower-median definition.
func TestMedianDoubled(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := nonEmpty.Draw(t, "xs")
		d := append(slices.Clone(xs), xs...)
		t.Classify(len(xs)%2 == 0, "even")
		if got, want := Median(d), Median(xs); got != want {
			t.Fatalf("Median(doubled %v) = %d, want %d", xs, got, want)
		}
	}, pbt.Verbose(), pbt.Checks(2000))
}

func TestMedianNegateCounterexample(t *testing.T) {
	xs := []int{1, 2}
	neg := []int{-1, -2}
	t.Logf("Median(%v)=%d, Median(%v)=%d (negated expectation %d)", xs, Median(xs), neg, Median(neg), -Median(xs))
}

// Does a "return the minimum" implementation satisfy the relations?
func minImpl(xs []int) int { return slices.Min(xs) }

func TestMinSatisfiesMostRelations(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := nonEmpty.Draw(t, "xs")
		c := pbt.IntRange(-50, 50).Draw(t, "c")
		k := pbt.IntRange(1, 5).Draw(t, "k")
		perm := pbt.Permutation(xs).Draw(t, "perm")
		shifted := make([]int, len(xs))
		scaled := make([]int, len(xs))
		for i, x := range xs {
			shifted[i] = x + c
			scaled[i] = x * k
		}
		m := minImpl(xs)
		if minImpl(perm) != m {
			t.Fatalf("perm")
		}
		if minImpl(shifted) != m+c {
			t.Fatalf("shift")
		}
		if minImpl(scaled) != m*k {
			t.Fatalf("scale")
		}
		added := append(slices.Clone(xs), m+pbt.IntRange(0, 100).Draw(t, "extra"))
		if minImpl(added) < m {
			t.Fatalf("add")
		}
		d := append(slices.Clone(xs), xs...)
		if minImpl(d) != m {
			t.Fatalf("double")
		}
	}, pbt.Checks(500))
}
