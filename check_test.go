// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

type fakeTB struct {
	name   string
	logs   []string
	errors []string
}

func (f *fakeTB) Helper()      {}
func (f *fakeTB) Name() string { return f.name }
func (f *fakeTB) Logf(format string, args ...any) {
	f.logs = append(f.logs, fmt.Sprintf(format, args...))
}
func (f *fakeTB) Errorf(format string, args ...any) {
	f.errors = append(f.errors, fmt.Sprintf(format, args...))
}

func TestCheckReportsSavesAndReplays(t *testing.T) {
	dir := t.TempDir()
	prop := func(t *T) {
		xs := SliceOf(Int()).Draw(t, "xs")
		t.Logf("drew %d elements", len(xs))
		if len(xs) >= 2 {
			t.Fatalf("too long: %d", len(xs))
		}
	}

	tb := &fakeTB{name: "TestSample/case_1"}
	Check(tb, prop, Seed(1), TestdataDir(dir))
	if len(tb.errors) != 1 {
		t.Fatalf("got %d errors, want 1: %q", len(tb.errors), tb.errors)
	}
	msg := tb.errors[0]
	for _, want := range []string{
		"property failed after",
		"xs (SliceOf(Int())) = []int{0, 0}",
		"log: drew 2 elements",
		"error: too long: 2",
		"saved to " + filepath.Join(dir, "TestSample", "case_1"),
		"-pbt.seed=0x1",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("report lacks %q:\n%s", want, msg)
		}
	}
	entries, err := os.ReadDir(filepath.Join(dir, "TestSample", "case_1"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("saved files: %v, %v", entries, err)
	}

	// A later run with another seed fails through the saved case before searching.
	tb2 := &fakeTB{name: "TestSample/case_1"}
	Check(tb2, prop, Seed(2), TestdataDir(dir))
	if len(tb2.errors) != 1 || !strings.Contains(tb2.errors[0], "still fails") {
		t.Errorf("replay did not report the saved case: %q", tb2.errors)
	}

	// Once the property holds, the saved case replays silently.
	tb3 := &fakeTB{name: "TestSample/case_1"}
	Check(tb3, func(t *T) { SliceOf(Int()).Draw(t, "xs") }, Seed(3), TestdataDir(dir))
	if len(tb3.errors) != 0 {
		t.Errorf("unexpected errors: %q", tb3.errors)
	}
}

func TestCheckPasses(t *testing.T) {
	Check(t, func(t *T) {
		xs := SliceOf(Int()).Draw(t, "xs")
		ys := slices.Clone(xs)
		slices.Sort(ys)
		if len(ys) != len(xs) {
			t.Fatalf("sorting changed the length")
		}
		for _, x := range xs {
			if !slices.Contains(ys, x) {
				t.Fatalf("%d went missing", x)
			}
		}
	}, NoSave())
}

func TestCheckRejectsBadOptions(t *testing.T) {
	tb := &fakeTB{name: "TestBad"}
	Check(tb, func(*T) {}, Checks(0), NoSave())
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "Checks(0)") {
		t.Errorf("got %q", tb.errors)
	}
}

func TestEncodeDecodeRoundTrip(t *testing.T) {
	c := &T{size: 7, choices: []uint64{0, 1, 2, 18446744073709551615, 5}, draws: []drawRecord{{label: "xs", gen: "SliceOf(Int())", value: []int{1, -2}}}}
	for i := range 40 {
		c.choices = append(c.choices, uint64(i))
	}
	size, choices, err := decodeCase(encodeCase(c))
	if err != nil {
		t.Fatal(err)
	}
	if size != 7 || !slices.Equal(choices, c.choices) {
		t.Errorf("decoded size %d choices %v", size, choices)
	}
	if _, _, err := decodeCase([]byte("something else\n")); err == nil {
		t.Error("decoding garbage succeeded")
	}
}

func TestSanitizeName(t *testing.T) {
	got := sanitizeName("TestFoo/sub test#1/..")
	want := filepath.Join("TestFoo", "sub_test_1", "_..")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExampleProducesValues(t *testing.T) {
	g := SliceOfN(IntRange(0, 9), 3, 3)
	if xs := g.Example(1); len(xs) != 3 {
		t.Errorf("Example() = %v", xs)
	}
	if a, b := g.Example(5), g.Example(5); !slices.Equal(a, b) {
		t.Errorf("Example with the same seed gave %v and %v", a, b)
	}
}

// nestedPair draws labeled values inside a generator, with an unlabeled
// generator in between, to exercise the nesting of the report.
var nestedPair = Custom("pair", func(t *T) [2]int {
	a := IntRange(0, 5).Draw(t, "a")
	inner := Custom("inner", func(t *T) int { return IntRange(0, 5).Draw(t, "deep") })
	b := Map(inner, func(x int) int { return x }).Draw(t, "b")
	return [2]int{a, b}
})

func TestReportNestsLabeledDrawsUnderTheirParent(t *testing.T) {
	prop := func(t *T) {
		nestedPair.Draw(t, "pair")
		SliceOfN(nestedPair, 1, 1).Draw(t, "")
		t.Fatalf("boom")
	}
	report := func(opts ...Option) string {
		tb := &fakeTB{name: "TestNested"}
		Check(tb, prop, append([]Option{NoSave(), Seed(1)}, opts...)...)
		if len(tb.errors) != 1 {
			t.Fatalf("errors %q", tb.errors)
		}
		return tb.errors[0]
	}
	full := report()
	want := []string{
		"[pbt] pair (pair) = [2]int{0, 0}",
		"[pbt]   a (IntRange(0, 5)) = 0",
		"[pbt]   b (Map(inner)) = 0",
		"[pbt]     deep (IntRange(0, 5)) = 0",
		"[pbt] SliceOfN(pair, 1, 1) = [][2]int{[2]int{0, 0}}",
		"[pbt]   a (IntRange(0, 5)) = 0",
		"[pbt]   b (Map(inner)) = 0",
		"[pbt]     deep (IntRange(0, 5)) = 0",
		"[pbt] error: boom",
	}
	if got := strings.Join(want, "\n"); !strings.Contains(full, got) {
		t.Errorf("report:\n%s\nwant the lines:\n%s", full, got)
	}
	shallow := report(ReportDepth(1))
	if strings.Contains(shallow, "deep (IntRange") || !strings.Contains(shallow, "(2 labeled draws nested deeper than 1 levels omitted; raise the depth with ReportDepth)") {
		t.Errorf("ReportDepth(1):\n%s", shallow)
	}
	top := report(ReportDepth(0))
	if strings.Contains(top, " a (") || !strings.Contains(top, "[pbt] pair (pair) = [2]int{0, 0}\n[pbt] SliceOfN") {
		t.Errorf("ReportDepth(0):\n%s", top)
	}
	if _, err := newConfig([]Option{ReportDepth(-1)}); err == nil {
		t.Error("ReportDepth(-1) was accepted")
	}
}

func TestReportMarksADrawThatDidNotFinish(t *testing.T) {
	tb := &fakeTB{name: "TestUndrawn"}
	Check(tb, func(t *T) {
		Custom("bad", func(t *T) int {
			Int().Draw(t, "inner")
			t.Fatalf("inside the generator")
			return 0
		}).Draw(t, "v")
	}, NoSave(), Seed(1))
	if len(tb.errors) != 1 || !strings.Contains(tb.errors[0], "[pbt] v (bad) = (the draw did not finish)\n[pbt]   inner (Int()) = ") {
		t.Errorf("report:\n%s", strings.Join(tb.errors, "\n"))
	}
}

func TestReplayFileAndNoReplay(t *testing.T) {
	dir := t.TempDir()
	failing := func(t *T) {
		if xs := SliceOf(Int()).Draw(t, "xs"); len(xs) >= 2 {
			t.Fatalf("too long")
		}
	}
	tb := &fakeTB{name: "TestA"}
	Check(tb, failing, Seed(1), TestdataDir(dir))
	entries, err := os.ReadDir(filepath.Join(dir, "TestA"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("saved files: %v, %v", entries, err)
	}
	path := filepath.Join(dir, "TestA", entries[0].Name())

	// Another test replays the file by name and reports it when it fails.
	tb2 := &fakeTB{name: "TestB"}
	Check(tb2, failing, Seed(1), TestdataDir(dir), ReplayFile(path))
	if len(tb2.errors) != 1 || !strings.Contains(tb2.errors[0], "replayed "+path+", which still fails") ||
		!strings.Contains(tb2.errors[0], "xs (SliceOf(Int())) = []int{0, 0}") {
		t.Errorf("errors %q", tb2.errors)
	}
	// When the property passes on the file, the run goes on as usual.
	tb3 := &fakeTB{name: "TestB"}
	runs := 0
	Check(tb3, func(t *T) { runs++; SliceOf(Int()).Draw(t, "xs") }, Seed(1), TestdataDir(dir), ReplayFile(path), Checks(5))
	if len(tb3.errors) != 0 || runs != 6 || !strings.Contains(strings.Join(tb3.logs, "\n"), "replayed "+path+": the property passes on it") {
		t.Errorf("errors %q, runs %d, logs %q", tb3.errors, runs, tb3.logs)
	}
	// A file that cannot be read is an error, unlike a stray file in testdata.
	tb4 := &fakeTB{name: "TestB"}
	Check(tb4, failing, Seed(1), TestdataDir(dir), ReplayFile(filepath.Join(dir, "missing")))
	if len(tb4.errors) != 1 || !strings.Contains(tb4.errors[0], "cannot replay") {
		t.Errorf("errors %q", tb4.errors)
	}

	// NoReplay skips the saved cases of the test itself.
	runs = 0
	tb5 := &fakeTB{name: "TestA"}
	Check(tb5, func(t *T) { runs++; SliceOf(Int()).Draw(t, "xs") }, Seed(1), TestdataDir(dir), Checks(5))
	if runs != 6 {
		t.Errorf("without NoReplay the property ran %d times, want 6", runs)
	}
	runs = 0
	tb6 := &fakeTB{name: "TestA"}
	Check(tb6, func(t *T) { runs++; SliceOf(Int()).Draw(t, "xs") }, Seed(1), TestdataDir(dir), Checks(5), NoReplay())
	if runs != 5 || len(tb5.errors)+len(tb6.errors) != 0 {
		t.Errorf("with NoReplay the property ran %d times, want 5; errors %q %q", runs, tb5.errors, tb6.errors)
	}
}
