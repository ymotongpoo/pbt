// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package discovery

import (
	"testing"

	"go.ymotongpoo.com/pbt"
)

func TestDupDensity(t *testing.T) {
	for _, g := range []struct {
		name string
		gen  pbt.Gen[[]int]
	}{{"Int", pbt.SliceOf(pbt.Int())}, {"IntRange0_3", pbt.SliceOf(pbt.IntRange(0, 3))}} {
		t.Run(g.name, func(t *testing.T) {
			pbt.Check(t, func(pt *pbt.T) {
				xs := g.gen.Draw(pt, "xs")
				dups := len(xs) - len(Dedup(xs))
				pt.Classify(dups > 0, "has duplicate")
				pt.Classify(dups >= 3, "3+ duplicates")
				pt.Measure("duplicates", float64(dups))
				pt.Measure("len", float64(len(xs)))
			}, pbt.Verbose(), pbt.Checks(1000))
		})
	}
}
