// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"flag"
	"fmt"
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
	return func(t *T) { body(t, drawForAll(t, g)) }
}

// ForAll2 builds a property over two generated values.
func ForAll2[A, B any](ga Gen[A], gb Gen[B], body func(t *T, a A, b B)) Property {
	return func(t *T) {
		a := drawForAll(t, ga)
		b := drawForAll(t, gb)
		body(t, a, b)
	}
}

// ForAll3 builds a property over three generated values.
func ForAll3[A, B, C any](ga Gen[A], gb Gen[B], gc Gen[C], body func(t *T, a A, b B, c C)) Property {
	return func(t *T) {
		a := drawForAll(t, ga)
		b := drawForAll(t, gb)
		c := drawForAll(t, gc)
		body(t, a, b, c)
	}
}

// drawForAll has no user draw site, including when the wrapper is inlined.
func drawForAll[V any](t *T, g Gen[V]) V {
	saved := t.omitDrawSite
	t.omitDrawSite = true
	defer func() { t.omitDrawSite = saved }()
	return g.Draw(t, "")
}

var (
	flagCausality    = flag.Int("pbt.causality", 0, "continuations per prefix for causality analysis of the first failure (0 disables)")
	flagFailures     = flag.Int("pbt.failures", 1, "maximum number of distinct failures pbt.Check reports")
	flagChecks       = flag.Int("pbt.checks", defaultChecks, "number of test cases pbt.Check runs per property")
	flagSeed         = flag.Uint64("pbt.seed", 0, "seed of the random number generator used by pbt.Check (0 picks one at random)")
	flagStartSize    = flag.Int("pbt.startsize", defaultStartSize, "size parameter of the first test case")
	flagMaxSize      = flag.Int("pbt.maxsize", defaultMaxSize, "size parameter the test cases grow up to")
	flagShrinkTime   = flag.Duration("pbt.shrinktime", defaultShrink, "time budget for shrinking a failing test case")
	flagNoSave       = flag.Bool("pbt.nosave", false, "do not save failing test cases under testdata/pbt")
	flagSearch       = flag.Int("pbt.searchsteps", defaultSearchSteps, "number of steps of a targeted search (a property that calls Maximize or Minimize)")
	flagCoverage     = flag.Bool("pbt.coverage", false, "guide pbt.Check with live block coverage (requires go test -cover)")
	flagV            = flag.Bool("pbt.v", false, "verbose output from pbt.Check")
	flagReplay       = flag.String("pbt.replay", "", "path of a saved test case that pbt.Check replays before searching")
	flagWarnHealth   = flag.String("pbt.warnhealth", "", "comma-separated optional health checks to enable as warnings (duplicate; the others warn by default)")
	flagCheckHealth  = flag.String("pbt.checkhealth", "", "comma-separated health checks that fail a passing run (filter, large, slow, flaky, shrink, stale, duplicate, or default for filter and large)")
	flagIgnoreHealth = flag.String("pbt.ignorehealth", "", "comma-separated health checks to silence (filter, large, slow, flaky, shrink, stale, duplicate)")
)

// Option adjusts how Check runs a property. Options take precedence over the
// -pbt.* command-line flags.
type Option func(*config)

type config struct {
	causality    int
	maxFailures  int
	checks       int
	seed         uint64
	hasSeed      bool
	startSize    int
	maxSize      int
	observe      string
	reportJSON   string
	budget       time.Duration
	shrinkTime   time.Duration
	noSave       bool
	testdata     string
	verbose      bool
	setup        func(*T) func()
	checkCover   bool
	coverage     bool
	search       int
	valueLimit   int
	reportDepth  int
	replayFile   string
	noReplay     bool
	warnHealth   map[HealthCheck]bool
	checkHealth  map[HealthCheck]bool
	ignoreHealth map[HealthCheck]bool
}

// Checks sets the number of passing test cases Check requires.
func Checks(n int) Option { return func(c *config) { c.checks = n } }

// MaxFailures sets the maximum number of distinct failure identities Check
// reports. The default is 1. With a larger limit, failing cases also count
// toward the case budget; shrinking may discover further identities.
// CheckExists ignores this option.
func MaxFailures(n int) Option { return func(c *config) { c.maxFailures = n } }

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

// ShrinkTime sets the time budget for shrinking each failing test case.
// With MaxFailures, each distinct failure gets this full budget.
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

// CheckCoverage makes Check fail when a Cover requirement is not met, as
// QuickCheck's checkCoverage does. Check then keeps running cases past the
// configured number, up to coverCases times as many, until the observed
// share of every label is decided with 99.7% confidence, and fails when a
// share falls short or is still undecided at that point. Without the option
// Check only logs a shortfall.
// A 100% requirement is checked directly: every passing case must hit the
// label, and one miss makes the requirement unmet.
func CheckCoverage() Option { return func(c *config) { c.checkCover = true } }

// coverCases bounds how many times the configured number of checks a run
// under CheckCoverage may grow to while a Cover requirement is undecided.
const coverCases = 20

// ReportDepth sets how many levels of nested labeled draws a report lists
// under each top-level draw; the default is 3, and 0 lists the top-level
// draws only.
func ReportDepth(levels int) Option { return func(c *config) { c.reportDepth = levels } }

// ReplayFile makes Check run the saved test case at path before searching,
// whatever test it was saved by, and report it when it still fails. The
// -pbt.replay flag does the same for every Check of a run. The file is one
// that Check saved under testdata/pbt.
func ReplayFile(path string) Option { return func(c *config) { c.replayFile = path } }

// NoReplay keeps Check from replaying the test cases saved under testdata/pbt
// before searching.
func NoReplay() Option { return func(c *config) { c.noReplay = true } }

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
		causality:   *flagCausality,
		maxFailures: *flagFailures,
		checks:      *flagChecks,
		seed:        *flagSeed,
		hasSeed:     *flagSeed != 0,
		startSize:   *flagStartSize,
		maxSize:     *flagMaxSize,
		observe:     *flagObserve,
		reportJSON:  *flagJSON,
		budget:      *flagBudget,
		shrinkTime:  *flagShrinkTime,
		noSave:      *flagNoSave,
		testdata:    defaultTestdata,
		verbose:     *flagV,
		search:      *flagSearch,
		coverage:    *flagCoverage,
		valueLimit:  defaultValueLimit,
		reportDepth: defaultReportDepth,
		replayFile:  *flagReplay,
	}
	var flagErr error
	if *flagWarnHealth != "" {
		checks, err := parseHealthChecks(*flagWarnHealth, false)
		if err != nil {
			flagErr = fmt.Errorf("-pbt.warnhealth: %v", err)
		} else {
			WarnHealth(checks...)(&c)
		}
	}
	if *flagCheckHealth != "" {
		checks, err := parseHealthChecks(*flagCheckHealth, true)
		if err != nil && flagErr == nil {
			flagErr = fmt.Errorf("-pbt.checkhealth: %v", err)
		}
		if err == nil {
			CheckHealth(checks...)(&c)
		}
	}
	if *flagIgnoreHealth != "" {
		checks, err := parseHealthChecks(*flagIgnoreHealth, false)
		if err != nil {
			if flagErr == nil {
				flagErr = fmt.Errorf("-pbt.ignorehealth: %v", err)
			}
		}
		if err == nil {
			IgnoreHealth(checks...)(&c)
		}
	}
	for _, o := range opts {
		o(&c)
	}
	if flagErr != nil {
		return c, flagErr
	}
	switch {
	case c.causality < 0:
		return c, fmt.Errorf("Causality(%d): need at least 0 continuations", c.causality)
	case c.budget < 0:
		return c, fmt.Errorf("Budget(%s): need a nonnegative duration", c.budget)
	case c.maxFailures < 1:
		return c, fmt.Errorf("MaxFailures(%d): need at least one failure", c.maxFailures)
	case c.checks < 1:
		return c, fmt.Errorf("Checks(%d): need at least one check", c.checks)
	case c.search < 1:
		return c, fmt.Errorf("SearchSteps(%d): need at least one step", c.search)
	case c.startSize < 0 || c.maxSize < c.startSize:
		return c, fmt.Errorf("Sizes(%d, %d): need 0 <= start <= max", c.startSize, c.maxSize)
	case c.reportDepth < 0:
		return c, fmt.Errorf("ReportDepth(%d): need at least 0", c.reportDepth)
	case c.valueLimit < 0:
		return c, fmt.Errorf("ReportLimit(%d): need at least 0", c.valueLimit)
	}
	if !c.hasSeed {
		c.seed = randomSeed()
	}
	return c, nil
}

// Check runs prop against generated inputs and reports the minimal failing
// input it finds through tb.
//
// Check first replays the test case named by ReplayFile or -pbt.replay, if
// any, and then the counterexamples saved under testdata/pbt for this test,
// and fails if one of them still fails. It then runs the property for the
// configured number of cases with sizes growing from the start size to the
// maximum. On failure it shrinks the input, saves it, and reports every
// value the property drew together with the seed that reproduces the run.
func Check(tb TB, prop Property, opts ...Option) {
	tb.Helper()
	cfg, err := newConfig(opts)
	r := &runner{tb: tb, prop: prop, cfg: cfg, health: newHealthStats()}
	if cfg.reportJSON != "" || cfg.observe != "" {
		r.startOutput()
		tb = r.tb
		defer r.finishOutput()
	}
	if err != nil {
		r.outputStatus("error")
		tb.Errorf("[pbt] %v", err)
		return
	}
	if cfg.maxFailures > 1 {
		r.checkMultiple()
		return
	}
	if path := cfg.replayFile; path != "" {
		t, err := r.replayFile(path)
		if err != nil {
			r.outputStatus("error")
			tb.Errorf("[pbt] cannot replay %s: %v", path, err)
			return
		}
		if t.failed && !t.discarded {
			r.stableFailure(t)
			r.report(t, "replayed "+path+", which still fails", nil)
			return
		}
		tb.Logf("[pbt] replayed %s: the property passes on it", path)
	}
	if !cfg.noReplay {
		if t, path := r.replaySaved(); t != nil {
			r.stableFailure(t)
			r.report(t, "saved counterexample "+filepath.Base(path)+" still fails", nil)
			return
		}
	}
	fail, passed, err := r.search()
	if err != nil {
		r.reportReplayHealth()
		tb.Errorf("[pbt] %v", err)
		return
	}
	if fail == nil {
		r.reportHealth()
		r.reportStats(passed)
		return
	}
	fail = r.stableFailure(fail)
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
	cfg, err := newConfig(append(append([]Option(nil), opts...), MaxFailures(1), Causality(0)))
	// Existential candidates intentionally omit health checks.
	r := &runner{tb: tb, prop: prop, cfg: cfg, exists: true}
	if cfg.reportJSON != "" || cfg.observe != "" {
		r.startOutput()
		tb = r.tb
		defer r.finishOutput()
	}
	if err != nil {
		r.outputStatus("error")
		tb.Errorf("[pbt] %v", err)
		return
	}
	witness, tried, err := r.search()
	if err != nil {
		tb.Errorf("[pbt] %v", err)
		return
	}
	if witness == nil {
		r.outputStatus("no_witness")
		tb.Errorf("[pbt] no witness found in %d cases (seed 0x%x)", tried, cfg.seed)
		return
	}
	minimal, steps := r.shrink(witness)
	if r.output != nil {
		w := r.caseReport(minimal, "witness found")
		r.output.report.Witness = &w
	}
	tb.Logf("%s", r.format(minimal, fmt.Sprintf("witness found after %d rejected cases and %d shrink steps (seed 0x%x)", tried, steps, cfg.seed), nil))
}

type runner struct {
	causality   []string
	observer    *observer
	output      *runOutput
	budgetEnded bool
	failures    []*T
	savedPaths  map[string]string
	health      *healthStats
	healthExtra []healthNote
	tb          TB
	prop        Property
	cfg         config
	stats       *stats
	exists      bool        // look for a passing case instead of a failing one
	objectives  []objective // objectives in order of first appearance

	coverageRead func() float64 // nil uses testing.Coverage; tests may inject a reader
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
	return r.runReplay(prefix, size, rng, nil)
}

func (r *runner) runReplay(prefix []uint64, size int, rng *rand.Rand, realign *spanReplay) *T {
	t := newT(r.tb.Name(), prefix, size, r.cfg.maxSize, rng)
	if r.cfg.causality > 0 {
		t.causal = &causalTrace{}
	}
	t.realign = realign
	if rng != nil {
		t.health = r.health
	}
	prop := r.prop
	if setup := r.cfg.setup; setup != nil {
		prop = func(t *T) {
			if teardown := setup(t); teardown != nil {
				t.Cleanup(teardown)
			}
			r.prop(t)
		}
	}
	var started time.Time
	if r.observer != nil && r.observer.err == nil {
		r.observer.evaluations++
		t.timing = &caseTiming{evaluation: r.observer.evaluations}
		started = time.Now()
	}
	t.execute(prop)
	if t.timing != nil {
		t.timing.execution = max(0, time.Since(started)-t.timing.generation)
		r.observeCase(t, r.observer.phase, false)
	}
	if t.health != nil {
		t.health.add(t)
	}
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
	if r.cfg.verbose || !st.empty() || len(r.objectives) > 0 || r.budgetEnded {
		summary := fmt.Sprintf("OK, passed %d checks (seed 0x%x)", passed, r.cfg.seed)
		if r.budgetEnded {
			summary = fmt.Sprintf("OK, passed %d checks in the %s budget (seed 0x%x)", passed, r.cfg.budget, r.cfg.seed)
		}
		lines := []string{summary}
		for _, o := range r.objectives {
			lines = append(lines, fmt.Sprintf("best %s = %s over %d search steps", o.name, formatNum(o.bestValue), passed))
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
	if und := st.undecided(); len(und) > 0 && r.cfg.checkCover {
		r.tb.Errorf("[pbt] insufficient evidence for the coverage after %d cases; raise Checks to run more:\n[pbt]   %s",
			passed, strings.Join(und, "\n[pbt]   "))
	}
}

// search runs test cases until one is wanted or the budget is spent. Under
// CheckCoverage the budget stretches, up to coverCases times, while a Cover
// requirement is undecided. Fresh
// cases grow in size by one per case, as in PropEr. Once a case reports a
// utility, the search becomes targeted: it mutates the current case and
// moves to the neighbor when its utility is higher, or with the Metropolis
// probability exp(-loss/(temperature*scale)) when it is lower, where the
// scale is a hundredth of the utility range seen so far. That is PropEr's
// simulated annealing in a scale-free form; the best case ever seen is kept
// apart for the report. A small share of the steps restart from a fresh
// case.
func (r *runner) search() (found *T, passed int, err error) {
	if r.output != nil {
		start := time.Now()
		defer func() {
			r.output.report.Durations.Search += time.Since(start).Seconds()
			if err != nil {
				r.outputStatus("gave_up")
			}
		}()
	}
	cfg := r.cfg
	var deadline time.Time
	if cfg.budget > 0 {
		deadline = time.Now().Add(cfg.budget)
	}
	guided := r.startCoverage()
	if guided != nil {
		defer func() {
			if cfg.verbose || (found == nil && err == nil && len(r.failures) == 0) {
				guided.report(r.tb)
			}
		}()
	}
	r.stats = newStats()
	discards := 0
	tried := 0
	targeted := false
	nextObjective := 0
	for {
		limit := cfg.checks
		if targeted {
			limit = cfg.search
		}
		if tried >= limit {
			extend := cfg.checkCover && !targeted && !r.exists && tried < coverCases*cfg.checks && !r.stats.settled()
			if !extend {
				return nil, passed, nil
			}
		}
		if cfg.budget > 0 && !time.Now().Before(deadline) {
			r.budgetEnded = true
			if r.exists || len(r.failures) > 0 {
				r.tb.Logf("[pbt] search stopped after %d cases and %d discards in the %s budget (seed 0x%x)", tried, discards, cfg.budget, cfg.seed)
			}
			return nil, passed, nil
		}
		if r.observer != nil {
			r.observer.phase = "generate"
			if targeted {
				r.observer.phase = "target"
			}
		}
		rng := newRand(caseSeed(cfg.seed, uint64(tried+discards)))
		temp := 1 - float64(tried)/float64(limit)
		var t *T
		selected := -1
		var current *T
		if len(r.objectives) > 0 {
			selected = nextObjective % len(r.objectives)
			nextObjective = selected + 1
			current = r.objectives[selected].current
		}
		var seed *T
		if !targeted && guided != nil {
			seed = guided.seed(rng)
			if seed != nil && r.observer != nil {
				r.observer.phase = "coverage"
			}
		}
		if targeted && current != nil && rng.Float64() >= restartProbability {
			t = r.run(mutate(rng, current, temp), current.size, rng)
		} else if seed != nil {
			t = r.run(mutate(rng, seed, coverageTemperature), seed.size, rng)
		} else {
			t = r.run(nil, min(cfg.startSize+tried, cfg.maxSize), rng)
		}
		// Mutants of coverage seeds are not ordinary random generation.
		r.observeDistinct(t, targeted || seed != nil)
		if r.output != nil {
			r.outputSearchCase(t)
		}
		if guided != nil {
			guided.observe(t)
		}
		switch {
		case t.discarded:
			discards++
			if discards > maxDiscardRatio*max(cfg.checks, limit, passed) {
				detail := ""
				if r.health != nil {
					detail = "; sources: " + topHealthSources(r.health.sources, " discards")
				}
				return nil, passed, fmt.Errorf("gave up after %d discarded test cases (%d passed); last reason: %s%s", discards, passed, t.discardReason, detail)
			}
		case r.want(t) && (r.exists || cfg.maxFailures == 1):
			return t, passed, nil
		default:
			if r.want(t) {
				r.collectFailure(t)
				if len(r.failures) >= cfg.maxFailures {
					return t, passed, nil
				}
			} else {
				r.stats.add(t)
				passed++
			}
			tried++
			if t.hasUtility {
				targeted = true
				r.observeObjectives(t, selected, temp, rng)
			}
		}
	}
}
