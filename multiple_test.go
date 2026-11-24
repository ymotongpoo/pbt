// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func multipleBugs(t *T) {
	x := IntRange(-100, 100).Draw(t, "x")
	if x > 10 {
		t.Fatal("positive bug")
	}
	if x < -10 {
		t.Fatal("negative bug")
	}
}

func TestMultipleFailuresShrinkSaveReplay(t *testing.T) {
	dir := t.TempDir()
	tb := &fakeTB{name: "two"}
	opts := []Option{Seed(1), MaxFailures(2), TestdataDir(dir)}
	Check(tb, multipleBugs, opts...)
	if len(tb.errors) != 2 {
		t.Fatal(healthOutput(tb))
	}
	out := healthOutput(tb)
	for _, want := range []string{"failure 1 of 2", "failure 2 of 2", "= 11", "= -11", "positive bug", "negative bug"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q: %s", want, out)
		}
	}
	t.Log(out)
	files, err := os.ReadDir(filepath.Join(dir, "two"))
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	calls := 0
	tb = &fakeTB{name: "two"}
	Check(tb, func(pt *T) { calls++; multipleBugs(pt) }, opts...)
	if len(tb.errors) != 2 || calls != 4 {
		t.Fatal(calls, healthOutput(tb))
	}
	for _, report := range tb.errors {
		if !strings.Contains(report, "still fails") {
			t.Fatal(report)
		}
	}
	// Explicit replay and automatic replay must not duplicate the same identity.
	tb = &fakeTB{name: "two"}
	Check(tb, multipleBugs, append(opts, ReplayFile(filepath.Join(dir, "two", files[0].Name())))...)
	if len(tb.errors) != 2 {
		t.Fatal(healthOutput(tb))
	}
}

func TestMultipleFailureDefaultUnchanged(t *testing.T) {
	a, b := &fakeTB{name: "one"}, &fakeTB{name: "one"}
	Check(a, multipleBugs, Seed(1), NoSave(), NoReplay())
	Check(b, multipleBugs, Seed(1), NoSave(), NoReplay(), MaxFailures(1))
	if len(a.errors) != 1 || strings.Join(a.errors, "") != strings.Join(b.errors, "") {
		t.Fatal(a.errors, b.errors)
	}
	tb := healthRun(multipleBugs, MaxFailures(0))
	if !strings.Contains(healthOutput(tb), "MaxFailures(0)") {
		t.Fatal(healthOutput(tb))
	}
}

func TestShrinkDiscoversAnotherFailure(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1), MaxFailures(2), NoSave()})
	r := &runner{tb: &fakeTB{name: "slip"}, cfg: cfg, health: newHealthStats(), prop: func(pt *T) {
		x := IntRange(0, 100).Draw(pt, "x")
		if x >= 10 {
			pt.Fatal("large bug")
		}
		pt.Fatal("small bug")
	}}
	original := r.run([]uint64{20}, 1, nil)
	if !strings.Contains(strings.Join(original.messages, ""), "large") {
		t.Fatal(original.messages)
	}
	r.collectFailure(original)
	a, _ := r.shrink(original)
	if len(r.failures) != 2 {
		t.Fatal("second identity not collected")
	}
	b, _ := r.shrink(r.failures[1])
	if a.draws[0].value != 10 || b.draws[0].value != 0 || a.failKey == b.failKey {
		t.Fatal(a.draws, b.draws)
	}
}

func TestMultipleFailureBudgetAndExists(t *testing.T) {
	calls := 0
	tb := healthRun(func(pt *T) {
		if pt.rng != nil {
			calls++
		}
		pt.Fatal("same")
	}, MaxFailures(3), Checks(7))
	if calls != 7 || len(tb.errors) != 1 {
		t.Fatal(calls, healthOutput(tb))
	}
	calls = 0
	tb = healthRun(func(pt *T) {
		if pt.rng != nil {
			calls++
		}
		pt.Maximize("x", float64(IntRange(0, 100).Draw(pt, "x")))
		pt.Fatal("same")
	}, MaxFailures(3), Checks(7), SearchSteps(20))
	if calls != 20 || len(tb.errors) != 1 {
		t.Fatal(calls, healthOutput(tb))
	}
	tb = &fakeTB{}
	CheckExists(tb, func(pt *T) {}, MaxFailures(3), NoSave(), Seed(1))
	if len(tb.errors) != 0 {
		t.Fatal(healthOutput(tb))
	}
}

func TestMultipleFailureHealthNotes(t *testing.T) {
	attempts := 0
	tb := healthRun(func(pt *T) {
		attempts++
		if attempts <= 200 {
			pt.Skip("warmup")
		}
		multipleBugs(pt)
	}, MaxFailures(2), ShrinkTime(0))
	if len(tb.errors) != 2 {
		t.Fatal(healthOutput(tb))
	}
	if strings.Count(tb.errors[0], "health: HealthFilter:") != 1 || strings.Contains(tb.errors[1], "health: HealthFilter:") {
		t.Fatal(healthOutput(tb))
	}
	for _, report := range tb.errors {
		if strings.Count(report, "health: HealthShrink:") != 1 {
			t.Fatal(report)
		}
	}
}

func TestMaxFailuresFlagAndExistsIgnore(t *testing.T) {
	old := *flagFailures
	*flagFailures = 2
	defer func() { *flagFailures = old }()
	tb := healthRun(multipleBugs)
	if len(tb.errors) != 2 {
		t.Fatal(healthOutput(tb))
	}
	*flagFailures = 0
	tb = &fakeTB{}
	CheckExists(tb, func(pt *T) {}, Seed(1), MaxFailures(-1))
	if len(tb.errors) != 0 {
		t.Fatal(healthOutput(tb))
	}
}

// A failed stability replay must not bypass the shrinker, even when no
// random seed would reliably reproduce the scheduler's decision.
func TestFlakyFailureStillShrinks(t *testing.T) {
	calls := 0
	cfg, _ := newConfig([]Option{Seed(1), NoSave()})
	r := &runner{tb: &fakeTB{}, cfg: cfg, health: newHealthStats(), prop: func(pt *T) {
		IntRange(0, 100).Draw(pt, "x")
		calls++
		if calls != 2 {
			pt.Fatal("intermittent")
		}
	}}
	original := r.run([]uint64{50}, 1, nil)
	seed := r.stableFailure(original)
	minimal, steps := r.shrink(seed)
	if minimal.draws[0].value != 0 || steps == 0 || len(r.healthExtra) != 1 || r.healthExtra[0].check != HealthFlaky {
		t.Fatal(minimal.draws, steps, r.healthExtra)
	}
}
