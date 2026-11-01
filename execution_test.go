// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"slices"
	"strings"
	"testing"
)

func TestConjunctionFailureSurvivesLaterDiscard(t *testing.T) {
	for _, fail := range []Named{
		{"Error", func(t *T) { t.Error("broken") }},
		{"Fatal", func(t *T) { t.Fatal("broken") }},
		{"panic", func(t *T) { panic("broken") }},
	} {
		t.Run(fail.Name, func(t *testing.T) {
			baseline := newT(t.Name(), nil, 1, 42, nil)
			baseline.execute(Conjunction(fail))
			c := newT(t.Name(), nil, 1, 42, nil)
			c.execute(Conjunction(fail, Named{"skip", func(t *T) { t.Assume(false) }}))
			if !c.failed || c.discarded || c.failKey != baseline.failKey {
				t.Fatalf("failed=%v discarded=%v key=%q, want failure %q", c.failed, c.discarded, c.failKey, baseline.failKey)
			}
			if !slices.Contains(c.messages, fail.Name+": broken") && fail.Name != "panic" {
				t.Fatalf("lost failure message: %v", c.messages)
			}
		})
	}
	// Without a preceding failure, a discard must still discard the case.
	c := newT(t.Name(), nil, 1, 42, nil)
	c.execute(Conjunction(Named{"skip", func(t *T) { t.SkipNow() }}))
	if c.failed || !c.discarded {
		t.Fatalf("failed=%v discarded=%v", c.failed, c.discarded)
	}
}

func TestConjunctionDoesNotPassByDiscardingFailures(t *testing.T) {
	tb := &fakeTB{name: t.Name()}
	Check(tb, func(t *T) {
		bad := Bool().Draw(t, "bad")
		Conjunction(
			Named{"check", func(t *T) {
				if bad {
					t.Error("broken")
				}
			}},
			Named{"skip", func(t *T) { t.Assume(!bad) }},
		)(t)
	}, Seed(1), NoSave(), NoReplay(), ShrinkTime(0))
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "check: broken") {
		t.Fatalf("failure was not reported: %v", tb.errors)
	}
}

func TestCleanupCannotReplaceBodyFailure(t *testing.T) {
	body := func(t *T) { panic("body failure") }
	baseline := newT(t.Name(), nil, 1, 42, nil)
	baseline.execute(body)
	for _, cleanup := range []Named{
		{"skip", func(t *T) { t.SkipNow() }},
		{"panic", func(t *T) { panic("cleanup failure") }},
	} {
		t.Run(cleanup.Name, func(t *testing.T) {
			var ran []int
			c := newT(t.Name(), nil, 1, 42, nil)
			c.execute(func(t *T) {
				t.Cleanup(func() { ran = append(ran, 1) })
				t.Cleanup(func() { ran = append(ran, 2); cleanup.Prop(t) })
				body(t)
			})
			if !c.failed || c.discarded || c.panicVal != "body failure" || c.failKey != baseline.failKey {
				t.Fatalf("failed=%v discarded=%v panic=%v key=%q, want body failure %q", c.failed, c.discarded, c.panicVal, c.failKey, baseline.failKey)
			}
			if !slices.Equal(ran, []int{2, 1}) {
				t.Fatalf("cleanup order = %v", ran)
			}
		})
	}
}

func TestCleanupFailureBeatsDiscard(t *testing.T) {
	for _, body := range []Named{
		{"pass", func(*T) {}},
		{"discard", func(t *T) { t.SkipNow() }},
	} {
		for _, skipFirst := range []bool{false, true} {
			c := newT(t.Name(), nil, 1, 42, nil)
			c.execute(func(t *T) {
				cleanup := []func(){func() { panic("cleanup failure") }, func() { t.SkipNow() }}
				if skipFirst {
					slices.Reverse(cleanup)
				}
				for _, f := range cleanup {
					t.Cleanup(f)
				}
				body.Prop(t)
			})
			if !c.failed || c.discarded || c.panicVal != "cleanup failure" {
				t.Fatalf("body=%s skipFirst=%v: failed=%v discarded=%v panic=%v", body.Name, skipFirst, c.failed, c.discarded, c.panicVal)
			}
		}
	}
}

func TestExamplePropagatesCleanupPanic(t *testing.T) {
	g := Custom("resource", func(t *T) int {
		t.Cleanup(func() { panic("cleanup failure") })
		return 1
	})
	if got := Recover(func() { g.Example(1) }); got != "cleanup failure" {
		t.Fatalf("panic = %v", got)
	}
}
