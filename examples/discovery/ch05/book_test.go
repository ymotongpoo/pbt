// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package ch05

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

func TestUnionCommutative(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := set.Draw(t, "a")
		b := set.Draw(t, "b")

		if got, want := Union(a, b), Union(b, a); !slices.Equal(got, want) {
			t.Fatalf("Union(%v, %v) = %v, but swapped = %v", a, b, got, want)
		}
	})
}

func TestUnionAssociative(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := set.Draw(t, "a")
		b := set.Draw(t, "b")
		c := set.Draw(t, "c")

		left, right := Union(Union(a, b), c), Union(a, Union(b, c))
		if !slices.Equal(left, right) {
			t.Fatalf("(%v, %v, %v): left = %v, right = %v", a, b, c, left, right)
		}
	})
}

func TestUnionIdempotent(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := set.Draw(t, "a")

		if got, want := Union(a, a), Union(a, nil); !slices.Equal(got, want) {
			t.Fatalf("Union(a, a) = %v, Union(a, nil) = %v, a = %v", got, want, a)
		}
	})
}

func TestUnionIdentity(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := set.Draw(t, "a")

		if got, want := Union(a, nil), normalize(a); !slices.Equal(got, want) {
			t.Fatalf("Union(%v, nil) = %v, want %v", a, got, want)
		}
		if got, want := Union(nil, a), normalize(a); !slices.Equal(got, want) {
			t.Fatalf("Union(nil, %v) = %v, want %v", a, got, want)
		}
	})
}

func TestUnionMonotonic(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := set.Draw(t, "a")
		b := set.Draw(t, "b")
		x := pbt.IntRange(0, 10).Draw(t, "x")

		before := Union(a, b)
		after := Union(append(slices.Clone(a), x), b)
		for _, e := range before {
			if !slices.Contains(after, e) {
				t.Fatalf("adding %d to %v lost %d (b = %v)", x, a, e, b)
			}
		}
	})
}

func TestMergeAssociative(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := sortedItems("a").Draw(t, "a")
		b := sortedItems("b").Draw(t, "b")
		c := sortedItems("c").Draw(t, "c")

		left, right := Merge(Merge(a, b), c), Merge(a, Merge(b, c))
		if !slices.Equal(left, right) {
			t.Fatalf("left = %v, right = %v", left, right)
		}
	})
}

func TestMergeStable(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := sortedItems("a").Draw(t, "a")
		b := sortedItems("b").Draw(t, "b")

		got := Merge(a, b)
		for i := 1; i < len(got); i++ {
			p, q := got[i-1], got[i]
			if p.Key == q.Key && p.Tag[0] == 'b' && q.Tag[0] == 'a' {
				t.Fatalf("Merge(%v, %v) = %v: %v precedes %v", a, b, got, p, q)
			}
		}
	})
}
