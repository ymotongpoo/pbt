// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"slices"
	"strings"
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: shrinkto
// year shrinks toward the 1970s, then the 20th century, rather than toward
// the year 0: the alternatives are tried in order while shrinking and never
// used while generating.
var year = pbt.ShrinkTo(pbt.IntRange(0, 9999), pbt.IntRange(1970, 1979), pbt.IntRange(1900, 1999))

// offset shrinks toward 0 from either side instead of toward -720.
var offset = pbt.IntRangeToward(-720, 840, 0)

// snippet: end

func TestShrinkTo(t *testing.T) {
	report := failureReport(t, "TestYear", func(t *pbt.T) {
		y := year.Draw(t, "year")
		if y >= 1990 {
			t.Fatalf("year %d", y)
		}
	}, pbt.Seed(1))
	mustContain(t, report, "year (ShrinkTo(IntRange(0, 9999))) = 1990")
	report = failureReport(t, "TestOffset", func(t *pbt.T) {
		o := offset.Draw(t, "offset")
		if o < -30 || o > 30 {
			t.Fatalf("offset %d", o)
		}
	}, pbt.Seed(1))
	if !strings.Contains(report, "= 31") && !strings.Contains(report, "= -31") {
		t.Errorf("expected 31 or -31:\n%s", report)
	}
}

// snippet: letshrink
// A concatenation that can shrink to one of its parts, as PropEr's
// ?LETSHRINK does; without it, the shrinker could only shorten each part.
var part = pbt.SliceOf(pbt.IntRange(1, 9))

var concat = pbt.LetShrink([]pbt.Gen[[]int]{part, part, part}, func(parts [][]int) []int {
	return slices.Concat(parts...)
})

// snippet: end

func TestLetShrink(t *testing.T) {
	report := failureReport(t, "TestConcat", func(t *pbt.T) {
		xs := concat.Draw(t, "xs")
		if len(xs) >= 2 {
			t.Fatalf("len %d", len(xs))
		}
	}, pbt.Seed(1))
	mustContain(t, report, "= []int{1, 1}")
}

// snippet: noshrink
// A token the system under test treats as opaque; shrinking it would only
// slow the search down.
var token = pbt.NoShrink(pbt.StringOfN(pbt.SampledFrom([]rune("0123456789abcdef")), 32, 32))

// snippet: end

func TestNoShrink(t *testing.T) {
	pbt.Check(t, func(t *pbt.T) {
		if tok := token.Draw(t, "token"); len(tok) != 32 {
			t.Fatalf("%q", tok)
		}
	})
}

// snippet: sampleshrink
func TestSampleShrink(t *testing.T) {
	var path []int
	for xs := range pbt.SampleShrink(pbt.SliceOfN(pbt.IntRange(1, 9), 1, 5), 1) {
		path = append(path, len(xs))
	}
	if path[len(path)-1] != 1 {
		t.Fatalf("the shrink path ended at length %d", path[len(path)-1])
	}
}

// snippet: end

// snippet: identity
// Two bugs: a panic for large inputs and a wrong answer for the rest. The
// shrinker keeps the failure it started from, so a panic found at a large
// input is reported as a panic, at the smallest input that still panics.
func lookupDiscount(qty int) int {
	if qty > 1000 {
		panic("quantity out of range")
	}
	if qty > 100 {
		return 5
	}
	return 0
}

var discountProperty = func(t *pbt.T) {
	qty := pbt.IntRange(0, 5000).Draw(t, "qty")
	if d := lookupDiscount(qty); qty > 100 && d != 10 {
		t.Fatalf("discount for %d units = %d, want 10", qty, d)
	}
}

// snippet: end

func TestIdentity(t *testing.T) {
	report := failureReport(t, "TestDiscount", discountProperty, pbt.Seed(3), pbt.Sizes(42, 42))
	if strings.Contains(report, "panic: quantity out of range") {
		mustContain(t, report, "qty (IntRange(0, 5000)) = 1001")
	} else {
		mustContain(t, report, "qty (IntRange(0, 5000)) = 101")
	}
}
