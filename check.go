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
// cases grow up to, as PropEr's start_size and max_size do.
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
	}
	for _, o := range opts {
		o(&c)
	}
	switch {
	case c.checks < 1:
		return c, fmt.Errorf("Checks(%d): need at least one check", c.checks)
	case c.startSize < 1 || c.maxSize < c.startSize:
		return c, fmt.Errorf("Sizes(%d, %d): need 1 <= start <= max", c.startSize, c.maxSize)
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

type runner struct {
	tb    TB
	prop  Property
	cfg   config
	stats *stats
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
	if r.cfg.verbose || !st.empty() {
		lines := append([]string{fmt.Sprintf("OK, passed %d checks (seed 0x%x)", passed, r.cfg.seed)}, st.lines()...)
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

// search runs fresh test cases until one fails or enough pass. The size
// grows by one per passing case, as in PropEr.
func (r *runner) search() (fail *T, passed int, err error) {
	cfg := r.cfg
	r.stats = newStats()
	discards := 0
	for passed < cfg.checks {
		size := min(cfg.startSize+passed, cfg.maxSize)
		t := r.run(nil, size, newRand(caseSeed(cfg.seed, uint64(passed+discards))))
		switch {
		case t.discarded:
			discards++
			if discards > maxDiscardRatio*cfg.checks {
				return nil, passed, fmt.Errorf("gave up after %d discarded test cases (%d passed); last reason: %s",
					discards, passed, t.discardReason)
			}
		case t.failed:
			return t, passed, nil
		default:
			r.stats.add(t)
			passed++
		}
	}
	return nil, passed, nil
}
