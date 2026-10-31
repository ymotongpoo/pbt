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
	c := &T{size: 7, choices: []uint64{0, 1, 2, 18446744073709551615, 5}, draws: []drawRecord{{"xs", "SliceOf(Int())", []int{1, -2}}}}
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
