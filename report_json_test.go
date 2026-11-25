// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func readReports(t *testing.T, path string) []jsonReport {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var reports []jsonReport
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var report jsonReport
		if err := json.Unmarshal([]byte(line), &report); err != nil {
			t.Fatal(err, string(data))
		}
		if report.Schema != reportSchema {
			t.Fatal(report.Schema)
		}
		reports = append(reports, report)
	}
	return reports
}
func TestReportJSONOutcomes(t *testing.T) {
	for _, mode := range []string{"passed", "failed", "gave_up", "no_witness", "witness", "error", "replay_error", "health", "coverage"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "report.jsonl")
			tb := &fakeTB{name: t.Name()}
			opts := []Option{ReportJSON(path), NoSave(), NoReplay(), Seed(1), Checks(3), ShrinkTime(0)}
			prop := Property(func(pt *T) { Just(12).Draw(pt, "value") })
			want := mode
			switch mode {
			case "failed", "no_witness":
				prop = func(pt *T) { Just(12).Draw(pt, "value"); pt.Error("broken") }
			case "gave_up":
				prop = func(pt *T) { pt.Skip("discard") }
			case "error":
				opts = append(opts, Checks(0))
			case "replay_error":
				opts = append(opts, ReplayFile(filepath.Join(t.TempDir(), "missing")))
				want = "error"
			case "witness":
				want = "passed"
			case "health":
				i := 0
				prop = func(pt *T) {
					i++
					if i <= 25 {
						pt.Skip("discard")
					}
				}
				opts = append(opts, CheckHealth(HealthFilter))
				want = "failed"
			case "coverage":
				prop = func(pt *T) { pt.Cover(false, 100, "required") }
				opts = append(opts, CheckCoverage())
				want = "failed"
			}
			if mode == "witness" || mode == "no_witness" {
				CheckExists(tb, prop, opts...)
			} else {
				Check(tb, prop, opts...)
			}
			reports := readReports(t, path)
			if len(reports) != 1 {
				t.Fatal(reports)
			}
			r := reports[0]
			if r.Status != want || r.Test != tb.Name() || r.Seed != "0x1" {
				t.Fatalf("%+v", r)
			}
			if mode == "passed" && r.Passed != 3 {
				t.Fatal(r.Passed)
			}
			if mode == "no_witness" && (r.Rejected != 3 || r.Passed != 0) {
				t.Fatal(r)
			}
			if mode == "gave_up" && r.Discards != 31 {
				t.Fatal(r.Discards)
			}
			if mode == "health" && (len(r.Health) != 1 || r.Health[0].Kind != "HealthFilter") {
				t.Fatal(r.Health)
			}
			if mode == "witness" && (r.Witness == nil || r.Witness.Draws[0].Value != "12" || r.Passed != 1) {
				t.Fatal(r)
			}
			if mode == "failed" && (len(r.Failures) != 1 || r.Failures[0].Identity == "" || r.Failures[0].Draws[0].Value != "12") {
				t.Fatal(r)
			}
		})
	}
}
func TestReportJSONStatisticsAndBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "report.jsonl")
		Check(&fakeTB{name: t.Name()}, func(pt *T) {
			pt.Classify(true, "class")
			pt.Collect("label")
			pt.Aggregate("a", "a", "b")
			pt.Measure("length", 2)
			pt.Measure("infinity", math.Inf(1))
			pt.Cover(true, 50, "half")
			pt.Maximize("score", 7)
			time.Sleep(time.Second)
		}, ReportJSON(path), Seed(1), NoSave(), NoReplay(), Checks(100), Budget(2*time.Second))
		r := readReports(t, path)[0]
		s := r.Statistics
		if !r.BudgetEnded || r.Passed != 2 || r.Durations.Search != 2 || r.Durations.Total < 2 {
			t.Fatal(r)
		}
		if s.Labels["class"] != 2 || s.Labels["label"] != 2 || s.Classes["a"] != 4 || s.Samples != 6 || s.Measures["length"].Mean != float64(2) || s.Measures["infinity"].Max != "+Inf" || s.Covers["half"].Verdict != "undecided" {
			t.Fatal(s)
		}
		if len(r.Objectives) != 1 || r.Objectives[0].Best != float64(7) {
			t.Fatal(r.Objectives)
		}
	})
}
func TestReportJSONMultipleSavedAndReplay(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.jsonl")
	dir := t.TempDir()
	tb := &fakeTB{name: t.Name()}
	prop := func(pt *T) {
		x := IntRange(0, 10).Draw(pt, "x")
		if x < 5 {
			pt.Error("low")
		} else {
			pt.Error("high")
		}
	}
	opts := []Option{ReportJSON(path), TestdataDir(dir), Seed(1), Checks(100), MaxFailures(2)}
	Check(tb, prop, opts...)
	first := readReports(t, path)[0]
	if len(first.Failures) != 2 {
		t.Fatal(first)
	}
	for _, f := range first.Failures {
		if _, err := os.Stat(f.SavedPath); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(f.Reproduce, "-pbt.replay=") || f.Headline == "" || len(f.Draws) != 1 {
			t.Fatal(f)
		}
	}
	Check(tb, prop, opts...)
	second := readReports(t, path)[1]
	if len(second.Failures) != 2 || second.Passed != 0 || second.Rejected != 0 {
		t.Fatal(second)
	}
}
func TestReportJSONConcurrentAppend(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.jsonl")
	t.Run("parallel", func(t *testing.T) {
		for i := range 24 {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				t.Parallel()
				for range 3 {
					Check(t, func(pt *T) { Int().Draw(pt, "x") }, ReportJSON(path), NoReplay(), NoSave(), Seed(1), Checks(2))
				}
			})
		}
	})
	if got := len(readReports(t, path)); got != 72 {
		t.Fatal(got)
	}
}
func TestReportJSONWriteErrorAndDisabled(t *testing.T) {
	tb := &fakeTB{}
	Check(tb, func(pt *T) {}, ReportJSON(t.TempDir()), Seed(1), NoSave(), NoReplay(), Checks(1))
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "cannot write JSON report") {
		t.Fatal(tb)
	}
	var draws [2][]int
	var logs [2][]string
	for i := range draws {
		tb := &fakeTB{name: "same"}
		opts := []Option{Seed(1), Checks(20), NoSave(), NoReplay(), Verbose()}
		if i == 1 {
			opts = append(opts, ReportJSON(filepath.Join(t.TempDir(), "report.jsonl")))
		}
		Check(tb, func(pt *T) { draws[i] = append(draws[i], Int().Draw(pt, "x")) }, opts...)
		logs[i] = tb.logs
	}
	if !slices.Equal(draws[0], draws[1]) || !slices.Equal(logs[0], logs[1]) {
		t.Fatal("reporting changed random choices or text output")
	}
	old := *flagJSON
	t.Cleanup(func() { *flagJSON = old })
	*flagJSON = "flag.jsonl"
	cfg, err := newConfig([]Option{ReportJSON("")})
	if err != nil || cfg.reportJSON != "" {
		t.Fatal(cfg, err)
	}
}

func TestReportJSONDrawLimitsAndFlagError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.jsonl")
	g := Custom("nested", func(pt *T) string { Just("inner").Draw(pt, "child"); return "abcdef" })
	Check(&fakeTB{name: t.Name()}, func(pt *T) { g.Draw(pt, "value"); Just("second").Draw(pt, "value"); pt.Error("failure") },
		ReportJSON(path), Seed(1), NoSave(), NoReplay(), ShrinkTime(0), ReportDepth(0), ReportLimit(3))
	r := readReports(t, path)[0]
	if len(r.Failures[0].Draws) != 2 || !strings.Contains(r.Failures[0].Draws[0].Value, "more characters") {
		t.Fatal(r.Failures)
	}
	old := *flagCheckHealth
	t.Cleanup(func() { *flagCheckHealth = old })
	*flagCheckHealth = "invalid"
	Check(&fakeTB{}, func(pt *T) { t.Fatal("invalid config ran property") }, ReportJSON(path))
	r = readReports(t, path)[1]
	if r.Status != "error" {
		t.Fatal(r)
	}
}
