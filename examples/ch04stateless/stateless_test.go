package ch04stateless

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

// Section 4.4: biggest against a model, the sorted list. The property needs
// a non-empty list, as the chapter discovers.
func TestBiggest(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		sorted := slices.Clone(xs)
		slices.Sort(sorted)
		if got, want := Biggest(xs), sorted[len(sorted)-1]; got != want {
			t.Fatalf("Biggest(%v) = %d, want %d", xs, got, want)
		}
	})
}

// The chapter's first attempt returned the head. The counterexample shrinks
// to two elements with the larger one last, like the book's [0,1].
func TestBiggestHeadIsCaught(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		xs := pbt.SliceOfN(pbt.Int(), 1, -1).Draw(t, "xs")
		sorted := slices.Clone(xs)
		slices.Sort(sorted)
		if got, want := BiggestHead(xs), sorted[len(sorted)-1]; got != want {
			t.Fatalf("BiggestHead(%v) = %d, want %d", xs, got, want)
		}
	}, pbt.Seed(1))
	if !strings.Contains(report, "xs (SliceOfN(Int(), 1, -1)) = []int{0, 1}") {
		t.Errorf("expected the counterexample [0, 1]:\n%s", report)
	}
}

func isOrdered(xs []int) bool {
	for i := 1; i < len(xs); i++ {
		if xs[i-1] > xs[i] {
			return false
		}
	}
	return true
}

// Section 4.4: sorting as invariants: ordered, same length, same elements.
func TestSort(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		sorted := slices.Clone(xs)
		slices.Sort(sorted)
		if !isOrdered(sorted) {
			t.Fatalf("not ordered: %v", sorted)
		}
		if len(sorted) != len(xs) {
			t.Fatalf("length changed")
		}
		for _, x := range xs {
			if !slices.Contains(sorted, x) {
				t.Fatalf("%d went missing", x)
			}
		}
		for _, x := range sorted {
			if !slices.Contains(xs, x) {
				t.Fatalf("%d appeared", x)
			}
		}
	})
}

// Section 4.4: last, generalized from the unit tests by appending a known
// element.
func TestLast(t *testing.T) {
	pbt.Check(t, pbt.ForAll2(pbt.SliceOf(pbt.Int()), pbt.Int(), func(t *pbt.T, xs []int, x int) {
		ys := append(slices.Clone(xs), x)
		if ys[len(ys)-1] != x {
			t.Fatalf("last of %v is not %d", ys, x)
		}
	}))
}

// Exercise 3: Seq has count+1 entries that increase by one.
func TestSeq(t *testing.T) {
	pbt.Check(t, pbt.ForAll2(pbt.Int(), pbt.IntRange(0, 1000), func(t *pbt.T, start, count int) {
		xs := Seq(start, count)
		if len(xs) != count+1 {
			t.Fatalf("Seq(%d, %d) has %d entries", start, count, len(xs))
		}
		for i := 1; i < len(xs); i++ {
			if xs[i] != xs[i-1]+1 {
				t.Fatalf("Seq(%d, %d) does not increment by one at %d: %v", start, count, i, xs)
			}
		}
	}))
}

// pairs generates pairs whose value records the original position, so that
// stability can be checked.
var pairs = pbt.Map(pbt.SliceOf(pbt.IntRange(0, 20)), func(keys []int) []Pair {
	ps := make([]Pair, len(keys))
	for i, k := range keys {
		ps[i] = Pair{Key: k, Val: strconv.Itoa(i)}
	}
	return ps
})

// Exercise 4: keysort orders the keys and keeps the elements; the property
// also checks stability, which the chapter's solution left out.
func TestKeySort(t *testing.T) {
	pbt.Check(t, pbt.ForAll(pairs, func(t *pbt.T, ps []Pair) {
		sorted := KeySort(ps)
		keys := make([]int, len(sorted))
		for i, p := range sorted {
			keys[i] = p.Key
		}
		if !isOrdered(keys) {
			t.Fatalf("keys not ordered: %v", keys)
		}
		if len(sorted) != len(ps) {
			t.Fatalf("length changed")
		}
		for i := 1; i < len(sorted); i++ {
			a, _ := strconv.Atoi(sorted[i-1].Val)
			b, _ := strconv.Atoi(sorted[i].Val)
			if sorted[i-1].Key == sorted[i].Key && a > b {
				t.Fatalf("not stable: %v became %v", ps, sorted)
			}
		}
	}))
}

// Exercise 5: the union of two sets against a model. The chapter's first
// model, sort(A ++ B), kept duplicates; the model must deduplicate.
func TestSetUnion(t *testing.T) {
	pbt.Check(t, pbt.ForAll2(pbt.SliceOf(pbt.IntRange(0, 10)), pbt.SliceOf(pbt.IntRange(0, 10)), func(t *pbt.T, a, b []int) {
		got := Union(SetFrom(a), SetFrom(b)).Sorted()
		model := slices.Clone(append(slices.Clone(a), b...))
		slices.Sort(model)
		model = slices.Compact(model)
		if !slices.Equal(got, model) {
			t.Fatalf("union = %v, model = %v", got, model)
		}
	}))
}

// Exercise 6: the chapter's property validated only the keys of the merged
// dictionary. This one also checks the values and the conflict rule.
func TestMerge(t *testing.T) {
	dict := pbt.MapOf(pbt.StringN(1, 2), pbt.Int())
	pbt.Check(t, pbt.ForAll2(dict, dict, func(t *pbt.T, a, b map[string]int) {
		got := Merge(a, b)
		if len(got) < max(len(a), len(b)) {
			t.Fatalf("merge lost keys: %v", got)
		}
		for k, v := range got {
			va, inA := a[k]
			vb, inB := b[k]
			switch {
			case !inA && !inB:
				t.Fatalf("key %q comes from nowhere", k)
			case inA && v != va:
				t.Fatalf("key %q = %d, want %d from the first map", k, v, va)
			case !inA && v != vb:
				t.Fatalf("key %q = %d, want %d from the second map", k, v, vb)
			}
		}
	}))
}

// Exercise 7: word counting, compared against an alternative implementation
// over strings that are heavy on spaces.
func TestWordCount(t *testing.T) {
	wordy := pbt.StringOf(pbt.Frequency(
		pbt.Weighted[rune]{Weight: 3, Gen: pbt.SampledFrom([]rune("abcde"))},
		pbt.Weighted[rune]{Weight: 2, Gen: pbt.Just(' ')},
	))
	pbt.Check(t, pbt.ForAll(wordy, func(t *pbt.T, s string) {
		t.Classify(strings.HasPrefix(s, " "), "leading space")
		t.Classify(strings.Contains(s, "  "), "double space")
		if got, want := WordCount(s), AltWordCount(s); got != want {
			t.Fatalf("WordCount(%q) = %d, AltWordCount = %d", s, got, want)
		}
	}))
}
