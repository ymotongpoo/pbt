// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
)

// snippet: options
func TestSortIsIdempotent(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		once := slices.Sorted(slices.Values(xs))
		twice := slices.Sorted(slices.Values(once))
		if !slices.Equal(once, twice) {
			t.Fatalf("sorting twice gave %v, then %v", once, twice)
		}
	}, pbt.Checks(500), pbt.Sizes(1, 60))
}

// MakeCheck turns a property into a test function, for table-driven or
// generated tests.
var TestReverseTwice = pbt.MakeCheck(func(t *pbt.T) {
	xs := pbt.SliceOf(pbt.Rune()).Draw(t, "xs")
	ys := slices.Clone(xs)
	slices.Reverse(ys)
	slices.Reverse(ys)
	if !slices.Equal(xs, ys) {
		t.Fatalf("reversing twice changed %v", xs)
	}
})

// snippet: end

// snippet: assume
func TestDivision(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		a := pbt.Int().Draw(t, "a")
		b := pbt.Int().Draw(t, "b")
		t.Assume(b != 0) // discards the case; too many discards fail the run
		if (a/b)*b+a%b != a {
			t.Fatalf("division identity fails for %d / %d", a, b)
		}
	})
}

// snippet: end

// snippet: setup
func TestFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	withFile := pbt.Setup(func(t *pbt.T) func() {
		path := filepath.Join(dir, "case")
		t.Cleanup(func() { os.Remove(path) }) // Cleanup runs after the case, latest first
		return nil                            // or return a teardown function
	})
	pbt.Check(t, func(t *pbt.T) {
		data := pbt.Bytes().Draw(t, "data")
		path := filepath.Join(dir, "case")
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path)
		if err != nil || !slices.Equal(got, data) {
			t.Fatalf("read back %v (%v), want %v", got, err, data)
		}
	}, withFile)
}

// snippet: end

// snippet: combinators
func TestStringProperties(t *testing.T) {
	// Conjunction runs every sub-property on the same case and names the
	// ones that fail.
	pbt.Check(t, pbt.Conjunction(
		pbt.Named{Name: "upper is idempotent", Prop: func(t *pbt.T) {
			s := pbt.String().Draw(t, "s")
			if strings.ToUpper(strings.ToUpper(s)) != strings.ToUpper(s) {
				t.Fatalf("%q", s)
			}
		}},
		pbt.Named{Name: "fields never contain spaces", Prop: func(t *pbt.T) {
			s := pbt.String().Draw(t, "s")
			for _, f := range strings.Fields(s) {
				if strings.ContainsRune(f, ' ') {
					t.Fatalf("%q", f)
				}
			}
		}},
	))

	// Deadline fails a case that takes too long. Go cannot stop a running
	// function, so the check happens after the property returns.
	pbt.Check(t, pbt.Deadline(time.Second, func(t *pbt.T) {
		s := pbt.StringN(0, 100).Draw(t, "s")
		if strings.Repeat(s, 2) != s+s {
			t.Fatal("repeat")
		}
	}))
}

// snippet: end

// snippet: onfail
func TestOnFail(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
		sum := 0
		t.OnFail(func() { t.Logf("partial sum when it failed: %d", sum) }) // runs only for the reported case
		for _, x := range xs {
			sum += x
		}
		if len(xs) == 0 && sum != 0 {
			t.Fatal("impossible")
		}
	})
}

// snippet: end

// snippet: panics
func mustPositive(n int) int {
	if n <= 0 {
		panic("not positive")
	}
	return n
}

func TestPanics(t *testing.T) {
	// Recover turns a panic into a value for a property to assert on.
	pbt.Check(t, func(t *pbt.T) {
		n := pbt.IntMax(0).Draw(t, "n")
		if p := pbt.Recover(func() { mustPositive(n) }); p == nil {
			t.Fatalf("mustPositive(%d) did not panic", n)
		}
	})
	// NoPanic generates the arguments of a function from its signature and
	// fails when a call panics.
	pbt.Check(t, pbt.NoPanic(strings.Repeat, pbt.WithType(pbt.IntRange(0, 3))))
}

// snippet: end

func TestConjunctionReport(t *testing.T) {
	report := failureReport(t, "TestStringProperties", pbt.Conjunction(
		pbt.Named{Name: "non-empty", Prop: func(t *pbt.T) {
			if s := pbt.String().Draw(t, "s"); s == "" {
				t.Fatal("empty")
			}
		}},
		pbt.Named{Name: "short", Prop: func(t *pbt.T) {
			if s := pbt.String().Draw(t, "s"); len(s) > 100 {
				t.Fatal("long")
			}
		}},
	), pbt.Seed(1))
	mustContain(t, report, "error: non-empty: empty", "conjunction: [non-empty] failed")
}

var errUnknownUser = errors.New("unknown user")

var _ = errUnknownUser
