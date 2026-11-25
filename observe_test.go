// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func readObservations(t *testing.T, dir, name string) ([]observationCase, []observationInfo) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, sanitizeName(name)+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []observationCase
	var infos []observationInfo
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &fields); err != nil {
			t.Fatal(err)
		}
		switch string(fields["type"]) {
		case `"test_case"`:
			var c observationCase
			if err := json.Unmarshal([]byte(line), &c); err != nil {
				t.Fatal(err)
			}
			for _, key := range []string{"run_start", "property", "status", "status_reason", "representation", "arguments", "how_generated", "features", "timing", "coverage", "metadata"} {
				if _, ok := fields[key]; !ok {
					t.Fatalf("missing %s: %s", key, line)
				}
			}
			if c.Status != "passed" && c.Status != "failed" && c.Status != "gave_up" {
				t.Fatal(c.Status)
			}
			if c.Coverage != nil || c.RunStart <= 0 || c.Arguments == nil || c.Features == nil || c.Timing == nil {
				t.Fatal(c)
			}
			for _, v := range c.Timing {
				if v < 0 {
					t.Fatal(c.Timing)
				}
			}
			cases = append(cases, c)
		case `"info"`:
			var info observationInfo
			if err := json.Unmarshal([]byte(line), &info); err != nil {
				t.Fatal(err)
			}
			if info.Title == "" || info.Content == "" || info.Property == "" || info.RunStart <= 0 {
				t.Fatal(info)
			}
			infos = append(infos, info)
		default:
			t.Fatal("unknown record", string(line))
		}
	}
	return cases, infos
}
func TestObservationsTimingFeaturesAndTarget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		dir := t.TempDir()
		reportPath := filepath.Join(t.TempDir(), "run.jsonl")
		g := Custom("timed", func(pt *T) string { time.Sleep(2 * time.Second); return "value" })
		tb := &fakeTB{name: t.Name()}
		Check(tb, func(pt *T) {
			g.Draw(pt, "arg")
			Just(2).Draw(pt, "arg")
			Just(3).Draw(pt, "arg#2")
			pt.Classify(false, "tag")
			pt.Classify(true, "tag")
			pt.Classify(true, "tag")
			pt.Classify(false, "tag")
			pt.Classify(false, "absent")
			pt.Collect("tag")
			pt.Classify(false, "7")
			pt.Collect(7)
			pt.Aggregate("sample", "sample")
			pt.Measure("measure", 1)
			pt.Measure("measure", 2)
			pt.Measure("infinite", math.Inf(1))
			pt.Cover(false, 10, "covered")
			pt.Cover(true, 10, "covered")
			pt.Maximize("utility", 10)
			time.Sleep(3 * time.Second)
		}, Observations(dir), ReportJSON(reportPath), Seed(1), Checks(5), SearchSteps(2), NoSave(), NoReplay(), IgnoreHealth(HealthSlow))
		if len(tb.errors) != 0 {
			t.Fatal(tb.errors)
		}
		cases, infos := readObservations(t, dir, tb.Name())
		if len(cases) != 2 || len(infos) != 1 {
			t.Fatal(len(cases), len(infos))
		}
		for i, c := range cases {
			if c.Timing["generate:input"] != 2 || c.Timing["execute:test"] != 3 {
				t.Fatal(c.Timing)
			}
			if c.Features["label:tag"] != "true" || c.Features["label:absent"] != "false" || c.Features["label:7"] != "false" || c.Features["collect"] != "tag" || c.Features["collect#2"] != "7" {
				t.Fatal(c.Features)
			}
			if _, ok := c.Features["label:never"]; ok {
				t.Fatalf("unexpected unclassified label: %v", c.Features)
			}
			if c.Features["aggregate:sample"] != float64(2) || c.Features["measure:measure"] != float64(2) || c.Features["measure:infinite"] != "+Inf" || c.Features["cover:covered"] != float64(1) || c.Features["target:utility"] != float64(10) {
				t.Fatal(c.Features)
			}
			if c.Arguments["arg"] != `"value"` || c.Arguments["arg#2"] != "3" || c.Arguments["arg#3"] != "2" {
				t.Fatal(c.Arguments)
			}
			if c.RunStart != infos[0].RunStart || c.Metadata.Evaluation != i+1 || c.Metadata.Seed != "0x1" {
				t.Fatal(c)
			}
		}
		if cases[0].HowGenerated != "generate" || cases[1].HowGenerated != "target" {
			t.Fatal(cases)
		}
		stats := readReports(t, reportPath)[0].Statistics
		if stats.Cases != 2 || !maps.Equal(stats.Labels, map[string]int{"tag": 6, "7": 2}) {
			t.Fatal(stats)
		}
		if logs := strings.Join(tb.logs, "\n"); !strings.Contains(logs, "300.0% tag") || !strings.Contains(logs, "100.0% 7") || strings.Contains(logs, "absent") {
			t.Fatal(logs)
		}
	})
}
func TestObservationsClassifyPerCase(t *testing.T) {
	dir := t.TempDir()
	tb := &fakeTB{name: t.Name()}
	conditions := [][]bool{{false, true, false}, {false, false}, nil}
	calls := 0
	Check(tb, func(pt *T) {
		for _, cond := range conditions[calls] {
			pt.Classify(cond, "tag")
		}
		calls++
	}, Observations(dir), Seed(1), Checks(len(conditions)), NoSave(), NoReplay())
	if len(tb.errors) != 0 {
		t.Fatal(tb.errors)
	}
	cases, _ := readObservations(t, dir, tb.Name())
	want := []map[string]any{
		{"label:tag": "true"},
		{"label:tag": "false"},
		{},
	}
	if len(cases) != len(want) {
		t.Fatal(len(cases))
	}
	for i, c := range cases {
		if !maps.Equal(c.Features, want[i]) {
			t.Fatalf("case %d features: got %v, want %v", i, c.Features, want[i])
		}
	}
}

func TestObservationsCollectCallOrder(t *testing.T) {
	dir := t.TempDir()
	tb := &fakeTB{name: t.Name()}
	values := [][]any{{"tag", 7, ""}, {7}, nil}
	calls := 0
	Check(tb, func(pt *T) {
		for _, value := range values[calls] {
			pt.Collect(value)
		}
		calls++
	}, Observations(dir), Seed(1), Checks(len(values)), NoSave(), NoReplay())
	if len(tb.errors) != 0 {
		t.Fatal(tb.errors)
	}
	cases, _ := readObservations(t, dir, tb.Name())
	want := []map[string]any{
		{"collect": "tag", "collect#2": "7", "collect#3": ""},
		{"collect": "7"},
		{},
	}
	if len(cases) != len(want) {
		t.Fatal(len(cases))
	}
	for i, c := range cases {
		if !maps.Equal(c.Features, want[i]) {
			t.Fatalf("case %d features: got %v, want %v", i, c.Features, want[i])
		}
	}
}
func TestObservationsFailuresShrinksAndReplay(t *testing.T) {
	dir := t.TempDir()
	saved := t.TempDir()
	reportPath := filepath.Join(t.TempDir(), "run.jsonl")
	tb := &fakeTB{name: t.Name()}
	calls := 0
	prop := func(pt *T) {
		calls++
		ShrinkTo(IntRange(10, 100), Just(0)).Draw(pt, "x")
		pt.Classify(false, "absent")
		pt.Error("broken")
	}
	Check(tb, prop, Observations(dir), ReportJSON(reportPath), Seed(1), TestdataDir(saved))
	cases, infos := readObservations(t, dir, tb.Name())
	counts := map[string]int{}
	var final observationCase
	for _, c := range cases {
		counts[c.HowGenerated]++
		if c.Features["label:absent"] != "false" {
			t.Fatalf("%s features: %v", c.HowGenerated, c.Features)
		}
		if c.Metadata.Final {
			final = c
		}
	}
	if counts["generate"] != 1 || counts["stability_replay"] != 1 || counts["shrink"] == 0 || counts["minimal"] != 1 || len(cases) != calls+1 || len(infos) != 1 {
		t.Fatal(counts, calls, len(cases))
	}
	if final.Status != "failed" || final.Arguments["x"] != "0" || final.Metadata.Evaluation == 0 {
		t.Fatal(final)
	}
	r := readReports(t, reportPath)[0]
	if r.Status != "failed" || r.Failures[0].Draws[0].Value != final.Arguments["x"] {
		t.Fatal(r)
	}
	Check(tb, prop, Observations(dir), Seed(1), TestdataDir(saved))
	cases, infos = readObservations(t, dir, tb.Name())
	if len(infos) != 2 || cases[len(cases)-2].HowGenerated != "replay" || cases[len(cases)-1].HowGenerated != "stability_replay" {
		t.Fatal(cases)
	}
	for _, c := range cases[len(cases)-2:] {
		if c.Features["label:absent"] != "false" {
			t.Fatalf("%s features: %v", c.HowGenerated, c.Features)
		}
	}
}
func TestObservationsExistentialAndDiscard(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	tb := &fakeTB{name: t.Name()}
	CheckExists(tb, func(pt *T) {
		calls++
		if calls == 1 {
			pt.Skip("skip this")
		}
		if calls == 2 {
			pt.Error("reject this")
		}
	}, Observations(dir), Seed(1), Checks(5), ShrinkTime(0))
	cases, infos := readObservations(t, dir, tb.Name())
	if calls != 3 || len(cases) != 4 || len(infos) != 1 || len(tb.errors) != 0 {
		t.Fatal(calls, cases, tb.errors)
	}
	if cases[0].Status != "gave_up" || cases[0].StatusReason != "skip this" || cases[1].Status != "failed" || cases[2].Status != "passed" || !cases[3].Metadata.Final {
		t.Fatal(cases)
	}
}
func TestObservationsConcurrentLargeLines(t *testing.T) {
	dir := t.TempDir()
	// Both names sanitize to the same path, testing colliding parallel appends.
	names := []string{"collision/a?", "collision/a!"}
	t.Run("parallel", func(t *testing.T) {
		for i := range 12 {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				t.Parallel()
				tb := &fakeTB{name: names[i%2]}
				Check(tb, func(pt *T) { Just(strings.Repeat("x", 70000)).Draw(pt, "large") }, Observations(dir), ReportLimit(0), Seed(1), Checks(3), NoSave(), NoReplay())
				if len(tb.errors) != 0 {
					t.Fatal(tb.errors)
				}
			})
		}
	})
	cases, infos := readObservations(t, dir, names[0])
	if len(cases) != 36 || len(infos) != 12 {
		t.Fatal(len(cases), len(infos))
	}
	for _, c := range cases {
		if len(c.Arguments["large"]) != 70002 {
			t.Fatal(len(c.Arguments["large"]))
		}
	}
}
func TestObservationsErrorsAndOptions(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("block"), 0o600); err != nil {
		t.Fatal(err)
	}
	tb := &fakeTB{name: t.Name()}
	calls := 0
	Check(tb, func(pt *T) { calls++ }, Observations(file), Seed(1), Checks(3), NoSave(), NoReplay())
	if calls != 3 || len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "cannot write observations") {
		t.Fatal(calls, tb)
	}
	Check(tb, func(pt *T) { t.Fatal("invalid config ran") }, Observations(dir), Checks(0))
	cases, infos := readObservations(t, dir, tb.Name())
	if len(cases) != 0 || len(infos) != 1 || !strings.HasPrefix(infos[0].Content, "error:") {
		t.Fatal(cases, infos)
	}
	old := *flagObserve
	t.Cleanup(func() { *flagObserve = old })
	*flagObserve = dir
	cfg, err := newConfig([]Option{Observations("")})
	if err != nil || cfg.observe != "" {
		t.Fatal(cfg, err)
	}
}
func TestObservationsPreserveStreamAndDisabledAllocations(t *testing.T) {
	var draws [2][]int
	var logs [2][]string
	for i := range draws {
		tb := &fakeTB{name: "stream"}
		opts := []Option{Seed(1), Checks(20), NoSave(), NoReplay(), Verbose()}
		if i == 1 {
			opts = append(opts, Observations(t.TempDir()))
		}
		Check(tb, func(pt *T) {
			x := Int().Draw(pt, "x")
			draws[i] = append(draws[i], x)
			pt.Collect(x)
			pt.Classify(x%2 == 0, "even")
		}, opts...)
		logs[i] = tb.logs
	}
	if !slices.Equal(draws[0], draws[1]) || !slices.Equal(logs[0], logs[1]) {
		t.Fatal("observations changed random choices or text")
	}
	// Under the race detector, sync.Pool drops a quarter of the values put
	// back, so fmt's buffers make allocation counts vary between runs.
	if raceEnabled {
		t.Skip("stream checked; allocation counts are unreliable under the race detector")
	}
	cfg, err := newConfig([]Option{Seed(1), NoSave(), NoReplay()})
	if err != nil {
		t.Fatal(err)
	}
	r := &runner{tb: &fakeTB{name: "allocations"}, cfg: cfg, prop: func(pt *T) {
		pt.Collect(Int().Draw(pt, "x"))
		pt.Classify(true, "tag")
	}}
	baseline := testing.AllocsPerRun(100, func() {
		runObservationBaseline(r)
	})
	actual := testing.AllocsPerRun(100, func() { r.run(nil, 1, nil) })
	if actual != baseline {
		t.Fatalf("disabled observer adds allocations: baseline %g actual %g", baseline, actual)
	}
}

// Return the case just as runner.run does, so escape analysis treats both
// sides alike (in particular under the race detector).
//
//go:noinline
func runObservationBaseline(r *runner) *T {
	pt := newT(r.tb.Name(), nil, 1, r.cfg.maxSize, nil)
	pt.execute(r.prop)
	return pt
}

// /dev/full exercises errors after opening the sink, including a final flush.
func TestObservationsFlushError(t *testing.T) {
	f, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Skip("requires /dev/full")
	}
	tb := &fakeTB{name: t.Name()}
	cfg, err := newConfig([]Option{Seed(1), Observations(t.TempDir())})
	if err != nil {
		f.Close()
		t.Fatal(err)
	}
	r := &runner{tb: tb, cfg: cfg, prop: func(pt *T) { Just(1).Draw(pt, "x") }}
	r.startOutput()
	if r.observer.err != nil {
		f.Close()
		t.Fatal(r.observer.err)
	}
	if err := r.observer.file.Close(); err != nil {
		f.Close()
		t.Fatal(err)
	}
	r.observer.file = f
	r.observer.buffer.Reset(lockedAppend{f})
	r.run(nil, 1, nil)
	r.finishOutput()
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "cannot write observations") {
		t.Fatal(tb.errors)
	}
}
