// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func Union(a, b []int) []int {
	seen := map[int]bool{}
	var out []int
	for _, xs := range [][]int{a, b} {
		for _, x := range xs {
			if !seen[x] {
				seen[x] = true
				out = append(out, x)
			}
		}
	}
	slices.Sort(out)
	return out
}

var small = pbt.SliceOf(pbt.IntRange(0, 10))

func TestUnionCommutative(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := small.Draw(t, "a")
		b := small.Draw(t, "b")
		if got, want := Union(a, b), Union(b, a); !slices.Equal(got, want) {
			t.Fatalf("Union(%v, %v) = %v, but swapped = %v", a, b, got, want)
		}
	})
}

func TestUnionAssociative(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := small.Draw(t, "a")
		b := small.Draw(t, "b")
		c := small.Draw(t, "c")
		if got, want := Union(Union(a, b), c), Union(a, Union(b, c)); !slices.Equal(got, want) {
			t.Fatalf("assoc: %v vs %v", got, want)
		}
	})
}

func TestUnionIdempotentAndIdentity(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := small.Draw(t, "a")
		if got, want := Union(a, a), Union(a, nil); !slices.Equal(got, want) {
			t.Fatalf("idem: %v vs %v", got, want)
		}
	})
}

// Monotonic: adding an element to a never removes anything from the union.
func TestUnionMonotonic(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := small.Draw(t, "a")
		b := small.Draw(t, "b")
		x := pbt.IntRange(0, 10).Draw(t, "x")
		before := Union(a, b)
		after := Union(append(slices.Clone(a), x), b)
		for _, e := range before {
			if !slices.Contains(after, e) {
				t.Fatalf("lost %d", e)
			}
		}
		// note: after may equal before when x already present -> "subset" wording in draft is wrong
		t.Classify(slices.Equal(before, after), "unchanged (x already present)")
	}, pbt.Verbose())
}
