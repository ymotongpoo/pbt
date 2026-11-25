// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// The third Inc silently breaks the counter. A Get only detects the bug
// after eight unrelated commands; fresh continuations seldom plant it.
func causalCounter() Property {
	step, counter := 0, 0
	broken := false
	next := func(Var) { step++ }
	inc := Command{Name: "Inc", Args: []any{1}, Next: next, Run: func(Env) any {
		counter++
		if counter == 3 {
			broken = true
		}
		return nil
	}}
	nop := Command{Name: "Nop", Next: next}
	get := Command{Name: "Get", Next: next, Post: func(Env, any) error {
		if broken {
			return fmt.Errorf("counter broke on its third increment")
		}
		return nil
	}}
	gen := func() CommandGen {
		switch step {
		case 0, 1:
			return Just(inc)
		case 2:
			return Frequency(W(1, Just(inc)), W(15, Just(nop)))
		case 11:
			return Just(get)
		default:
			return Just(nop)
		}
	}
	return func(pt *T) {
		counter, broken = 0, false
		Commands(func() { step = 0 }, gen, Length(12, 12)).Draw(pt, "program").RunSequential(pt)
	}
}

func TestCausalityFindsPlantingStepAndIsDeterministic(t *testing.T) {
	run := func() string {
		prop := causalCounter()
		replays, continuations := 0, 0
		tb := healthRun(func(pt *T) {
			if pt.rng == nil {
				replays++
			} else if pt.causal == nil {
				continuations++
				if replays != 1 {
					t.Error("analysis ran after shrinking began")
				}
			}
			prop(pt)
		}, Causality(64), Checks(1000))
		if len(tb.errors) != 1 {
			t.Fatal(healthOutput(tb))
		}
		if continuations == 0 || continuations > 16*64 {
			t.Fatal("continuation budget", continuations)
		}
		return tb.errors[0]
	}
	a, b := run(), run()
	if a != b {
		t.Fatalf("same seed produced different reports:\n%s\n%s", a, b)
	}
	for _, want := range []string{
		"step 12: postcondition of Get() failed",
		"causality (64 continuations per prefix, seed 0x1):",
		"after 0 of 12 steps:",
		"after 2 of 12 steps:",
		"after 3 of 12 steps: 64/64 fail with the same identity <- largest measured rise at step 3: Inc(1)",
	} {
		if !strings.Contains(a, want) {
			t.Fatalf("missing %q:\n%s", want, a)
		}
	}
	t.Log(a)
}

func TestCausalitySkipsUnstableFailure(t *testing.T) {
	for _, outcome := range []string{"pass", "discard", "different failure"} {
		t.Run(outcome, func(t *testing.T) {
			calls := 0
			tb := healthRun(func(pt *T) {
				calls++
				Commands(nil, func() CommandGen { return Just(Command{Name: "Nop"}) }, Length(1, 1)).Draw(pt, "program")
				if calls == 1 {
					pt.Fatal("original failure")
				}
				switch outcome {
				case "discard":
					pt.Skip("later discarded")
				case "different failure":
					pt.Fatal("another failure")
				}
			}, Causality(64), ShrinkTime(0))
			if calls != 2 || !strings.Contains(healthOutput(tb), "skipped: the full input did not reproduce the same failure (flaky)") {
				t.Fatal(calls, healthOutput(tb))
			}
		})
	}
}

func TestCausalityOffPreservesOutputAndChoices(t *testing.T) {
	prop := causalCounter()
	run := func(opts ...Option) (*fakeTB, [][]uint64) {
		var choices [][]uint64
		tb := healthRun(func(pt *T) {
			defer func() { choices = append(choices, slices.Clone(pt.choices)) }()
			prop(pt)
		}, append([]Option{Checks(1000)}, opts...)...)
		return tb, choices
	}
	a, ac := run()
	b, bc := run(Causality(0))
	if healthOutput(a) != healthOutput(b) || !reflect.DeepEqual(ac, bc) || strings.Contains(healthOutput(a), "causality") {
		t.Fatal("disabled causality changed output or choices", healthOutput(a), healthOutput(b))
	}
	// Opt-in tracing also preserves the generation stream and shrink spans,
	// including deterministic commands whose one-choice spans are dropped.
	cfg, _ := newConfig([]Option{Seed(1)})
	r := runner{tb: &fakeTB{}, cfg: cfg, prop: prop}
	off := r.run(nil, 12, newRand(1))
	r.cfg.causality = 1
	on := r.run(nil, 12, newRand(1))
	if !slices.Equal(off.choices, on.choices) || !reflect.DeepEqual(off.spans, on.spans) || off.causal != nil || len(on.causal.steps) != 12 {
		t.Fatal("tracing changed choices/spans or lost deterministic commands")
	}
}

func TestCausalityRatesAndIsolation(t *testing.T) {
	const continuations = 64
	cfg, _ := newConfig([]Option{Seed(1), Causality(continuations), MaxFailures(3), TestdataDir(t.TempDir())})
	tb := &fakeTB{name: "isolated"}
	setups, cleanups, onFail, calls := 0, 0, 0, 0
	counts := [3]int{}
	cfg.setup = func(pt *T) func() { setups++; return func() { cleanups++ } }
	r := runner{tb: tb, cfg: cfg, health: newHealthStats(), stats: newStats(), prop: func(pt *T) {
		calls++
		kind := SampledFrom([]int{0, 1, 2}).Draw(pt, "kind")
		Commands(nil, func() CommandGen { return Just(Command{Name: "Nop"}) }, Length(1, 1)).Draw(pt, "program").RunSequential(pt)
		pt.OnFail(func() { onFail++ })
		pt.Classify(true, "class")
		pt.Collect("sample")
		pt.Measure("measurement", 42)
		pt.Cover(true, 100, "covered")
		pt.Maximize("objective", 42)
		Filter(IntRange(0, 1), func(n int) bool { return n == 0 }).Draw(pt, "filtered")
		if pt.rng != nil && len(pt.prefix) == 0 {
			counts[kind]++
		}
		switch kind {
		case 0:
			pt.Fatal("target failure")
		case 1:
			pt.Fatal("another failure")
		case 2:
			pt.Skip("discarded continuation")
		}
	}}
	original := r.run(nil, 10, nil)
	if !original.failed {
		t.Fatal("fixture did not fail")
	}
	r.stableFailure(original)
	out := strings.Join(r.causality, "\n")
	want := fmt.Sprintf("after 0 of 1 steps: %d/%d fail with the same identity (%d discarded)", counts[0], counts[0]+counts[1], counts[2])
	if counts[0] == 0 || counts[1] == 0 || counts[2] == 0 || !strings.Contains(out, want) || !strings.Contains(out, "after 1 of 1 steps: 64/64 fail with the same identity") {
		t.Fatal(counts, out)
	}
	if calls != 2+2*continuations || setups != calls || cleanups != calls || onFail != 0 {
		t.Fatal("execution lifecycle changed", calls, setups, cleanups, onFail)
	}
	if !reflect.DeepEqual(r.health, newHealthStats()) || !reflect.DeepEqual(r.stats, newStats()) || len(r.objectives) != 0 || len(r.failures) != 0 || len(tb.errors)+len(tb.logs) != 0 {
		t.Fatal("continuations leaked into runner state", r.health, r.stats, r.objectives, r.failures, tb)
	}
	files, err := os.ReadDir(cfg.testdata)
	if err != nil || len(files) != 0 {
		t.Fatal("continuations saved files", files, err)
	}
}

func TestCausalityAllDiscardsAtPrefix(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1), Causality(16)})
	r := runner{tb: &fakeTB{}, cfg: cfg, prop: func(pt *T) {
		// The original replay can use the zero-weight alternative; fresh
		// generation always discards, so the prefix-zero rate is undefined.
		allowed := Frequency(W(0, Just(true)), W(1, Just(false))).Draw(pt, "allowed")
		pt.Assume(allowed)
		Commands(nil, func() CommandGen { return Just(Command{Name: "Nop"}) }, Length(1, 1)).Draw(pt, "program")
		pt.Fatal("failure")
	}}
	r.stableFailure(r.run(nil, 10, nil))
	out := strings.Join(r.causality, "\n")
	if !strings.Contains(out, "after 0 of 1 steps: all 16 continuations discarded; rate unavailable") ||
		!strings.Contains(out, "after 1 of 1 steps: 16/16 fail with the same identity") || strings.Contains(out, "largest measured rise") {
		t.Fatal(out)
	}
}

func TestCausalityScopeAndOptions(t *testing.T) {
	old := *flagCausality
	defer func() { *flagCausality = old }()
	*flagCausality = 4
	for _, tc := range []struct {
		name string
		prop Property
		want string
	}{
		{"ordinary", func(pt *T) { Int().Draw(pt, "x"); pt.Fatal("failure") }, "no sequential command steps"},
		{"parallel", func(pt *T) {
			ParallelCommands(nil, func() CommandGen { return Just(Command{Name: "Nop"}) }, Length(1, 1)).Draw(pt, "program")
			pt.Fatal("failure")
		}, "ParallelCommands generation order is not execution order"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := healthRun(tc.prop)
			if !strings.Contains(healthOutput(tb), "causality (4 continuations") || !strings.Contains(healthOutput(tb), tc.want) {
				t.Fatal(healthOutput(tb))
			}
		})
	}
	tb := healthRun(multipleBugs, Causality(0))
	if strings.Contains(healthOutput(tb), "causality") {
		t.Fatal("option did not override flag", healthOutput(tb))
	}
	*flagCausality = -1
	tb = healthRun(multipleBugs)
	if !strings.Contains(healthOutput(tb), "Causality(-1)") {
		t.Fatal(healthOutput(tb))
	}
	tb = &fakeTB{}
	CheckExists(tb, func(*T) {}, Seed(1), Causality(-2))
	if len(tb.errors) != 0 || strings.Contains(healthOutput(tb), "causality") {
		t.Fatal("CheckExists did not ignore causality", healthOutput(tb))
	}
}

func TestCausalityOnlyFirstFailureIncludingReplay(t *testing.T) {
	dir := t.TempDir()
	opts := []Option{Seed(1), MaxFailures(2), Causality(8), TestdataDir(dir)}
	prop := func(pt *T) {
		Commands(nil, func() CommandGen { return Just(Command{Name: "Nop"}) }, Length(1, 1)).Draw(pt, "program")
		multipleBugs(pt)
	}
	check := func(extra ...Option) {
		tb := &fakeTB{name: "two"}
		Check(tb, prop, append(slices.Clone(opts), extra...)...)
		if len(tb.errors) != 2 || strings.Count(healthOutput(tb), "causality (") != 1 || !strings.Contains(tb.errors[0], "causality (") {
			t.Fatal(healthOutput(tb))
		}
	}
	check()
	check()
	files, err := os.ReadDir(filepath.Join(dir, "two"))
	if err != nil || len(files) != 2 {
		t.Fatal(files, err)
	}
	// Also exercise both single-failure replay paths, where stableFailure's
	// returned case is intentionally not used by Check.
	checkSingle := func(extra ...Option) {
		tb := &fakeTB{name: "two"}
		Check(tb, prop, append(append(slices.Clone(opts), MaxFailures(1)), extra...)...)
		if len(tb.errors) != 1 || strings.Count(healthOutput(tb), "causality (") != 1 {
			t.Fatal(healthOutput(tb))
		}
	}
	checkSingle()
	checkSingle(ReplayFile(filepath.Join(dir, "two", files[0].Name())))
}

func TestCausalityLongTraceBudget(t *testing.T) {
	const length, planting = 4096, 1234
	step, calls := 0, 0
	broken := false
	next := func(Var) { step++ }
	gen := func() CommandGen {
		nop := Just(Command{Name: "Nop", Next: next})
		if step == planting-1 {
			// A zero-weight bug gives a sharp, noise-free jump when replayed,
			// even with one continuation at each measured prefix.
			return Frequency(W(0, Just(Command{Name: "Plant", Next: next, Run: func(Env) any { broken = true; return nil }})), W(1, nop))
		}
		return nop
	}
	cfg, _ := newConfig([]Option{Seed(1), Causality(1)})
	r := runner{tb: &fakeTB{}, cfg: cfg, prop: func(pt *T) {
		calls++
		broken = false
		Commands(func() { step = 0 }, gen, Length(length, length)).Draw(pt, "program").RunSequential(pt)
		if broken {
			pt.Fatal("detected after the whole program")
		}
	}}
	original := r.run(nil, 10, nil)
	r.stableFailure(original)
	out := strings.Join(r.causality, "\n")
	if calls != 18 || !strings.Contains(out, "prefix budget exhausted") || strings.Contains(out, "largest measured rise at step") {
		t.Fatal(calls, out)
	}
}

func TestCausalityPreservesSavedCounterexample(t *testing.T) {
	run := func(n int) string {
		dir := t.TempDir()
		tb := &fakeTB{name: "saved"}
		Check(tb, causalCounter(), Seed(1), Causality(n), Checks(1000), TestdataDir(dir))
		files, err := os.ReadDir(filepath.Join(dir, "saved"))
		if err != nil || len(files) != 1 {
			t.Fatal(files, err, healthOutput(tb))
		}
		data, err := os.ReadFile(filepath.Join(dir, "saved", files[0].Name()))
		if err != nil {
			t.Fatal(err)
		}
		return files[0].Name() + "\n" + string(data)
	}
	if off, on := run(0), run(16); off != on {
		t.Fatalf("analysis changed saved counterexample:\n%s\n%s", off, on)
	}
}
