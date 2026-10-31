// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"strings"
	"testing"
)

// walkDistance is the example of targeted testing from the PropEr
// documentation: a random walk of unit steps, where the search looks for
// walks that end far from the origin. Random walks of n steps end about
// sqrt(n) away, so a distance of 30 is rare without guidance.
func walkDistance(steps []int) int {
	pos := 0
	for _, s := range steps {
		pos += s
	}
	if pos < 0 {
		return -pos
	}
	return pos
}

var step = Frequency(Weighted[int]{Weight: 1, Gen: Just(-1)}, Weighted[int]{Weight: 1, Gen: Just(1)})

func TestTargetedSearchFindsFarWalks(t *testing.T) {
	tb := &fakeTB{name: "TestTargeted"}
	Check(tb, func(t *T) {
		steps := SliceOf(step).Draw(t, "steps")
		d := walkDistance(steps)
		t.Maximize("distance", float64(d))
		if d >= 30 {
			t.Fatalf("distance %d", d)
		}
	}, Seed(1), NoSave(), SearchSteps(2000))
	if len(tb.errors) != 1 {
		t.Fatalf("targeted search did not reach distance 30 in 2000 steps: %q", tb.errors)
	}
	if !strings.Contains(tb.errors[0], "utility distance = 30") {
		t.Errorf("report lacks the utility line:\n%s", tb.errors[0])
	}
}

func TestMinimizeHomesInOnAValue(t *testing.T) {
	tb := &fakeTB{name: "TestMinimize"}
	Check(tb, func(t *T) {
		x := IntRange(0, 100000).Draw(t, "x")
		t.Minimize("gap", float64(max(x-50000, 50000-x)))
		if x == 50000 {
			t.Fatalf("hit")
		}
	}, Seed(1), NoSave(), SearchSteps(3000))
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "x (IntRange(0, 100000)) = 50000") {
		t.Fatalf("the search did not reach 50000: %q", tb.errors)
	}
}

func TestTargetedPassLogsTheBest(t *testing.T) {
	tb := &fakeTB{name: "TestTargetedPass"}
	Check(tb, func(t *T) {
		x := IntRange(0, 100).Draw(t, "x")
		t.Maximize("x", float64(x))
	}, Seed(1), NoSave(), SearchSteps(200))
	if len(tb.errors) != 0 {
		t.Fatalf("unexpected failure: %q", tb.errors)
	}
	logs := strings.Join(tb.logs, "\n")
	if !strings.Contains(logs, "best x = 100 over 200 search steps") {
		t.Errorf("logs lack the best utility:\n%s", logs)
	}
}

func TestCheckExistsFindsAndShrinksAWitness(t *testing.T) {
	tb := &fakeTB{name: "TestExists"}
	CheckExists(tb, func(t *T) {
		x := IntRange(0, 1000).Draw(t, "x")
		if x < 900 {
			t.Fail()
		}
	}, Seed(1), NoSave(), Checks(500))
	if len(tb.errors) != 0 {
		t.Fatalf("unexpected failure: %q", tb.errors)
	}
	logs := strings.Join(tb.logs, "\n")
	for _, want := range []string{"witness found after", "x (IntRange(0, 1000)) = 900"} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %q:\n%s", want, logs)
		}
	}
}

func TestCheckExistsReportsMissingWitness(t *testing.T) {
	tb := &fakeTB{name: "TestNoWitness"}
	CheckExists(tb, func(t *T) {
		Int().Draw(t, "x")
		t.Fail()
	}, Seed(1), NoSave(), Checks(50))
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "no witness found in 50 cases") {
		t.Errorf("got %q", tb.errors)
	}
}

func TestCheckExistsWithGuidance(t *testing.T) {
	tb := &fakeTB{name: "TestExistsTargeted"}
	CheckExists(tb, func(t *T) {
		steps := SliceOf(step).Draw(t, "steps")
		d := walkDistance(steps)
		t.Maximize("distance", float64(d))
		if d < 25 {
			t.Fail()
		}
	}, Seed(1), NoSave(), SearchSteps(2000))
	if len(tb.errors) != 0 {
		t.Fatalf("no witness: %q", tb.errors)
	}
}
