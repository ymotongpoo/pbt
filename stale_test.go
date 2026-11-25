// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"unsafe"
)

func savedStaleFixture(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	cfg, _ := newConfig([]Option{Seed(1), TestdataDir(dir)})
	r := &runner{tb: &fakeTB{name: "stale"}, cfg: cfg, prop: staleProperty(0, true)}
	path, err := r.save(r.run([]uint64{0}, 1, nil))
	if err != nil {
		t.Fatal(err)
	}
	return dir, path
}

func staleProperty(offset int, fail bool) Property {
	g := Custom("versioned", func(pt *T) int { return IntRange(0, 9).drawSpan(pt) + offset })
	return func(pt *T) {
		g.Draw(pt, "x")
		if fail {
			pt.Error("bad")
		}
	}
}

func TestHealthStaleReplayPaths(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		for _, fails := range []bool{false, true} {
			for _, multiple := range []bool{false, true} {
				t.Run(fmt.Sprintf("explicit=%v/fails=%v/multiple=%v", explicit, fails, multiple), func(t *testing.T) {
					dir, path := savedStaleFixture(t)
					opts := []Option{Seed(1), Checks(1), TestdataDir(dir), NoSave(), WarnHealth(HealthStale)}
					if explicit {
						opts = append(opts, ReplayFile(path), NoReplay())
					}
					if multiple {
						opts = append(opts, MaxFailures(2))
					}
					tb := &fakeTB{name: "stale"}
					Check(tb, staleProperty(1, fails), opts...)
					out := healthOutput(tb)
					for _, want := range []string{fmt.Sprintf("%q", path), "HealthStale", "draw 1", `saved "x (versioned) = 0"`, `replayed now "x (versioned) = 1"`} {
						if !strings.Contains(out, want) {
							t.Fatalf("missing %q: %s", want, out)
						}
					}
					if (len(tb.errors) != 0) != fails || strings.Count(out, "is stale at draw") != 1 {
						t.Fatal(out)
					}
				})
			}
		}
	}
}

func TestHealthStaleOptions(t *testing.T) {
	dir, path := savedStaleFixture(t)
	for _, tc := range []struct {
		name       string
		opts       []Option
		warn, fail bool
	}{
		{"off", nil, false, false},
		{"default", []Option{CheckHealth()}, false, false},
		{"warn", []Option{WarnHealth(HealthStale)}, true, false},
		{"fail", []Option{CheckHealth(HealthStale)}, true, true},
		{"warn-after-check", []Option{CheckHealth(HealthStale), WarnHealth(HealthStale)}, true, true},
		{"ignore", []Option{WarnHealth(HealthStale), CheckHealth(HealthStale), IgnoreHealth(HealthStale)}, false, false},
		{"ignore-first", []Option{IgnoreHealth(HealthStale), CheckHealth(HealthStale), WarnHealth(HealthStale)}, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tb := &fakeTB{name: "stale"}
			Check(tb, staleProperty(1, false), append([]Option{Seed(1), Checks(1), TestdataDir(dir), NoSave()}, tc.opts...)...)
			if out := healthOutput(tb); strings.Contains(out, "HealthStale") != tc.warn || (len(tb.errors) > 0) != tc.fail {
				t.Fatal(out)
			}
		})
	}
	// Replay reads the old v1 format and never rewrites it.
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), persistHeader+"\n") || !strings.Contains(string(data), "# x (versioned) = 0\n") {
		t.Fatal(string(data), err)
	}
}

func TestStaleFirstDifferentDrawAndCounts(t *testing.T) {
	draw := func(n int) drawRecord { return drawRecord{label: "x", gen: "Int()", value: n} }
	old := &T{size: 1, draws: []drawRecord{draw(1), draw(2), draw(3)}}
	data := encodeCase(old)
	for _, tc := range []struct {
		draws []drawRecord
		want  string
	}{
		{[]drawRecord{draw(1), draw(20), draw(30)}, "draw 2"},
		{[]drawRecord{draw(1)}, `replayed now "(no draw)"`},
		{append(append([]drawRecord(nil), old.draws...), draw(4)), `saved "(no draw)"`},
	} {
		note := staleDrawNote("saved", data, &T{draws: tc.draws})
		if note == nil || !strings.Contains(note.text, tc.want) {
			t.Fatal(note)
		}
	}
	if note := staleDrawNote("saved", data, old); note != nil {
		t.Fatal(note)
	}
}

type staleGoString struct{}

func (staleGoString) GoString() string { return "unstable" }

type staleFormatter int

func (staleFormatter) Format(s fmt.State, verb rune) { fmt.Fprint(s, "unstable") }

func TestStaleUnstableRenderings(t *testing.T) {
	n := 1
	cyclic := map[string]any{}
	cyclic["self"] = cyclic
	for _, v := range []any{
		&n, unsafe.Pointer(&n), func() {}, make(chan int),
		[]any{&n}, map[string]any{"p": &n}, struct{ private *int }{&n},
		staleGoString{}, staleFormatter(1), cyclic, map[float64]int{math.NaN(): 1, math.NaN(): 2},
	} {
		if stableDrawValue(reflect.ValueOf(v), 0) {
			t.Fatalf("unstable %T accepted", v)
		}
		// An old address must not cause a warning, even when the text differs.
		data := []byte(persistHeader + "\nsize 1\n# x = old address\nchoices\n")
		if note := staleDrawNote("saved", data, &T{draws: []drawRecord{{label: "x", value: v}}}); note != nil {
			t.Fatal(note)
		}
	}
	// Skipping one unstable draw still checks the later stable draws.
	old := &T{draws: []drawRecord{{value: &n}, {value: 1}}}
	now := &T{draws: []drawRecord{{value: &n}, {value: 2}}}
	if note := staleDrawNote("saved", encodeCase(old), now); note == nil || !strings.Contains(note.text, "draw 2") {
		t.Fatal(note)
	}
}

func TestStaleMapsAndTruncation(t *testing.T) {
	old := &T{draws: []drawRecord{{label: "m", value: map[string]int{"b": 2, "a": 1}}}}
	now := &T{draws: []drawRecord{{label: "m", value: map[string]int{"a": 1, "b": 2}}}}
	if note := staleDrawNote("saved", encodeCase(old), now); note != nil {
		t.Fatal(note)
	}
	now.draws[0].value = map[string]int{"a": 1, "b": 3}
	if note := staleDrawNote("saved", encodeCase(old), now); note == nil {
		t.Fatal("changed map not detected")
	}
	long := strings.Repeat("あ", defaultValueLimit)
	old.draws = []drawRecord{{label: "s", value: long + "a"}}
	data := encodeCase(old)
	if !strings.Contains(string(data), "more characters; raise the limit with ReportLimit") {
		t.Fatal("expected the historical fixed truncation")
	}
	now.draws = []drawRecord{{label: "s", value: long + "b"}}
	// The saved prefix and original length cannot distinguish equal-length tails.
	if note := staleDrawNote("saved", data, now); note != nil {
		t.Fatal(note)
	}
	now.draws[0].value = "changed" + long
	if note := staleDrawNote("saved", data, now); note == nil {
		t.Fatal("changed visible prefix not detected")
	}
	for _, limit := range []int{0, 1, 10000} {
		cfg, _ := newConfig([]Option{Seed(1), ReportLimit(limit), WarnHealth(HealthStale)})
		r := &runner{tb: &fakeTB{}, cfg: cfg, prop: func(pt *T) {
			Custom("long", func(*T) string { return long }).Draw(pt, "s")
		}}
		original := r.run(nil, 1, nil)
		r.checkStale("saved", encodeCase(original), r.run(original.choices, original.size, nil))
		if len(r.healthExtra) != 0 {
			t.Fatal(r.healthExtra)
		}
	}
}

func TestStaleSurvivesGenerationError(t *testing.T) {
	for _, maxFailures := range []int{1, 2} {
		dir, _ := savedStaleFixture(t)
		calls := 0
		tb := &fakeTB{name: "stale"}
		Check(tb, func(pt *T) {
			staleProperty(1, false)(pt)
			calls++
			if calls > 1 {
				pt.Skip("no generated cases")
			}
		}, Seed(1), Checks(1), NoSave(), TestdataDir(dir), MaxFailures(maxFailures), WarnHealth(HealthStale))
		out := healthOutput(tb)
		if !strings.Contains(out, "gave up") || !strings.Contains(out, "HealthStale") {
			t.Fatal(out)
		}
	}
}

func TestHealthStaleFlags(t *testing.T) {
	warn, check, ignore := *flagWarnHealth, *flagCheckHealth, *flagIgnoreHealth
	defer func() { *flagWarnHealth, *flagCheckHealth, *flagIgnoreHealth = warn, check, ignore }()
	*flagWarnHealth, *flagCheckHealth, *flagIgnoreHealth = "HealthStale", "stale", ""
	cfg, err := newConfig([]Option{Seed(1)})
	if err != nil || !cfg.healthEnabled(HealthStale) || !cfg.checkHealth[HealthStale] {
		t.Fatal(cfg, err)
	}
	*flagIgnoreHealth = "STALE"
	cfg, err = newConfig([]Option{Seed(1)})
	if err != nil || cfg.healthEnabled(HealthStale) {
		t.Fatal(cfg, err)
	}
	*flagWarnHealth = "unknown"
	if _, err := newConfig([]Option{Seed(1)}); err == nil || !strings.Contains(err.Error(), "-pbt.warnhealth") {
		t.Fatal(err)
	}
}

func TestStaleExplicitAndAutomaticReplayShareNote(t *testing.T) {
	dir, path := savedStaleFixture(t)
	tb := &fakeTB{name: "stale"}
	Check(tb, staleProperty(1, true), Seed(1), NoSave(), TestdataDir(dir), ReplayFile(path), MaxFailures(2), WarnHealth(HealthStale))
	if out := healthOutput(tb); len(tb.errors) != 1 || strings.Count(out, "is stale at draw") != 1 {
		t.Fatal(out)
	}
}

func TestStaleWhitespaceAndEmptyDraws(t *testing.T) {
	c := &T{draws: []drawRecord{{label: "a\nb ", gen: "custom\n", value: "text"}}}
	if note := staleDrawNote("saved", encodeCase(c), c); note != nil {
		t.Fatal(note)
	}
	if note := staleDrawNote("saved", encodeCase(&T{}), &T{}); note != nil {
		t.Fatal(note)
	}
}
