// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"strings"
	"testing"
)

type expr struct {
	value int
	args  []*expr
}

// internal counts the nodes that have children; the budget of Recursive
// bounds those, while every node may hold as many leaves as its slice has.
func internal(e *expr) int {
	if len(e.args) == 0 {
		return 0
	}
	n := 1
	for _, a := range e.args {
		n += internal(a)
	}
	return n
}

var exprGen = Recursive(
	Map(Int(), func(v int) *expr { return &expr{value: v} }),
	func(self Gen[*expr]) Gen[*expr] {
		return Map(SliceOfN(self, 1, -1), func(args []*expr) *expr { return &expr{args: args} })
	},
)

func TestRecursiveStaysWithinBudget(t *testing.T) {
	Check(t, func(t *T) {
		e := exprGen.Draw(t, "e")
		t.Collect(internal(e))
		if n := internal(e); n > t.Size()+1 {
			t.Fatalf("%d internal nodes at size %d", n, t.Size())
		}
	}, NoSave())
	c := counterexample(t, func(t *T) {
		exprGen.Draw(t, "e")
		t.Fail()
	})
	if e := drawn[*expr](t, c, 0); len(e.args) != 0 || e.value != 0 {
		t.Errorf("minimal expression is %+v", e)
	}
}

func divide(a, b int) int { return a / b }

func TestNoPanicFindsTheDivisionByZero(t *testing.T) {
	c := counterexample(t, NoPanic(divide))
	if b := drawn[int](t, c, 1); b != 0 {
		t.Errorf("arg1 = %d, want 0", b)
	}
	if !strings.Contains(strings.Join(c.messages, "\n"), "panic: runtime error: integer divide by zero") {
		t.Errorf("messages: %q", c.messages)
	}
	Check(t, NoPanic(func(s string, xs []int, m map[string]bool) int { return len(s) + len(xs) + len(m) }), NoSave())
	Check(t, NoPanic(func(prefix string, parts ...int) string { return prefix }), NoSave(), Checks(20))
}

func TestVerboseLogsShrinkSteps(t *testing.T) {
	tb := &fakeTB{name: "TestVerbose"}
	Check(tb, func(t *T) {
		if x := IntRange(0, 1000).Draw(t, "x"); x >= 10 {
			t.Fatalf("x = %d", x)
		}
	}, Seed(1), NoSave(), Verbose())
	logs := strings.Join(tb.logs, "\n")
	if !strings.Contains(logs, "shrink step") || !strings.Contains(logs, "x (IntRange(0, 1000)) = ") {
		t.Errorf("verbose logs lack the shrink steps:\n%s", logs)
	}
}
