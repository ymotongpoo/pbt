// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"testing"
)

func TestObservationsDisabledLabelAllocations(t *testing.T) {
	if raceEnabled {
		t.Skip("fmt's pool makes allocation counts unreliable under the race detector")
	}
	cfg, err := newConfig([]Option{Seed(1), Observations(""), NoSave(), NoReplay()})
	if err != nil {
		t.Fatal(err)
	}
	// Precompute and box the values outside the measurement. Both properties
	// format the same values, so only the label storage differs.
	values := []any{fmt.Sprint(1), fmt.Sprint(2)}
	for _, counts := range []struct{ collect, held, missed int }{
		{2, 2, 2},
		{0, 0, 2},
		{0, 2, 2},
		{2, 0, 2},
		{2, 2, 0},
		{5, 3, 5},
		{16, 16, 16},
	} {
		t.Run(fmt.Sprintf("collect=%d/true=%d/false=%d", counts.collect, counts.held, counts.missed), func(t *testing.T) {
			r := &runner{tb: &fakeTB{name: "allocations"}, cfg: cfg}
			r.prop = func(pt *T) {
				for i := 0; i < counts.collect; i++ {
					pt.Classify(true, fmt.Sprint(values[i%len(values)]))
				}
				for i := 0; i < counts.held; i++ {
					pt.Classify(true, "held")
				}
			}
			baseline := testing.AllocsPerRun(1000, func() { r.run(nil, 1, nil) })
			r.prop = func(pt *T) {
				for i := 0; i < counts.collect; i++ {
					pt.Collect(values[i%len(values)])
				}
				for i := 0; i < counts.held; i++ {
					pt.Classify(true, "held")
				}
				for i := 0; i < counts.missed; i++ {
					pt.Classify(false, "missed")
				}
			}
			actual := testing.AllocsPerRun(1000, func() { r.run(nil, 1, nil) })
			t.Logf("allocations/case: true-only=%g mixed=%g", baseline, actual)
			if actual != baseline {
				t.Fatalf("disabled observations changed label allocations: true-only %g mixed %g", baseline, actual)
			}
		})
	}
}
