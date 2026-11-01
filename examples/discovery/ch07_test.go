// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func IndexSorted(xs []int, v int) int {
	lo, hi := 0, len(xs)
	for lo < hi {
		mid := (lo + hi) / 2
		if xs[mid] < v {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo < len(xs) && xs[lo] == v {
		return lo
	}
	return -1
}

func indexLinear(xs []int, v int) int {
	for i, x := range xs {
		if x == v {
			return i
		}
	}
	return -1
}

var sortedDistinct = pbt.Map(
	pbt.SliceOf(pbt.IntRange(0, 30)),
	func(xs []int) []int {
		slices.Sort(xs)
		return slices.Compact(xs)
	},
)

func TestIndexSortedModel(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := sortedDistinct.Draw(t, "xs")
		v := pbt.IntRange(0, 30).Draw(t, "v")
		t.Classify(slices.Contains(xs, v), "present")
		if got, want := IndexSorted(xs, v), indexLinear(xs, v); got != want {
			t.Fatalf("IndexSorted(%v, %d) = %d, model says %d", xs, v, got, want)
		}
	}, pbt.Verbose())
}

var _ = pbt.SliceOfDistinct(pbt.IntRange(0, 30), func(x int) int { return x })
