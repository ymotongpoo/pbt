// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func Dedup(xs []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func TestDedupExamples(t *testing.T) {
	cases := []struct{ in, want []int }{
		{[]int{1, 1, 2}, []int{1, 2}},
		{[]int{3, 1, 3}, []int{3, 1}},
		{[]int{}, []int{}},
	}
	for _, c := range cases {
		if got := Dedup(c.in); !slices.Equal(got, c.want) {
			t.Errorf("Dedup(%v) = %v, want %v", c.in, got, c.want)
		}
	}
}

func hasDuplicate(xs []int) bool {
	seen := map[int]bool{}
	for _, x := range xs {
		if seen[x] {
			return true
		}
		seen[x] = true
	}
	return false
}

func TestDedupProps(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.IntRange(0, 3)).Draw(t, "xs")
		t.Classify(hasDuplicate(xs), "has duplicate")
		got := Dedup(xs)

		count := map[int]int{}
		for _, x := range got {
			count[x]++
			if count[x] > 1 {
				t.Fatalf("Dedup(%v) = %v has duplicate %d", xs, got, x)
			}
		}
		for _, x := range xs {
			if count[x] == 0 {
				t.Fatalf("Dedup(%v) = %v lost element %d", xs, got, x)
			}
		}
	}, pbt.Verbose())
}

// Experiment: how often does SliceOf(Int()) contain a duplicate?
func TestDupRateWideInt(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		t.Classify(hasDuplicate(xs), "has duplicate (Int)")
		t.Classify(len(xs) == 0, "empty")
		t.Measure("len", float64(len(xs)))
	}, pbt.Verbose(), pbt.Checks(1000))
}
