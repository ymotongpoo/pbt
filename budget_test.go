// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func TestBudgetSearch(t *testing.T) {
	for _, mode := range []string{"plain", "statistics", "targeted", "coverage", "exists", "failures", "discards"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				tb := &fakeTB{name: t.Name()}
				calls := 0
				opts := []Option{Seed(1), NoSave(), NoReplay(), Budget(3 * time.Second), Checks(100), ShrinkTime(0)}
				if mode == "coverage" {
					opts = append(opts, Checks(1), CheckCoverage())
				}
				if mode == "failures" {
					opts = append(opts, MaxFailures(5))
				}
				prop := func(pt *T) {
					calls++
					time.Sleep(time.Second)
					switch mode {
					case "targeted":
						pt.Maximize("value", float64(IntRange(0, 10).Draw(pt, "x")))
					case "coverage":
						pt.Cover(true, 50, "all")
					case "statistics":
						pt.Collect("all")
					case "exists", "failures":
						pt.Error("rejected")
					case "discards":
						pt.Skip("discarded")
					}
				}
				if mode == "exists" {
					CheckExists(tb, prop, opts...)
				} else {
					Check(tb, prop, opts...)
				}
				want := 3
				if mode == "failures" {
					want++ // stability replay remains outside the search budget
				}
				if calls != want {
					t.Fatalf("got %d calls, want %d", calls, want)
				}
				wantError := mode == "coverage" || mode == "exists" || mode == "failures"
				if (len(tb.errors) > 0) != wantError || !strings.Contains(strings.Join(tb.logs, "\n"), "3s budget") {
					t.Fatalf("errors %v; logs %v", tb.errors, tb.logs)
				}
			})
		})
	}
}

func TestBudgetReplayAndShrink(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "case")
		if err := os.WriteFile(path, []byte(persistHeader+"\nsize 1\nchoices\n9\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		calls := 0
		tb := &fakeTB{name: t.Name()}
		Check(tb, func(pt *T) { calls++; time.Sleep(2 * time.Second) },
			Seed(1), NoSave(), NoReplay(), ReplayFile(path), Checks(10), Budget(time.Second))
		if calls != 2 || len(tb.errors) != 0 {
			t.Fatalf("replay consumed search budget: calls=%d errors=%v", calls, tb.errors)
		}
		calls = 0
		Check(tb, func(pt *T) {
			calls++
			x := ShrinkTo(IntRange(10, 100), Just(0)).Draw(pt, "x")
			time.Sleep(2 * time.Second)
			if x >= 0 {
				pt.Error("failure")
			}
		}, Seed(2), NoSave(), NoReplay(), Sizes(42, 42), Budget(time.Second), ShrinkTime(10*time.Second))
		if calls <= 2 || len(tb.errors) == 0 {
			t.Fatalf("expected shrinking after deadline: calls=%d errors=%v", calls, tb.errors)
		}
	})
}

func TestBudgetDisabledAndCaseLimit(t *testing.T) {
	var sequences [2][]int
	for i := range sequences {
		tb := &fakeTB{}
		opts := []Option{Seed(1), NoSave(), NoReplay(), Checks(20)}
		if i == 1 {
			opts = append(opts, Budget(0))
		}
		Check(tb, func(pt *T) { sequences[i] = append(sequences[i], Int().Draw(pt, "x")) }, opts...)
		if len(tb.logs) != 0 || len(tb.errors) != 0 {
			t.Fatal(tb)
		}
	}
	if !slices.Equal(sequences[0], sequences[1]) {
		t.Fatal("disabled budget changed choices")
	}
	synctest.Test(t, func(t *testing.T) {
		tb := &fakeTB{}
		Check(tb, func(pt *T) { time.Sleep(time.Second) }, Seed(1), NoSave(), NoReplay(), Checks(2), Budget(2*time.Second))
		if len(tb.logs) != 0 || len(tb.errors) != 0 {
			t.Fatal("case limit should finish normally", tb)
		}
	})
	if _, err := newConfig([]Option{Budget(-time.Second)}); err == nil {
		t.Fatal("negative budget accepted")
	}
	saved := *flagBudget
	t.Cleanup(func() { *flagBudget = saved })
	*flagBudget = time.Second
	cfg, err := newConfig([]Option{Budget(0)})
	if err != nil || cfg.budget != 0 {
		t.Fatal("option did not override flag", cfg, err)
	}
}
