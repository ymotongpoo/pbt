// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func healthRun(prop Property, opts ...Option) *fakeTB {
	tb := &fakeTB{name: "health"}
	Check(tb, prop, append([]Option{Seed(1), NoSave(), NoReplay()}, opts...)...)
	return tb
}
func healthOutput(tb *fakeTB) string {
	return strings.Join(append(append([]string(nil), tb.logs...), tb.errors...), "\n")
}

func TestHealthFilterDiscards(t *testing.T) {
	for _, mode := range []string{"Assume", "Skip", "SkipNow", "Skipf"} {
		t.Run(mode, func(t *testing.T) {
			n := 0
			tb := healthRun(func(pt *T) {
				n++
				if n%2 == 1 {
					switch mode {
					case "Assume":
						pt.Assume(false)
					case "Skip":
						pt.Skip("skip")
					case "SkipNow":
						pt.SkipNow()
					case "Skipf":
						pt.Skipf("skip %d", n)
					}
				}
			}, Checks(30))
			out := healthOutput(tb)
			if len(tb.errors) != 0 || !strings.Contains(out, "discarded 30/60 cases") || !strings.Contains(out, "health_test.go:") {
				t.Fatal(out)
			}
			t.Log(out)
		})
	}
}

func TestHealthFilterValuesAndOptions(t *testing.T) {
	for _, mode := range []string{"warn", "fail", "ignore", "ignore-first"} {
		t.Run(mode, func(t *testing.T) {
			n := 0
			g := Filter(Just(1), func(int) bool { n++; return n%10 == 0 })
			opts := []Option{Checks(20)}
			switch mode {
			case "fail":
				opts = append(opts, CheckHealth())
			case "ignore":
				opts = append(opts, CheckHealth(), IgnoreHealth(HealthFilter))
			case "ignore-first":
				opts = append(opts, IgnoreHealth(HealthFilter), CheckHealth())
			}
			tb := healthRun(func(pt *T) { g.Draw(pt, "") }, opts...)
			out := healthOutput(tb)
			if strings.HasPrefix(mode, "ignore") {
				if out != "" {
					t.Fatal(out)
				}
				return
			}
			if !strings.Contains(out, "180/200 values rejected") || (mode == "fail") != (len(tb.errors) > 0) {
				t.Fatal(out)
			}
			t.Log(out)
		})
	}
}

func TestHealthGaveUpBreakdown(t *testing.T) {
	tb := healthRun(func(pt *T) { Filter(Int(), func(int) bool { return false }).Draw(pt, "") }, Checks(2))
	out := healthOutput(tb)
	for _, want := range []string{"gave up after 21 discarded", "Filter(Int()): 21 discards", "2100/2100 values rejected"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s: %s", want, out)
		}
	}
	t.Log(out)
}

func TestHealthLarge(t *testing.T) {
	g := Custom("huge", func(pt *T) int {
		for range healthLargeChoices + 1 {
			pt.draw(1, plainMeta, modeUniform)
		}
		return 1
	})
	tb := healthRun(func(pt *T) { huge := g.Draw(pt, ""); _ = huge }, Checks(10))
	out := healthOutput(tb)
	if len(tb.errors) != 0 || !strings.Contains(out, "10/10 cases exceeded 10000 choices") || !strings.Contains(out, "huge: 100010 choices") {
		t.Fatal(out)
	}
	t.Log(out)
}

func TestHealthSlow(t *testing.T) {
	g := Custom("slow", func(*T) int { time.Sleep(11 * time.Millisecond); return 1 })
	tb := healthRun(func(pt *T) { g.Draw(pt, "") }, Checks(100), CheckHealth())
	out := healthOutput(tb)
	if len(tb.errors) != 0 || !strings.Contains(out, "HealthSlow") {
		t.Fatal(out)
	}
	t.Log(out)
	tb = healthRun(func(pt *T) { g.Draw(pt, "") }, Checks(100), CheckHealth(HealthSlow))
	if len(tb.errors) != 1 {
		t.Fatal(healthOutput(tb))
	}
}

func TestHealthFlaky(t *testing.T) {
	for _, outcome := range []string{"passed", "failed elsewhere", "discarded"} {
		t.Run(outcome, func(t *testing.T) {
			n := 0
			tb := healthRun(func(pt *T) {
				Int().Draw(pt, "x")
				n++
				if n%2 == 1 {
					pt.Fatal("original")
				}
				switch outcome {
				case "failed elsewhere":
					pt.Fatal("other")
				case "discarded":
					pt.Skip("skip")
				}
			})
			out := healthOutput(tb)
			if n < 2 || !strings.Contains(out, "replay "+outcome) || strings.Contains(out, "shrinking and saving skipped") {
				t.Fatal(out)
			}
			t.Log(out)
		})
	}
}

func TestHealthShrinkBudget(t *testing.T) {
	tb := healthRun(func(pt *T) {
		SliceOfN(Int(), 10, 10).Draw(pt, "xs")
		time.Sleep(3 * time.Millisecond)
		pt.Fatal("bad")
	}, ShrinkTime(time.Millisecond))
	out := healthOutput(tb)
	for _, want := range []string{"HealthShrink", "shrinking stopped after ", " steps:", "ShrinkTime", "-pbt.shrinktime"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %s: %s", want, out)
		}
	}
	t.Log(out)
}

func TestHealthQuietAndRandomness(t *testing.T) {
	for _, opts := range [][]Option{nil, {CheckHealth()}, {IgnoreHealth(HealthFilter, HealthLarge, HealthSlow)}} {
		tb := healthRun(func(pt *T) { n := Int().Draw(pt, ""); pt.Assume(n%3 != 0) }, opts...)
		// A moderate rejection rate is useful, not unhealthy.
		if out := healthOutput(tb); out != "" {
			t.Fatal(out)
		}
	}
	cfg, _ := newConfig([]Option{Seed(1)})
	prop := func(pt *T) {
		Filter(Int(), func(n int) bool { return n%2 == 0 }).Draw(pt, "")
		SliceOf(Int()).Draw(pt, "")
	}
	a := &runner{tb: &fakeTB{}, cfg: cfg, prop: prop}
	b := &runner{tb: &fakeTB{}, cfg: cfg, prop: prop, health: newHealthStats()}
	for seed := uint64(0); seed < 20; seed++ {
		x, y := a.run(nil, 20, newRand(seed)), b.run(nil, 20, newRand(seed))
		if fmt.Sprint(x.choices) != fmt.Sprint(y.choices) {
			t.Fatal("health changed choices")
		}
	}
}

func TestHealthIgnoresPropertyTimeAndNestedDrawTime(t *testing.T) {
	// A nested draw's duration is included once by its top-level owner.
	g := Custom("outer", func(pt *T) int {
		return Custom("inner", func(*T) int { time.Sleep(2 * time.Millisecond); return 0 }).Draw(pt, "inner")
	})
	cfg, _ := newConfig(nil)
	r := &runner{tb: &fakeTB{}, cfg: cfg, health: newHealthStats(), prop: func(pt *T) { g.Draw(pt, ""); time.Sleep(30 * time.Millisecond) }}
	start := time.Now()
	r.run(nil, 1, newRand(1))
	elapsed := time.Since(start)
	if r.health.generation >= elapsed-20*time.Millisecond || len(r.health.draws) != 1 {
		t.Fatalf("generation %s, elapsed %s, draws %v", r.health.generation, elapsed, r.health.draws)
	}
}

func TestHealthThresholdBoundaries(t *testing.T) {
	cfg, _ := newConfig(nil)
	r := &runner{cfg: cfg, health: newHealthStats()}
	h := r.health
	h.cases = 40
	h.discards = 19
	h.filters["f"] = filterHealth{99, 99}
	if len(r.healthNotes()) != 0 {
		t.Fatal(r.healthNotes())
	}
	h.discards = 20
	if len(r.healthNotes()) != 1 {
		t.Fatal(r.healthNotes())
	}
	h.discards = 0
	h.filters["f"] = filterHealth{100, 89}
	if len(r.healthNotes()) != 0 {
		t.Fatal(r.healthNotes())
	}
	h.filters["f"] = filterHealth{100, 90}
	if len(r.healthNotes()) != 1 {
		t.Fatal(r.healthNotes())
	}
}

func TestHealthFailingTrailerAndIgnore(t *testing.T) {
	for _, ignore := range []bool{false, true} {
		n := 0
		opts := []Option{}
		if ignore {
			opts = append(opts, IgnoreHealth(HealthFilter))
		}
		tb := healthRun(func(pt *T) {
			n++
			if n <= 20 {
				pt.Skip("discard")
			}
			pt.Fatal("failed")
		}, opts...)
		out := healthOutput(tb)
		if len(tb.errors) != 1 || strings.Contains(out, "health: HealthFilter") == ignore {
			t.Fatal(out)
		}
	}
}

func TestHealthFlakySavedEvenWhenIgnored(t *testing.T) {
	dir := t.TempDir()
	n := 0
	tb := &fakeTB{name: "flaky"}
	Check(tb, func(pt *T) {
		n++
		if n%2 == 1 {
			pt.Fatal("unstable")
		}
	}, Seed(1), TestdataDir(dir), IgnoreHealth(HealthFlaky))
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(entries) != 1 || strings.Contains(healthOutput(tb), "health:") {
		t.Fatal(n, entries, healthOutput(tb))
	}
}

func TestHealthStepLimit(t *testing.T) {
	s := &shrinker{steps: maxShrinkSteps, deadline: time.Now().Add(time.Hour)}
	if !s.exhausted() || s.exhaustedBy != "step limit reached" {
		t.Fatal(s.exhaustedBy)
	}
}

func TestHealthNotCollectedOutsideCheck(t *testing.T) {
	g := Custom("standalone", func(pt *T) int {
		if pt.health != nil {
			t.Fatal("unexpected health counters")
		}
		return Int().Draw(pt, "")
	})
	g.Example(1)
	for range SampleShrink(g, 1) {
	}
	tb := &fakeTB{name: "exists"}
	CheckExists(tb, func(pt *T) { g.Draw(pt, "") }, Seed(1), CheckHealth(HealthFilter, HealthLarge, HealthSlow))
	if strings.Contains(healthOutput(tb), "health:") {
		t.Fatal(healthOutput(tb))
	}
}

func TestHealthNestingLimitSource(t *testing.T) {
	var g Gen[int]
	g = Custom("recursive", func(pt *T) int { return g.Draw(pt, "") })
	tb := healthRun(func(pt *T) { g.Draw(pt, "") }, Checks(1))
	if !strings.Contains(healthOutput(tb), "generator nesting limit: 11 discards") {
		t.Fatal(healthOutput(tb))
	}
}
