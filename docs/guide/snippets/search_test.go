// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package snippets

import (
	"testing"

	"go.ymotongpoo.com/pbt"
)

// snippet: slow-input
// allDistinct counts the comparisons that this quadratic scan performs.
func allDistinct(xs []int) (bool, int) {
	comparisons := 0
	for i, x := range xs {
		for _, y := range xs[:i] {
			comparisons++
			if x == y {
				return false, comparisons
			}
		}
	}
	return true, comparisons
}

var scanWorkProperty = func(t *pbt.T) {
	xs := pbt.SliceOfN(pbt.IntRange(0, 9), 0, 10).Draw(t, "xs")
	_, comparisons := allDistinct(xs)
	t.Maximize("comparisons", float64(comparisons))
	if comparisons > 20 {
		t.Fatalf("scan used %d comparisons, limit 20", comparisons)
	}
}

// snippet: end

func TestSearchFindsExpensiveScan(t *testing.T) {
	report := failureReport(t, "TestScanWork", scanWorkProperty, pbt.Seed(1))
	mustContain(t, report, "utility comparisons = 21", "scan used 21 comparisons, limit 20",
		"property failed after 194 passed checks and 213 shrink steps (size 42)",
		"xs (SliceOfN(IntRange(0, 9), 0, 10)) = []int{0, 1, 2, 3, 5, 4, 4}")
}

// snippet: coverage-guided
func TestCoverageGuidedHeader(t *testing.T) {
	if testing.CoverMode() == "" {
		t.Skip("run with -cover -coverpkg=go.ymotongpoo.com/pbt/docs/guide/snippets")
	}
	property := func(t *pbt.T) {
		b := pbt.SliceOfN(pbt.SampledFrom([]byte("PBT!?")), 4, 4).Draw(t, "header")
		if got := recognizesHeader(b); got != (string(b) == "PBT!") {
			t.Fatalf("header %q: accepted=%t", b, got)
		}
	}
	logs := passingLogs(t, "TestHeader", property, pbt.Seed(1),
		pbt.Checks(200), pbt.CoverageGuided(), pbt.Verbose())
	mustContain(t, logs, "[pbt] coverage-guided:", "of 200 passing cases", "seeds kept")
}

// snippet: end
