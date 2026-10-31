// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: maximize
// A random walk rarely ends far to the right of the origin. Maximize turns
// the run into a search for walks that do, and the property fails once one
// gets past 50; without the search, 100 random cases would almost never
// fail.
var walkProperty = func(t *pbt.T) {
	steps := pbt.SliceOf(pbt.SampledFrom([]int{-1, 1})).Draw(t, "steps")
	pos := 0
	for _, s := range steps {
		pos += s
	}
	t.Maximize("position", float64(pos))
	if pos > 50 {
		t.Fatalf("the walk reached %d", pos)
	}
}

// snippet: end

func TestMaximizeFindsAFarWalk(t *testing.T) {
	report := failureReport(t, "TestWalk", walkProperty, pbt.Seed(1), pbt.SearchSteps(3000))
	mustContain(t, report, "utility position = 51", "the walk reached 51")
}

// snippet: exists
// CheckExists succeeds once a case passes: here, a Pythagorean triple. The
// witness is shrunk and logged.
func TestATripleExists(t *testing.T) {
	pbt.CheckExists(t, func(t *pbt.T) {
		a := pbt.IntRange(1, 50).Draw(t, "a")
		b := pbt.IntRange(1, 50).Draw(t, "b")
		c := pbt.IntRange(1, 70).Draw(t, "c")
		if a*a+b*b != c*c {
			t.Fail()
		}
	}, pbt.Checks(5000))
}

// snippet: end
