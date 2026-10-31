// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TB is the part of testing.TB that Check uses. *testing.T, *testing.B and
// *testing.F satisfy it.
type TB interface {
	Helper()
	Name() string
	Logf(format string, args ...any)
	Errorf(format string, args ...any)
}

// ForAll builds a property over one generated value.
func ForAll[A any](g Gen[A], body func(t *T, a A)) Property {
	return func(t *T) { body(t, g.Draw(t, "")) }
}

// ForAll2 builds a property over two generated values.
func ForAll2[A, B any](ga Gen[A], gb Gen[B], body func(t *T, a A, b B)) Property {
	return func(t *T) {
		a := ga.Draw(t, "")
		b := gb.Draw(t, "")
		body(t, a, b)
	}
}

// ForAll3 builds a property over three generated values.
func ForAll3[A, B, C any](ga Gen[A], gb Gen[B], gc Gen[C], body func(t *T, a A, b B, c C)) Property {
	return func(t *T) {
		a := ga.Draw(t, "")
		b := gb.Draw(t, "")
		c := gc.Draw(t, "")
		body(t, a, b, c)
	}
}

var (
	flagChecks     = flag.Int("pbt.checks", defaultChecks, "number of test cases pbt.Check runs per property")
	flagSeed       = flag.Uint64("pbt.seed", 0, "seed of the random number generator used by pbt.Check (0 picks one at random)")
	flagStartSize  = flag.Int("pbt.startsize", defaultStartSize, "size parameter of the first test case")
	flagMaxSize    = flag.Int("pbt.maxsize", defaultMaxSize, "size parameter the test cases grow up to")
	flagShrinkTime = flag.Duration("pbt.shrinktime", defaultShrink, "time budget for shrinking a failing test case")
	flagNoSave     = flag.Bool("pbt.nosave", false, "do not save failing test cases under testdata/pbt")
	flagSearch     = flag.Int("pbt.searchsteps", defaultSearchSteps, "number of steps of a targeted search (a property that calls Maximize or Minimize)")
	flagV          = flag.Bool("pbt.v", false, "verbose output from pbt.Check")
)

// Option adjusts how Check runs a property. Options take precedence over the
// -pbt.* command-line flags.
type Option func(*config)

type config struct {
	checks     int
	seed       uint64
	hasSeed    bool
	startSize  int
	maxSize    int
	shrinkTime time.Duration
	noSave     bool
	testdata   string
	verbose    bool
	setup      func(*T) func()
	checkCover bool
	search     int
	valueLimit int
}

// Checks sets the number of passing test cases Check requires.
func Checks(n int) Option { return func(c *config) { c.checks = n } }

// Seed fixes the seed of the random number generator, which makes the run
// reproducible.
func Seed(seed uint64) Option {
	return func(c *config) {
		c.seed = seed
		c.hasSeed = true
	}
}

// Sizes sets the size parameter of the first test case and the size the
// cases grow up to, as PropEr's start_size and max_size do. At size 0,
// collections are empty and numbers are 0 or 1.
func Sizes(start, max int) Option {
	return func(c *config) {
		c.startSize = start
		c.maxSize = max
	}
}

// ShrinkTime sets the time budget for shrinking a failing test case.
func ShrinkTime(d time.Duration) Option { return func(c *config) { c.shrinkTime = d } }

// NoSave keeps Check from writing failing test cases under testdata/pbt.
func NoSave() Option { return func(c *config) { c.noSave = true } }

// TestdataDir changes the directory under which failing test cases are saved
// and replayed. The default is testdata/pbt relative to the test binary's
// working directory, which go test sets to the package directory.
func TestdataDir(dir string) Option { return func(c *config) { c.testdata = dir } }

// Setup runs f before every test case, as PropEr's ?SETUP does. The function
// f returns may be nil; otherwise it runs after the case, like Cleanup.
func Setup(f func(t *T) (teardown func())) Option { return func(c *config) { c.setup = f } }

// CheckCoverage makes Check fail when a Cover requirement is not met. Without
// it, Check only logs the shortfall.
func CheckCoverage() Option { return func(c *config) { c.checkCover = true } }

// Verbose makes Check log the summary of a passing run and every accepted
// shrinking step, as the -pbt.v flag does.
func Verbose() Option { return func(c *config) { c.verbose = true } }

// SearchSteps sets the number of cases a targeted search runs, as PropEr's
// search_steps does. It applies once the property calls Maximize or Minimize.
func SearchSteps(n int) Option { return func(c *config) { c.search = n } }

// ReportLimit sets how many characters of each drawn value a report shows;
// the default is 2000, and 0 shows everything.
func ReportLimit(chars int) Option { return func(c *config) { c.valueLimit = chars } }

func newConfig(opts []Option) (config, error) {
	c := config{
		checks:     *flagChecks,
		seed:       *flagSeed,
		hasSeed:    *flagSeed != 0,
		startSize:  *flagStartSize,
		maxSize:    *flagMaxSize,
		shrinkTime: *flagShrinkTime,
		noSave:     *flagNoSave,
		testdata:   defaultTestdata,
		verbose:    *flagV,
		search:     *flagSearch,
		valueLimit: defaultValueLimit,
	}
	for _, o := range opts {
		o(&c)
	}
	switch {
	case c.checks < 1:
		return c, fmt.Errorf("Checks(%d): need at least one check", c.checks)
	case c.search < 1:
		return c, fmt.Errorf("SearchSteps(%d): need at least one step", c.search)
	case c.startSize < 0 || c.maxSize < c.startSize:
		return c, fmt.Errorf("Sizes(%d, %d): need 0 <= start <= max", c.startSize, c.maxSize)
	}
	if !c.hasSeed {
		c.seed = randomSeed()
	}
	return c, nil
}

// Check runs prop against generated inputs and reports the minimal failing
// input it finds through tb.
//
// Check first replays the counterexamples saved under testdata/pbt for this
// test and fails if one of them still fails. It then runs the property for
// the configured number of cases with sizes growing from the start size to
// the maximum. On failure it shrinks the input, saves it, and reports every
// value the property drew together with the seed that reproduces the run.
func Check(tb TB, prop Property, opts ...Option) {
	tb.Helper()
	cfg, err := newConfig(opts)
	if err != nil {
		tb.Errorf("[pbt] %v", err)
		return
	}
	r := &runner{tb: tb, prop: prop, cfg: cfg}
	if t, path := r.replaySaved(); t != nil {
		r.report(t, "saved counterexample "+filepath.Base(path)+" still fails", nil)
		return
	}
	fail, passed, err := r.search()
	if err != nil {
		tb.Errorf("[pbt] %v", err)
		return
	}
	if fail == nil {
		r.reportStats(passed)
		return
	}
	minimal, steps := r.shrink(fail)
	var trailer []string
	if !cfg.noSave {
		if path, err := r.save(minimal); err != nil {
			trailer = append(trailer, fmt.Sprintf("could not save the counterexample: %v", err))
		} else {
			trailer = append(trailer, fmt.Sprintf("saved to %s; go test replays it until the file is deleted", path))
		}
	}
	trailer = append(trailer, fmt.Sprintf("reproduce: go test -run '%s' -pbt.seed=0x%x", tb.Name(), cfg.seed))
	r.report(minimal, fmt.Sprintf("property failed after %d passed checks and %d shrink steps", passed, steps), trailer)
}

// MakeCheck turns a property into a test function.
func MakeCheck(prop Property, opts ...Option) func(*testing.T) {
	return func(t *testing.T) {
		t.Helper()
		Check(t, prop, opts...)
	}
}

// CheckExists looks for a test case on which prop holds, as PropEr's
// ?EXISTS does. A case that fails is a rejected candidate; the first case
// that passes is the witness, which CheckExists shrinks and logs. The test
// fails when no witness turns up within the number of checks, or within the
// search steps when the property guides the search with Maximize or
// Minimize. Nothing is saved under testdata/pbt.
func CheckExists(tb TB, prop Property, opts ...Option) {
	tb.Helper()
	cfg, err := newConfig(opts)
	if err != nil {
		tb.Errorf("[pbt] %v", err)
		return
	}
	r := &runner{tb: tb, prop: prop, cfg: cfg, exists: true}
	witness, tried, err := r.search()
	if err != nil {
		tb.Errorf("[pbt] %v", err)
		return
	}
	if witness == nil {
		tb.Errorf("[pbt] no witness found in %d cases (seed 0x%x)", tried, cfg.seed)
		return
	}
	minimal, steps := r.shrink(witness)
	tb.Logf("%s", r.format(minimal, fmt.Sprintf("witness found after %d rejected cases and %d shrink steps (seed 0x%x)", tried, steps, cfg.seed), nil))
}

type runner struct {
	tb      TB
	prop    Property
	cfg     config
	stats   *stats
	exists  bool // look for a passing case instead of a failing one
	best    *T   // best case of a targeted search, for the report
	current *T   // the case a targeted search mutates
}

// want reports whether a case is what the search looks for: a failure for
// Check, a success for CheckExists.
func (r *runner) want(t *T) bool {
	if r.exists {
		return !t.failed
	}
	return t.failed
}

// run executes the property once. prefix is replayed first; rng supplies
// further choices and may be nil, in which case missing choices are 0.
func (r *runner) run(prefix []uint64, size int, rng *rand.Rand) *T {
	t := newT(r.tb.Name(), prefix, size, r.cfg.maxSize, rng)
	prop := r.prop
	if setup := r.cfg.setup; setup != nil {
		prop = func(t *T) {
			if teardown := setup(t); teardown != nil {
				t.Cleanup(teardown)
			}
			r.prop(t)
		}
	}
	t.execute(prop)
	return t
}

// reportStats logs the statistics of a passing run and checks the Cover
// requirements.
func (r *runner) reportStats(passed int) {
	r.tb.Helper()
	st := r.stats
	if st == nil {
		st = newStats()
	}
	if r.cfg.verbose || !st.empty() || r.best != nil {
		lines := []string{fmt.Sprintf("OK, passed %d checks (seed 0x%x)", passed, r.cfg.seed)}
		if r.best != nil {
			lines = append(lines, fmt.Sprintf("best %s = %s over %d search steps", r.best.utilityName, formatNum(r.best.utility), passed))
		}
		lines = append(lines, st.lines()...)
		r.tb.Logf("[pbt] %s", strings.Join(lines, "\n[pbt] "))
	}
	if short := st.shortfalls(); len(short) > 0 {
		msg := "[pbt] insufficient coverage:\n[pbt]   " + strings.Join(short, "\n[pbt]   ")
		if r.cfg.checkCover {
			r.tb.Errorf("%s", msg)
		} else {
			r.tb.Logf("%s (add the CheckCoverage option to fail on this)", msg)
		}
	}
}

// search runs test cases until one is wanted or the budget is spent. Fresh
// cases grow in size by one per case, as in PropEr. Once a case reports a
// utility, the search becomes targeted: it mutates the current case and
// moves to the neighbor when its utility is higher, or with the Metropolis
// probability exp(-loss/(temperature*scale)) when it is lower, where the
// scale is a hundredth of the utility range seen so far. That is PropEr's
// simulated annealing in a scale-free form; the best case ever seen is kept
// apart for the report. A small share of the steps restart from a fresh
// case.
func (r *runner) search() (found *T, passed int, err error) {
	cfg := r.cfg
	r.stats = newStats()
	discards := 0
	targeted := false
	uMin, uMax := math.Inf(1), math.Inf(-1)
	for {
		limit := cfg.checks
		if targeted {
			limit = cfg.search
		}
		if passed >= limit {
			return nil, passed, nil
		}
		rng := newRand(caseSeed(cfg.seed, uint64(passed+discards)))
		temp := 1 - float64(passed)/float64(limit)
		var t *T
		if targeted && r.current != nil && rng.Float64() >= restartProbability {
			t = r.run(mutate(rng, r.current, temp), r.current.size, rng)
		} else {
			t = r.run(nil, min(cfg.startSize+passed, cfg.maxSize), rng)
		}
		switch {
		case t.discarded:
			discards++
			if discards > maxDiscardRatio*max(cfg.checks, limit) {
				return nil, passed, fmt.Errorf("gave up after %d discarded test cases (%d passed); last reason: %s",
					discards, passed, t.discardReason)
			}
		case r.want(t):
			return t, passed, nil
		default:
			r.stats.add(t)
			passed++
			if t.hasUtility {
				targeted = true
				uMin, uMax = min(uMin, t.utility), max(uMax, t.utility)
				scale := (uMax - uMin) / 100
				switch {
				case r.current == nil || t.utility > r.current.utility:
					r.current = t
				case scale > 0 && temp > 0 && rng.Float64() < math.Exp(-(r.current.utility-t.utility)/(scale*temp)):
					r.current = t
				}
				if r.best == nil || t.utility > r.best.utility {
					r.best = t
				}
			}
		}
	}
}
