// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: largest
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

func TestLargest(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		if got, want := largest(xs), slices.Max(xs); got != want {
			t.Fatalf("largest(%v) = %d, want %d", xs, got, want)
		}
	})
}

// snippet: end

// snippet: biggest
// biggest starts from 0, which is wrong for a slice of negative numbers.
func biggest(xs []int) int {
	m := 0
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}

var biggestProperty = func(t *pbt.T) {
	xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
	if got, want := biggest(xs), slices.Max(xs); got != want {
		t.Fatalf("biggest(%v) = %d, want %d", xs, got, want)
	}
}

// snippet: end

func TestBiggestReport(t *testing.T) {
	report := failureReport(t, "TestBiggest", biggestProperty, pbt.Seed(1))
	mustContain(t, report, "xs (SliceOfN(Int(), 1, -1)) = []int{-1}", "biggest([-1]) = 0, want -1", "-pbt.seed=0x1")
}
