// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestCoveragePool(t *testing.T) {
	value := 0.1
	c := newCoverageSearch(func() float64 { return value })
	input := &T{size: 7, choices: []uint64{10, 20}, metas: []choiceMeta{plainMeta, flagMeta}, spans: []span{{start: 0, end: 2}}}
	c.observe(input)
	if len(c.pool) != 0 {
		t.Fatal("unchanged coverage added a seed")
	}
	value = 0.2
	c.observe(input)
	seed := c.pool[0]
	if seed.size != input.size || !slices.Equal(seed.choices, input.choices) || !slices.Equal(seed.metas, input.metas) || !slices.Equal(seed.spans, input.spans) {
		t.Fatal("pool lost mutation data")
	}
	input.choices[0], input.metas[0], input.spans[0].end = 99, flagMeta, 1
	if seed.choices[0] != 10 || seed.metas[0] != plainMeta || seed.spans[0].end != 2 {
		t.Fatal("pool aliases the completed case")
	}
	for i := range 70 {
		value += 0.01
		input.choices[0] = uint64(i)
		c.observe(input)
	}
	if len(c.pool) != 64 || c.pool[0].choices[0] != 6 || c.pool[63].choices[0] != 69 {
		t.Fatal("pool did not evict in FIFO order")
	}
}

func TestCoverageRejectedCases(t *testing.T) {
	value := 0.1
	c := newCoverageSearch(func() float64 { return value })
	input := &T{choices: []uint64{7}, metas: []choiceMeta{plainMeta}}
	for _, status := range []string{"discard", "failure"} {
		input.discarded, input.failed = status == "discard", status == "failure"
		value += 0.1
		c.observe(input)
		input.discarded, input.failed = false, false
		c.observe(input)
	}
	if len(c.pool) != 0 || c.cases != 2 || c.novel != 0 {
		t.Fatal("coverage from rejected cases was attributed to passing cases")
	}
	value += 0.1
	c.observe(&T{})
	if len(c.pool) != 0 || c.cases != 3 || c.novel != 1 {
		t.Fatal("a case without choices must be counted but cannot be mutated")
	}
}

func TestCoverageSeedScheduling(t *testing.T) {
	value := 0.0
	c := newCoverageSearch(func() float64 { return value })
	rng, untouched := newRand(1), newRand(1)
	for range 5 {
		if c.seed(rng) != nil {
			t.Fatal("empty pool returned a seed")
		}
	}
	if rng.Uint64() != untouched.Uint64() {
		t.Fatal("empty pool consumed randomness")
	}
	for i := range 3 {
		value += 0.1
		c.observe(&T{size: i, choices: []uint64{uint64(i)}, metas: []choiceMeta{plainMeta}})
	}
	seen := make(map[int]bool)
	for range 100 {
		seed := c.seed(rng)
		if seed == nil {
			t.Fatal("expected a mutation seed")
		}
		seen[seed.size] = true
		if c.seed(rng) != nil {
			t.Fatal("expected a fresh attempt after a mutation")
		}
	}
	if len(seen) != 3 {
		t.Fatal("not all pool seeds were selected")
	}
}

func TestCoverageSearchScheduling(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1), Checks(6), CoverageGuided()})
	var cases []*T
	reads := 0
	r := &runner{tb: &fakeTB{}, cfg: cfg}
	r.coverageRead = func() float64 {
		reads++
		if reads == 1 {
			return 0
		}
		return 0.1 // only the first case becomes a seed
	}
	r.prop = func(c *T) {
		Byte().Draw(c, "byte")
		cases = append(cases, c)
		if len(cases) == 2 {
			c.Skip("discard a mutation")
		}
	}
	found, passed, err := r.search()
	if found != nil || err != nil || passed != 6 || len(cases) != 7 || reads != 8 {
		t.Fatalf("found %v, passed %d, attempts %d, reads %d, error %v", found != nil, passed, len(cases), reads, err)
	}
	for i, c := range cases {
		if i%2 == 1 {
			if len(c.prefix) == 0 || c.size != cases[0].size {
				t.Fatalf("attempt %d did not mutate at the seed's size", i)
			}
		} else if c.prefix != nil {
			t.Fatalf("attempt %d was not fresh", i)
		}
	}
	if cases[2].size != 2 || cases[4].size != 4 || cases[6].size != 6 {
		t.Fatal("fresh size progression changed")
	}
}

func TestCoverageTargetedPrecedence(t *testing.T) {
	run := func(guided bool) [][]uint64 {
		cfg, _ := newConfig([]Option{Seed(1), Checks(5), SearchSteps(30)})
		cfg.coverage = guided
		var history [][]uint64
		r := &runner{tb: &fakeTB{}, cfg: cfg}
		value := 0.0
		r.coverageRead = func() float64 { value += 0.01; return value }
		r.prop = func(c *T) {
			x := Byte().Draw(c, "byte")
			// Even if later cases omit the utility, targeted search wins.
			if len(history)%3 == 0 {
				c.Maximize("byte", float64(x))
			}
			history = append(history, slices.Clone(c.choices))
		}
		found, passed, err := r.search()
		if found != nil || err != nil || passed != 30 {
			t.Fatalf("targeted search: found %v, passed %d, error %v", found != nil, passed, err)
		}
		return history
	}
	if !reflect.DeepEqual(run(false), run(true)) {
		t.Fatal("coverage guidance changed the targeted random stream")
	}
}

func TestCoverageUnavailable(t *testing.T) {
	if testing.CoverMode() != "" {
		t.Skip("requires an uninstrumented test binary")
	}
	run := func(guided bool) ([]byte, []string) {
		var history []byte
		tb := &fakeTB{}
		opts := []Option{Seed(1), Checks(50), NoSave(), NoReplay(), Verbose()}
		if guided {
			opts = append(opts, CoverageGuided())
		} else {
			opts = append(opts, func(c *config) { c.coverage = false })
		}
		Check(tb, func(c *T) { history = append(history, Byte().Draw(c, "byte")) }, opts...)
		if len(tb.errors) != 0 {
			t.Fatal(tb.errors)
		}
		return history, tb.logs
	}
	plain, plainLogs := run(false)
	guided, logs := run(true)
	if !slices.Equal(plain, guided) || len(logs) != len(plainLogs)+1 || !slices.Equal(logs[1:], plainLogs) {
		t.Fatalf("fallback changed cases or output: %q versus %q", logs, plainLogs)
	}
	if !strings.Contains(logs[0], "run go test -cover -coverpkg=... to enable it") {
		t.Fatal("missing instrumentation hint")
	}
}

func TestCoverageOptionAndFlag(t *testing.T) {
	saved := *flagCoverage
	t.Cleanup(func() { *flagCoverage = saved })
	for _, enabled := range []bool{false, true} {
		*flagCoverage = enabled
		cfg, err := newConfig([]Option{Seed(1)})
		if err != nil || cfg.coverage != enabled {
			t.Fatalf("flag %t: config %t, error %v", enabled, cfg.coverage, err)
		}
		cfg, err = newConfig([]Option{Seed(1), CoverageGuided()})
		if err != nil || !cfg.coverage {
			t.Fatal("option did not enable coverage")
		}
	}
}

func TestCoverageExistsIgnoresGuidance(t *testing.T) {
	cfg, _ := newConfig([]Option{Seed(1), Checks(3), CoverageGuided()})
	tb := &fakeTB{}
	r := &runner{tb: tb, cfg: cfg, exists: true, prop: func(c *T) { c.Fail() }}
	r.coverageRead = func() float64 { t.Fatal("CheckExists read coverage"); return 0 }
	if found, tried, err := r.search(); found != nil || tried != 3 || err != nil || len(tb.logs) != 0 {
		t.Fatal("coverage guidance affected existential search")
	}
}

func TestCoverageReport(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		for _, fail := range []bool{false, true} {
			cfg, _ := newConfig([]Option{Seed(1), Checks(3), CoverageGuided()})
			cfg.verbose = verbose
			tb := &fakeTB{}
			r := &runner{tb: tb, cfg: cfg}
			value := 0.1
			r.coverageRead = func() float64 { return value }
			r.prop = func(c *T) {
				Byte().Draw(c, "byte")
				value += 0.1
				if fail {
					c.Fail()
				}
			}
			r.search()
			want := "[pbt] coverage-guided: 3 of 3 passing cases reached new code, 3 seeds kept, coverage 10.0% -> 40.0%"
			if fail {
				want = "[pbt] coverage-guided: 0 of 0 passing cases reached new code, 0 seeds kept, coverage 10.0% -> 20.0%"
			}
			if fail && !verbose {
				if len(tb.logs) != 0 {
					t.Fatal("nonverbose failing search logged a summary")
				}
			} else if len(tb.logs) != 1 || tb.logs[0] != want {
				t.Fatalf("summary: %q, want %q", tb.logs, want)
			}
		}
	}
}

func TestCoverageGuidedMagicBytes(t *testing.T) {
	if testing.CoverMode() == "" {
		t.Skip("run go test -cover -run TestCoverageGuidedMagicBytes")
	}
	// Test files are not instrumented. Build an actual package fixture,
	// instrument just the SUT, and give each search a fresh process so that
	// neither earlier tests nor -count can consume its novelty signal.
	binary := filepath.Join(t.TempDir(), "magic.test")
	cmd := exec.CommandContext(t.Context(), "go", "test", "-c", "-cover", "-coverpkg=go.ymotongpoo.com/pbt/testdata/coverage", "-o", binary, "./testdata/coverage")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build coverage fixture: %v\n%s", err, out)
	}
	for _, name := range []string{"TestMagicRandom", "TestMagicGuided"} {
		cmd := exec.CommandContext(t.Context(), binary, "-test.run=^"+name+"$", "-test.v")
		out, err := cmd.CombinedOutput()
		t.Logf("%s:\n%s", name, out)
		if err != nil {
			t.Fatalf("coverage fixture: %v", err)
		}
	}
}
