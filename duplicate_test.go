// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestHealthDuplicateOptions(t *testing.T) {
	prop := func(pt *T) { Bool().Draw(pt, "b") }
	for _, tc := range []struct {
		name       string
		opts       []Option
		warn, fail bool
	}{
		{"off", nil, false, false},
		{"default", []Option{CheckHealth()}, false, false},
		{"warn", []Option{WarnHealth(HealthDuplicate)}, true, false},
		{"fail", []Option{CheckHealth(HealthDuplicate)}, true, true},
		{"ignore", []Option{CheckHealth(HealthDuplicate), IgnoreHealth(HealthDuplicate)}, false, false},
		{"ignore-first", []Option{IgnoreHealth(HealthDuplicate), WarnHealth(HealthDuplicate), CheckHealth(HealthDuplicate)}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := healthRun(prop, append([]Option{Sizes(1, 1), Checks(20)}, tc.opts...)...)
			out := healthOutput(tb)
			if strings.Contains(out, "health: HealthDuplicate:") != tc.warn || (len(tb.errors) > 0) != tc.fail {
				t.Fatal(out)
			}
			if tc.warn {
				for _, want := range []string{"2 of 20", "Bool or SampledFrom", "low maximum size", "more Checks"} {
					if !strings.Contains(out, want) {
						t.Fatalf("missing %q: %s", want, out)
					}
				}
			}
		})
	}
}

func TestDistinctInputThresholds(t *testing.T) {
	for _, tc := range []struct {
		cases, distinct int
		want            bool
	}{
		{19, 1, false}, {20, 9, true}, {20, 10, false}, {21, 10, true}, {21, 11, false},
	} {
		d := distinctInputs{}
		for i := range tc.cases {
			d.add(&T{size: 1, choices: []uint64{uint64(i % tc.distinct)}})
		}
		if (d.note() != nil) != tc.want {
			t.Fatalf("%+v: %v", tc, d.note())
		}
	}
}

func TestDistinctInputKeyAndBound(t *testing.T) {
	d := distinctInputs{}
	for _, c := range []*T{
		{size: 1, choices: []uint64{1, 2}},
		{size: 2, choices: []uint64{1, 2}}, // the size is not part of the key
		{size: 1, choices: []uint64{2, 1}},
		{size: 1, choices: []uint64{1}},
		{size: 1, choices: []uint64{1, 2}, prefix: []uint64{99, 99, 99}},
	} {
		d.add(c)
	}
	if len(d.seen) != 3 || d.cases != 5 {
		t.Fatal(d)
	}
	d = distinctInputs{}
	for i := range duplicateMaxDistinct {
		d.add(&T{choices: []uint64{uint64(i)}})
	}
	if len(d.seen) != duplicateMaxDistinct || d.saturated {
		t.Fatal("set filled too early")
	}
	for range duplicateMaxDistinct + 1 {
		d.add(&T{choices: []uint64{0}})
	}
	if d.note() == nil {
		t.Fatal("duplicates at the cap should still be diagnosed")
	}
	d.add(&T{choices: []uint64{duplicateMaxDistinct}})
	for range 10000 {
		d.add(&T{})
	}
	if !d.saturated || d.seen != nil || d.note() != nil {
		t.Fatal("saturated set must release memory and suppress the diagnosis")
	}
}

func TestHealthDuplicateExcludesOtherPhases(t *testing.T) {
	for _, phase := range []string{"replay", "discards", "targeted", "shrink"} {
		t.Run(phase, func(t *testing.T) {
			opts := []Option{Checks(19), Sizes(1, 1), WarnHealth(HealthDuplicate)}
			calls := 0
			prop := Property(func(pt *T) { Bool().Draw(pt, "b") })
			switch phase {
			case "replay":
				_, path := savedStaleFixture(t)
				opts = append(opts, ReplayFile(path))
			case "discards":
				prop = func(pt *T) {
					calls++
					Bool().Draw(pt, "b")
					pt.Assume(calls > 5)
				}
			case "targeted":
				opts = append(opts, SearchSteps(100))
				prop = func(pt *T) {
					Bool().Draw(pt, "b")
					pt.Maximize("constant", 0)
				}
			case "shrink":
				opts = append(opts, MaxFailures(2))
				prop = func(pt *T) {
					IntRange(100000, 200000).Draw(pt, "x")
					pt.Fail()
				}
			}
			tb := healthRun(prop, opts...)
			if strings.Contains(healthOutput(tb), "health: HealthDuplicate:") {
				t.Fatal(healthOutput(tb))
			}
		})
	}
	// Failing random cases count, even when they share one failure identity.
	tb := healthRun(func(pt *T) { Bool().Draw(pt, "b"); pt.Fail() }, Checks(20), Sizes(1, 1), MaxFailures(2), WarnHealth(HealthDuplicate))
	if len(tb.errors) != 1 || strings.Count(healthOutput(tb), "HealthDuplicate:") != 1 {
		t.Fatal(healthOutput(tb))
	}
}

func TestHealthDuplicatePreservesChoicesAndOtherDiagnostics(t *testing.T) {
	var baseline [][]uint64
	for i, opts := range [][]Option{nil, {WarnHealth(HealthDuplicate)}, {CheckHealth(HealthDuplicate)}, {IgnoreHealth(HealthDuplicate)}} {
		var got [][]uint64
		tb := healthRun(func(pt *T) {
			SliceOf(Int()).Draw(pt, "xs")
			got = append(got, slices.Clone(pt.choices))
		}, append([]Option{Checks(30)}, opts...)...)
		if i == 0 {
			baseline = got
		} else if !reflect.DeepEqual(got, baseline) {
			t.Fatal("duplicate tracking changed the random stream")
		}
		if out := healthOutput(tb); out != "" {
			t.Fatal(out)
		}
	}
	// The size is not part of the key: a run whose sizes grow still finds
	// that a Bool property has two inputs, at the price of counting a value
	// that depends on the size alone as a duplicate.
	tb := healthRun(func(pt *T) { Bool().Draw(pt, "b") }, Checks(40), WarnHealth(HealthDuplicate))
	if !strings.Contains(healthOutput(tb), "health: HealthDuplicate:") {
		t.Fatal(healthOutput(tb))
	}
	tb = healthRun(func(pt *T) { Just(pt.Size()).Draw(pt, "size") }, Checks(20), WarnHealth(HealthDuplicate))
	if !strings.Contains(healthOutput(tb), "health: HealthDuplicate:") {
		t.Fatal(healthOutput(tb))
	}
	// The entire choice stream is a proxy, not a hash of returned values.
	tb = healthRun(func(pt *T) {
		Map(Uint64(), func(uint64) bool { return true }).Draw(pt, "constant")
	}, Checks(20), Sizes(42, 42), WarnHealth(HealthDuplicate))
	if strings.Contains(healthOutput(tb), "health: HealthDuplicate:") {
		t.Fatal(healthOutput(tb))
	}
}

func TestHealthDuplicateFlags(t *testing.T) {
	for _, name := range []string{"duplicate", "HealthDuplicate", "DUPLICATE"} {
		got, err := parseHealthChecks(name, false)
		if err != nil || !slices.Equal(got, []HealthCheck{HealthDuplicate}) || HealthDuplicate.String() != "HealthDuplicate" {
			t.Fatal(got, err)
		}
	}
	warn, check, ignore := *flagWarnHealth, *flagCheckHealth, *flagIgnoreHealth
	defer func() { *flagWarnHealth, *flagCheckHealth, *flagIgnoreHealth = warn, check, ignore }()
	for _, mode := range []string{"warn", "check", "ignore"} {
		*flagWarnHealth, *flagCheckHealth, *flagIgnoreHealth = "duplicate", "", ""
		if mode != "warn" {
			*flagCheckHealth = "duplicate"
		}
		if mode == "ignore" {
			*flagIgnoreHealth = "duplicate"
		}
		tb := healthRun(func(pt *T) { Just(0).Draw(pt, "x") }, Checks(20), Sizes(1, 1))
		out := healthOutput(tb)
		if strings.Contains(out, "HealthDuplicate:") != (mode != "ignore") || (len(tb.errors) > 0) != (mode == "check") {
			t.Fatal(fmt.Sprintf("%s: %s", mode, out))
		}
	}
}
