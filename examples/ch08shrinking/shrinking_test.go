// Package ch08shrinking ports Chapter 8 of the PropEr Testing book: how
// generators shrink, how ShrinkTo re-centers a value, and how LetShrink
// lets a composite shrink to one of its parts.
package ch08shrinking

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
	"go.ymotongpoo.com/pbt/examples/internal/expect"
)

// Section 8.1: a date-time string whose parts shrink toward values a human
// finds unremarkable, the 1970 epoch and whole hours, rather than toward 0.

type date struct{ y, m, d int }
type clock struct{ h, mi, s int }
type zone struct {
	sign string
	h, m int
}
type datetime struct {
	date date
	time clock
	zone zone
}

var (
	year  = pbt.ShrinkTo(pbt.IntRange(0, 9999), pbt.IntRange(1970, 2000), pbt.IntRange(1900, 2100))
	month = pbt.IntRange(1, 12)
	day   = pbt.IntRange(1, 31)

	dateGen = pbt.Filter(pbt.Custom("date", func(t *pbt.T) date {
		return date{year.Draw(t, ""), month.Draw(t, ""), day.Draw(t, "")}
	}), func(d date) bool {
		_, err := time.Parse("2006-1-2", fmt.Sprintf("%d-%d-%d", d.y, d.m, d.d))
		return err == nil
	})

	clockGen = pbt.Custom("time", func(t *pbt.T) clock {
		return clock{pbt.IntRange(0, 24).Draw(t, ""), pbt.IntRange(0, 59).Draw(t, ""), pbt.IntRange(0, 60).Draw(t, "")}
	})

	zoneGen = pbt.Custom("timezone", func(t *pbt.T) zone {
		return zone{
			sign: pbt.SampledFrom([]string{"+", "-"}).Draw(t, ""),
			h:    pbt.ShrinkTo(pbt.IntRange(0, 99), pbt.IntRange(0, 14), pbt.Just(0)).Draw(t, ""),
			m:    pbt.ShrinkTo(pbt.IntRange(0, 99), pbt.Just(0), pbt.Just(15), pbt.Just(30), pbt.Just(45)).Draw(t, ""),
		}
	})

	datetimeGen = pbt.Custom("datetime", func(t *pbt.T) datetime {
		return datetime{dateGen.Draw(t, ""), clockGen.Draw(t, ""), zoneGen.Draw(t, "")}
	})

	strdatetime = pbt.Map(datetimeGen, toStr)
)

func toStr(dt datetime) string {
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d%s%02d:%02d",
		dt.date.y, dt.date.m, dt.date.d, dt.time.h, dt.time.mi, dt.time.s, dt.zone.sign, dt.zone.h, dt.zone.m)
}

// SampleShrink shows the path a value takes, as proper_gen:sampleshrink
// does; it ends at the epoch with a zero offset.
func TestDatetimeShrinksToTheEpoch(t *testing.T) {
	var last string
	for s := range pbt.SampleShrink(strdatetime, 1) {
		last = s
	}
	if last != "1970-01-01T00:00:00+00:00" {
		t.Errorf("shrinking ended at %q", last)
	}
}

// A property that only fails in July shrinks to the first of July 1970.
func TestFailingInJulyShrinksToJuly1970(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		dt := datetimeGen.Draw(t, "dt")
		if dt.date.m == 7 {
			t.Fatalf("%s", toStr(dt))
		}
	}, pbt.Seed(1))
	if !strings.Contains(report, "1970-07-01T00:00:00+00:00") {
		t.Errorf("expected 1970-07-01T00:00:00+00:00:\n%s", report)
	}
}

// Section 8.2: a tree whose structure is fixed by its size only shrinks the
// numbers in its leaves. With LetShrink, a branch can shrink to one of its
// subtrees, and the tree itself gets smaller.
type tree struct {
	left, right *tree
	value       int
}

func (n *tree) String() string {
	if n.left == nil {
		return fmt.Sprintf("{leaf,%d}", n.value)
	}
	return fmt.Sprintf("{branch,%v,%v}", n.left, n.right)
}

func leaves(n *tree) int {
	if n.left == nil {
		return 1
	}
	return leaves(n.left) + leaves(n.right)
}

func treeFixed(n int) pbt.Gen[*tree] {
	if n <= 1 {
		return pbt.Map(pbt.Int(), func(v int) *tree { return &tree{value: v} })
	}
	per := n / 2
	return pbt.Custom("branch", func(t *pbt.T) *tree {
		return &tree{left: treeFixed(per).Draw(t, ""), right: treeFixed(per).Draw(t, "")}
	})
}

func treeShrink(n int) pbt.Gen[*tree] {
	if n <= 1 {
		return pbt.Map(pbt.Int(), func(v int) *tree { return &tree{value: v} })
	}
	per := n / 2
	return pbt.LetShrink([]pbt.Gen[*tree]{treeShrink(per), treeShrink(per)}, func(parts []*tree) *tree {
		return &tree{left: parts[0], right: parts[1]}
	})
}

func TestTreeShrinking(t *testing.T) {
	var last *tree
	for n := range pbt.SampleShrink(treeFixed(4), 1) {
		last = n
	}
	if leaves(last) != 4 {
		t.Errorf("the fixed tree changed shape: %v", last)
	}
	for n := range pbt.SampleShrink(treeShrink(16), 1) {
		last = n
	}
	if leaves(last) != 1 || last.value != 0 {
		t.Errorf("the LetShrink tree did not shrink to {leaf,0}: %v", last)
	}
}

// The generic LetShrink example: a concatenation shrinks to one of its parts.
func TestConcatenationShrinksToAPart(t *testing.T) {
	part := pbt.SliceOf(pbt.Int())
	concat := pbt.LetShrink([]pbt.Gen[[]int]{part, part, part}, func(ps [][]int) []int { return slices.Concat(ps...) })
	report := expect.Failure(t, func(t *pbt.T) {
		if xs := concat.Draw(t, "xs"); len(xs) >= 2 {
			t.Fatalf("len %d", len(xs))
		}
	}, pbt.Seed(1))
	if !strings.Contains(report, "= []int{0, 0}") {
		t.Errorf("expected [0 0]:\n%s", report)
	}
}

// Exercise 3: a meal with four courses must shrink to a single course that
// breaks the "no dairy" property. The chapter's first generator returned
// all four courses as a fixed list, so nothing could be dropped; wrapping
// each course in a list and concatenating lets LetShrink keep one.
var (
	appetizers = []string{"soup", "salad", "cheesesticks"}
	drinks     = []string{"coffee", "tea", "milk", "water", "juice"}
	entrees    = []string{"lasagna", "tofu", "steak"}
	desserts   = []string{"cake", "chocolate", "icecream"}
)

func isDairy(item string) bool {
	switch item {
	case "cheesesticks", "lasagna", "icecream", "milk":
		return true
	}
	return false
}

func course(items []string) pbt.Gen[[]string] {
	return pbt.Map(pbt.SampledFrom(items), func(s string) []string { return []string{s} })
}

var meal = pbt.LetShrink(
	[]pbt.Gen[[]string]{course(appetizers), course(drinks), course(entrees), course(desserts)},
	func(courses [][]string) []string { return slices.Concat(courses...) },
)

func TestMealShrinksToOneDairyItem(t *testing.T) {
	report := expect.Failure(t, func(t *pbt.T) {
		food := meal.Draw(t, "food")
		for _, item := range food {
			if isDairy(item) {
				t.Fatalf("dairy: %v", food)
			}
		}
	}, pbt.Seed(1))
	single := false
	for _, item := range []string{"cheesesticks", "milk", "lasagna", "icecream"} {
		single = single || strings.Contains(report, fmt.Sprintf("= []string{%q}", item))
	}
	if !single {
		t.Errorf("expected a single dairy course:\n%s", report)
	}
}
