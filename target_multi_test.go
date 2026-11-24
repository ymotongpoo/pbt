// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

func TestTargetedSeedPin(t *testing.T) {
	h := sha256.New()
	tb := &fakeTB{name: "pin"}
	Check(tb, func(pt *T) {
		x := IntRange(0, 100).Draw(pt, "x")
		fmt.Fprintf(h, "%v;", pt.choices)
		pt.Maximize("x", float64(x))
	}, Seed(1), NoSave(), NoReplay(), SearchSteps(200))
	// Captured from v0.3.4 (c062a63), including every generated search case.
	const want = "7cb870087a2b5249a47107156ebdf5aa084472228dc4e4fbcbafa5df4f837cb4"
	if got := fmt.Sprintf("%x", h.Sum(nil)); got != want {
		t.Fatal(got)
	}
	if len(tb.errors) != 0 {
		t.Fatal(tb.errors)
	}
}

func oppositeObjectives(pt *T) {
	x := IntRange(0, 100).Draw(pt, "x")
	pt.Maximize("high", float64(x))
	pt.Minimize("low", float64(x))
}

func TestSeveralObjectivesReachTheirOptima(t *testing.T) {
	tb := healthRun(oppositeObjectives, SearchSteps(500))
	out := healthOutput(tb)
	for _, want := range []string{"best high = 100", "best low = 0"} {
		if !strings.Contains(out, want) {
			t.Fatal(out)
		}
	}
	if len(tb.errors) != 0 {
		t.Fatal(out)
	}
	t.Log(out)
	tb = healthRun(func(pt *T) { oppositeObjectives(pt); pt.Fatal("both utilities") })
	out = healthOutput(tb)
	if !strings.Contains(out, "utility high = 0") || !strings.Contains(out, "utility low = 0") {
		t.Fatal(out)
	}
	t.Log(out)
}

func TestSeveralObjectivesExist(t *testing.T) {
	tb := &fakeTB{name: "exists"}
	CheckExists(tb, func(pt *T) {
		x := IntRange(0, 100).Draw(pt, "x")
		pt.Maximize("high", float64(x))
		pt.Minimize("low", float64(x))
		if x != 100 {
			pt.Fail()
		}
	}, Seed(1), SearchSteps(500), MaxFailures(5))
	if len(tb.errors) != 0 || !strings.Contains(healthOutput(tb), "= 100") {
		t.Fatal(healthOutput(tb))
	}
}

func TestObjectivesMissingFromCaseAreNotAdopted(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1)})
	r := &runner{cfg: cfg}
	a, b := &T{}, &T{}
	a.Maximize("a", 10)
	b.Maximize("b", 20)
	rng := newRand(1)
	r.observeObjectives(a, -1, 1, rng)
	r.observeObjectives(b, 0, 1, rng)
	if len(r.objectives) != 2 || r.objectives[0].current != a || r.objectives[1].current != b {
		t.Fatal(r.objectives)
	}
	// Updating one objective must not evaluate another objective's annealing.
	c := &T{}
	c.Maximize("a", 0)
	c.Maximize("b", 100)
	r.observeObjectives(c, 0, 0, rng)
	if r.objectives[0].current != a || r.objectives[1].current != b || r.objectives[1].best != c {
		t.Fatal(r.objectives)
	}
}

func TestSeveralInteriorOptima(t *testing.T) {
	tb := healthRun(func(pt *T) {
		x := IntRange(0, 200000).Draw(pt, "x")
		pt.Maximize("left", -float64(max(x-50000, 50000-x)))
		pt.Maximize("right", -float64(max(x-150000, 150000-x)))
	}, SearchSteps(12000))
	out := healthOutput(tb)
	if !strings.Contains(out, "best left = 0") || !strings.Contains(out, "best right = 0") {
		t.Fatal(out)
	}
}
