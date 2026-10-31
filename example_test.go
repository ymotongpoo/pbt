package pbt_test

import (
	"slices"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// biggest returns the largest element of a non-empty slice. It is the
// running example of the first chapters of the PropEr Testing book.
func biggest(xs []int) int {
	m := xs[0]
	for _, x := range xs[1:] {
		if x > m {
			m = x
		}
	}
	return m
}

func TestBiggest(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		sorted := slices.Clone(xs)
		slices.Sort(sorted)
		if got, want := biggest(xs), sorted[len(sorted)-1]; got != want {
			t.Fatalf("biggest(%v) = %d, want %d", xs, got, want)
		}
	})
}

func TestLastAfterAppend(t *testing.T) {
	pbt.Check(t, pbt.ForAll2(pbt.SliceOf(pbt.Int()), pbt.Int(), func(t *pbt.T, xs []int, x int) {
		ys := append(slices.Clone(xs), x)
		if ys[len(ys)-1] != x {
			t.Fatalf("last of %v is not %d", ys, x)
		}
	}))
}
