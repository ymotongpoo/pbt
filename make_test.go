// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"strings"
	"testing"
)

type inner struct {
	Flag bool
	Note string
}

type record struct {
	ID       int
	Name     string `pbt:"min=1,max=5"`
	Score    float64
	Tags     []string
	Counts   map[string]uint8
	Opt      *inner
	Fixed    [3]int16
	Age      int `pbt:"min=0,max=120"`
	skipped  int
	Ignored  int `pbt:"-"`
	Children []*record
}

type node struct {
	Value int
	Next  *node
}

func length(n *node) int {
	if n == nil {
		return 0
	}
	return 1 + length(n.Next)
}

func TestMakeGeneratesEveryExportedField(t *testing.T) {
	Check(t, func(t *T) {
		r := Make[record]().Draw(t, "r")
		if len(r.Name) < 1 || len([]rune(r.Name)) > 5 {
			t.Fatalf("Name %q violates its tag", r.Name)
		}
		if r.Age < 0 || r.Age > 120 {
			t.Fatalf("Age %d violates its tag", r.Age)
		}
		if r.skipped != 0 || r.Ignored != 0 {
			t.Fatalf("unexported or skipped fields were set: %+v", r)
		}
		t.Classify(r.Opt != nil, "pointer set")
		t.Classify(len(r.Children) > 0, "has children")
	}, NoSave())
}

func TestMakeRecursiveTypeTerminates(t *testing.T) {
	Check(t, func(t *T) {
		n := Make[*node]().Draw(t, "n")
		t.Collect(length(n))
	}, NoSave())
}

func TestMakeShrinksToZeroValues(t *testing.T) {
	c := counterexample(t, func(t *T) {
		Make[record]().Draw(t, "r")
		t.Fail()
	})
	r := drawn[record](t, c, 0)
	if r.ID != 0 || r.Name != "0" || r.Score != 0 || len(r.Tags) != 0 || len(r.Counts) != 0 || r.Opt != nil || len(r.Children) != 0 {
		t.Errorf("minimal record is %+v", r)
	}
}

func TestMakeOverrides(t *testing.T) {
	Check(t, func(t *T) {
		r := Make[record](WithType(Just("fixed")), WithField[record]("ID", IntRange(7, 7))).Draw(t, "r")
		if r.ID != 7 || r.Name != "fixed" {
			t.Fatalf("overrides not applied: %+v", r)
		}
		for _, tag := range r.Tags {
			if tag != "fixed" {
				t.Fatalf("WithType did not reach the slice elements: %q", tag)
			}
		}
	}, NoSave())
}

func TestMakeRejectsUnsupportedTypes(t *testing.T) {
	defer func() {
		if rec := recover(); rec == nil || !strings.Contains(rec.(string), "cannot derive") {
			t.Errorf("expected a panic about an unsupported type, got %v", rec)
		}
	}()
	Make[struct{ F func() }]()
}

func TestMakeRejectsBadTags(t *testing.T) {
	defer func() {
		if rec := recover(); rec == nil || !strings.Contains(rec.(string), `"abc"`) {
			t.Errorf("expected a panic naming the bad bound, got %v", rec)
		}
	}()
	Make[struct {
		N int `pbt:"min=abc"`
	}]()
}
