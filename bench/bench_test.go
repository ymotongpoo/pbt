// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Package bench compares pbt with rapid on the same properties: generation
// throughput as benchmarks, and shrinking quality and time as a test that
// prints a table. Run it with:
//
//	go test -bench . -run TestCompareShrinking -v ./...
package bench

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"go.ymotongpoo.com/pbt"
	"pgregory.net/rapid"
)

// recorder satisfies pbt.TB and rapid.TB and keeps the reports.
type recorder struct {
	name   string
	logs   []string
	errors []string
}

func (r *recorder) Helper()         {}
func (r *recorder) Name() string    { return r.name }
func (r *recorder) Log(args ...any) { r.logs = append(r.logs, fmt.Sprint(args...)) }
func (r *recorder) Logf(format string, args ...any) {
	r.logs = append(r.logs, fmt.Sprintf(format, args...))
}
func (r *recorder) Error(args ...any) { r.errors = append(r.errors, fmt.Sprint(args...)) }
func (r *recorder) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}
func (r *recorder) Fatal(args ...any)                 { r.Error(args...) }
func (r *recorder) Fatalf(format string, args ...any) { r.Errorf(format, args...) }
func (r *recorder) FailNow()                          {}
func (r *recorder) Fail()                             {}
func (r *recorder) Failed() bool                      { return len(r.errors) > 0 }
func (r *recorder) Skip(args ...any)                  {}
func (r *recorder) Skipf(format string, args ...any)  {}
func (r *recorder) SkipNow()                          {}

type point struct {
	X, Y int
	Tags []string
}

// Generation throughput: one Check of 100 passing cases per iteration.

func BenchmarkPBTIntSlice(b *testing.B) {
	for b.Loop() {
		pbt.Check(&recorder{name: "b"}, func(t *pbt.T) { pbt.SliceOf(pbt.Int()).Draw(t, "xs") }, pbt.NoSave(), pbt.Checks(100))
	}
}

func BenchmarkRapidIntSlice(b *testing.B) {
	for b.Loop() {
		rapid.Check(&recorder{name: "b"}, func(t *rapid.T) { rapid.SliceOf(rapid.Int()).Draw(t, "xs") })
	}
}

func BenchmarkPBTStrings(b *testing.B) {
	for b.Loop() {
		pbt.Check(&recorder{name: "b"}, func(t *pbt.T) { pbt.SliceOf(pbt.String()).Draw(t, "ss") }, pbt.NoSave(), pbt.Checks(100))
	}
}

func BenchmarkRapidStrings(b *testing.B) {
	for b.Loop() {
		rapid.Check(&recorder{name: "b"}, func(t *rapid.T) { rapid.SliceOf(rapid.String()).Draw(t, "ss") })
	}
}

func BenchmarkPBTMap(b *testing.B) {
	for b.Loop() {
		pbt.Check(&recorder{name: "b"}, func(t *pbt.T) { pbt.MapOf(pbt.String(), pbt.Int()).Draw(t, "m") }, pbt.NoSave(), pbt.Checks(100))
	}
}

func BenchmarkRapidMap(b *testing.B) {
	for b.Loop() {
		rapid.Check(&recorder{name: "b"}, func(t *rapid.T) { rapid.MapOf(rapid.String(), rapid.Int()).Draw(t, "m") })
	}
}

func BenchmarkPBTMakeStruct(b *testing.B) {
	g := pbt.Make[point]()
	for b.Loop() {
		pbt.Check(&recorder{name: "b"}, func(t *pbt.T) { g.Draw(t, "p") }, pbt.NoSave(), pbt.Checks(100))
	}
}

func BenchmarkRapidMakeStruct(b *testing.B) {
	g := rapid.Make[point]()
	for b.Loop() {
		rapid.Check(&recorder{name: "b"}, func(t *rapid.T) { g.Draw(t, "p") })
	}
}

// The same generators with pbt's size parameter capped at 6, which brings
// the amount of generated data close to rapid's default.

func BenchmarkPBTIntSliceSmall(b *testing.B) {
	for b.Loop() {
		pbt.Check(&recorder{name: "b"}, func(t *pbt.T) { pbt.SliceOf(pbt.Int()).Draw(t, "xs") }, pbt.NoSave(), pbt.Checks(100), pbt.Sizes(1, 6))
	}
}

func BenchmarkPBTStringsSmall(b *testing.B) {
	for b.Loop() {
		pbt.Check(&recorder{name: "b"}, func(t *pbt.T) { pbt.SliceOf(pbt.String()).Draw(t, "ss") }, pbt.NoSave(), pbt.Checks(100), pbt.Sizes(1, 6))
	}
}

// TestVolume prints how much data each library generates in 100 cases,
// which is what the throughput benchmarks must be read against.
func TestVolume(t *testing.T) {
	count := func(name string, run func(add func(strings, runes int))) {
		strings, runes := 0, 0
		run(func(s, r int) { strings += s; runes += r })
		t.Logf("%-28s %6d strings %8d runes", name, strings, runes)
	}
	count("pbt default sizes", func(add func(int, int)) {
		pbt.Check(&recorder{name: "v"}, func(t *pbt.T) {
			for _, s := range pbt.SliceOf(pbt.String()).Draw(t, "ss") {
				add(1, len([]rune(s)))
			}
		}, pbt.NoSave(), pbt.Checks(100), pbt.Seed(1))
	})
	count("pbt Sizes(1, 6)", func(add func(int, int)) {
		pbt.Check(&recorder{name: "v"}, func(t *pbt.T) {
			for _, s := range pbt.SliceOf(pbt.String()).Draw(t, "ss") {
				add(1, len([]rune(s)))
			}
		}, pbt.NoSave(), pbt.Checks(100), pbt.Seed(1), pbt.Sizes(1, 6))
	})
	count("rapid", func(add func(int, int)) {
		rapid.Check(&recorder{name: "v"}, func(t *rapid.T) {
			for _, s := range rapid.SliceOf(rapid.String()).Draw(t, "ss") {
				add(1, len([]rune(s)))
			}
		})
	})
}

// Shrinking: the same failing properties under both libraries. The table
// shows the reported minimal values and the wall time of the whole Check,
// which includes the search for the first failure.

type comparison struct {
	name  string
	pbt   pbt.Property
	rapid func(*rapid.T)
}

var comparisons = []comparison{
	{"int greater than a million",
		func(t *pbt.T) {
			if x := pbt.Int().Draw(t, "x"); x > 1000000 {
				t.Fatalf("x")
			}
		},
		func(t *rapid.T) {
			if x := rapid.Int().Draw(t, "x"); x > 1000000 {
				t.Fatalf("x")
			}
		}},
	{"two elements above a million",
		func(t *pbt.T) {
			xs := pbt.SliceOf(pbt.Int()).Draw(t, "xs")
			if countAbove(xs) > 1 {
				t.Fatalf("xs")
			}
		},
		func(t *rapid.T) {
			xs := rapid.SliceOf(rapid.Int()).Draw(t, "xs")
			if countAbove(xs) > 1 {
				t.Fatalf("xs")
			}
		}},
	{"length 1 mod 3 with a large last element",
		func(t *pbt.T) {
			xs := pbt.SliceOfN(pbt.Int(), 4, -1).Draw(t, "xs")
			if len(xs)%3 == 1 && xs[len(xs)-1] >= 100 {
				t.Fatalf("xs")
			}
		},
		func(t *rapid.T) {
			xs := rapid.SliceOfN(rapid.Int(), 4, -1).Draw(t, "xs")
			if len(xs)%3 == 1 && xs[len(xs)-1] >= 100 {
				t.Fatalf("xs")
			}
		}},
	{"five distinct positive elements",
		func(t *pbt.T) {
			xs := pbt.SliceOfN(pbt.IntMin(1), 5, -1).Draw(t, "xs")
			if distinct(xs) {
				t.Fatalf("xs")
			}
		},
		func(t *rapid.T) {
			xs := rapid.SliceOfN(rapid.IntMin(1), 5, -1).Draw(t, "xs")
			if distinct(xs) {
				t.Fatalf("xs")
			}
		}},
	{"first string longer than the second",
		func(t *pbt.T) {
			s1, s2 := pbt.String().Draw(t, "s1"), pbt.String().Draw(t, "s2")
			if len(s1) > len(s2) {
				t.Fatalf("s")
			}
		},
		func(t *rapid.T) {
			s1, s2 := rapid.String().Draw(t, "s1"), rapid.String().Draw(t, "s2")
			if len(s1) > len(s2) {
				t.Fatalf("s")
			}
		}},
	{"struct with a tag and X above Y",
		func(t *pbt.T) {
			p := pbt.Make[point]().Draw(t, "p")
			if len(p.Tags) > 0 && p.X > p.Y {
				t.Fatalf("p")
			}
		},
		func(t *rapid.T) {
			p := rapid.Make[point]().Draw(t, "p")
			if len(p.Tags) > 0 && p.X > p.Y {
				t.Fatalf("p")
			}
		}},
	{"map with three entries",
		func(t *pbt.T) {
			if m := pbt.MapOf(pbt.String(), pbt.Int()).Draw(t, "m"); len(m) >= 3 {
				t.Fatalf("m")
			}
		},
		func(t *rapid.T) {
			if m := rapid.MapOf(rapid.String(), rapid.Int()).Draw(t, "m"); len(m) >= 3 {
				t.Fatalf("m")
			}
		}},
}

func countAbove(xs []int) int {
	n := 0
	for _, x := range xs {
		if x > 1000000 {
			n++
		}
	}
	return n
}

func distinct(xs []int) bool {
	ys := slices.Clone(xs)
	slices.Sort(ys)
	return slices.Equal(ys, slices.Compact(slices.Clone(ys)))
}

func TestCompareShrinking(t *testing.T) {
	var b strings.Builder
	fmt.Fprintf(&b, "\n%-44s | %-40s | %8s | %-40s | %8s\n", "property", "pbt minimum", "pbt", "rapid minimum", "rapid")
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 150))
	for _, c := range comparisons {
		rp := &recorder{name: "pbt"}
		start := time.Now()
		pbt.Check(rp, c.pbt, pbt.NoSave(), pbt.Checks(100))
		pbtTime := time.Since(start)
		rr := &recorder{name: "rapid"}
		start = time.Now()
		rapid.Check(rr, c.rapid)
		rapidTime := time.Since(start)
		fmt.Fprintf(&b, "%-44s | %-40s | %8v | %-40s | %8v\n", c.name,
			trim(values(rp.errors, " = ", "[pbt] "), 40), pbtTime.Round(time.Millisecond),
			trim(values(rr.logs, "] draw ", "[rapid] draw "), 40), rapidTime.Round(time.Millisecond))
	}
	t.Log(b.String())
}

// values extracts the drawn values from a report.
func values(report []string, marker, prefix string) string {
	var out []string
	for _, line := range strings.Split(strings.Join(report, "\n"), "\n") {
		if strings.Contains(line, marker) && !strings.Contains(line, "error:") && !strings.Contains(line, "failed after") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), prefix)))
		}
	}
	return strings.Join(out, "; ")
}

func trim(s string, n int) string {
	if len(s) > n {
		return s[:n-1] + "…"
	}
	return s
}
