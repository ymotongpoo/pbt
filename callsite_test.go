// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkTopLevelDraws(b *testing.B) {
	g := Int()
	prop := func(t *T) {
		g.Draw(t, "")
		g.Draw(t, "")
		g.Draw(t, "")
		g.Draw(t, "")
	}
	for b.Loop() {
		t := newT("bench", nil, 20, 42, newRand(1))
		t.execute(prop)
	}
}

func TestDrawCallsite(t *testing.T) {
	pt := newT("", nil, 1, 42, nil)
	n := Int().Draw(pt, "")
	var m = Int().Draw(pt, "")
	n = Int().Draw(pt, "")
	_ = n + m
	Int().Draw(pt, "")
	_ = Int().Draw(pt, "")
	explicit := Int().Draw(pt, "chosen")
	_ = explicit
	for i, name := range []string{"n", "m", "n", "", "", "chosen"} {
		d := pt.draws[i]
		if got := drawName(d); got != name {
			t.Errorf("draw %d name %q, want %q", i, got, name)
		}
		got := formatDraw(d)
		if name == "" {
			if !strings.Contains(got, "Int() at callsite_test.go:") {
				t.Error(got)
			}
		} else if got != name+" (Int()) = 0" {
			t.Error(got)
		}
		if got := formatDrawSite(d, 0, true); !strings.Contains(got, " at callsite_test.go:") {
			t.Error(got)
		}
	}
}

func TestDrawSourceFallback(t *testing.T) {
	if got := sourceName(filepath.Join(t.TempDir(), "missing.go"), 1); got != "" {
		t.Fatal(got)
	}
	file := filepath.Join(t.TempDir(), "bad.go")
	if err := os.WriteFile(file, []byte("not go"), 0600); err != nil {
		t.Fatal(err)
	}
	if got := sourceName(file, 1); got != "" {
		t.Fatal(got)
	}
}

func TestForAllDrawsHaveNoSite(t *testing.T) {
	props := []Property{
		ForAll(Int(), func(*T, int) {}),
		ForAll2(Int(), Int(), func(*T, int, int) {}),
		ForAll3(Int(), Int(), Int(), func(*T, int, int, int) {}),
	}
	for _, prop := range props {
		pt := newT("", nil, 1, 42, nil)
		pt.execute(prop)
		for _, d := range pt.draws {
			if d.pc != 0 || formatDrawSite(d, 0, true) != "Int() = 0" {
				t.Fatalf("%+v", d)
			}
		}
	}
}

func TestCallsiteReport(t *testing.T) {
	tb := &fakeTB{name: "callsite"}
	Check(tb, func(pt *T) {
		n := IntRange(5, 10).Draw(pt, "")
		pt.Fatalf("bad n: %d", n)
	}, Seed(1), NoSave(), NoReplay())
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "n (IntRange(5, 10)) = 5") {
		t.Fatal(tb.errors)
	}
	t.Log(tb.errors[0])
}
