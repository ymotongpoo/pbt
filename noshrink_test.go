// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"slices"
	"testing"
)

func TestNoShrinkPreservesUnconditionallyFailingValue(t *testing.T) {
	for _, g := range []Gen[int]{
		NoShrink(IntRange(10, 1000)),
		Map(NoShrink(IntRange(10, 1000)), func(x int) int { return x }),
		NoShrink(NoShrink(IntRange(10, 1000))),
	} {
		values := slices.Collect(SampleShrink(g, 1))
		if len(values) != 1 {
			t.Errorf("%s changed values: %v", g, values)
		}
	}
}

func TestNoShrinkKeepsSurvivingCollectionElements(t *testing.T) {
	for _, minLen := range []int{0, 1, 3} {
		g := SliceOfN(NoShrink(IntRange(10, 1000)), minLen, 3)
		cfg, _ := newConfig([]Option{NoSave()})
		r := &runner{tb: t, cfg: cfg, prop: func(t *T) {
			g.Draw(t, "xs")
			t.Fail()
		}}
		// Three distinct, nonminimal elements. A shrink may delete them,
		// but cannot regenerate any retained element from missing choices.
		original := r.run([]uint64{1, 12, 1, 24, 1, 36, 0}, 42, nil)
		before := drawn[[]int](t, original, 0)
		minimal, _ := r.shrink(original)
		after := drawn[[]int](t, minimal, 0)
		if len(after) != minLen {
			t.Fatalf("minLen=%d: shrank %v to %v", minLen, before, after)
		}
		next := 0
		for _, x := range after {
			for next < len(before) && before[next] != x {
				next++
			}
			if next == len(before) {
				t.Fatalf("shrank %v to %v, changing a protected element", before, after)
			}
			next++
		}
	}
}

func TestNoShrinkPreservesSeparateDraws(t *testing.T) {
	cfg, _ := newConfig([]Option{NoSave()})
	r := &runner{tb: t, cfg: cfg, prop: func(t *T) {
		NoShrink(IntRange(10, 1000)).Draw(t, "a")
		NoShrink(IntRange(10, 1000)).Draw(t, "b")
		t.Fail()
	}}
	original := r.run([]uint64{42, 0}, 42, nil)
	minimal, _ := r.shrink(original)
	for i := range 2 {
		if got, want := drawn[int](t, minimal, i), drawn[int](t, original, i); got != want {
			t.Errorf("draw %d changed from %d to %d", i, want, got)
		}
	}
}
