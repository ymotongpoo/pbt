package pbt

import (
	"fmt"
	"math/rand/v2"
	"runtime/debug"
	"sync"
	"time"
)

// choiceKind tells the shrinker how a recorded choice maps to a value.
type choiceKind uint8

const (
	// kindPlain choices map monotonically to values: a smaller choice is a
	// simpler value.
	kindPlain choiceKind = iota
	// kindFlag choices are booleans and indices that decide structure
	// (continue a list, pick an alternative). They are never redistributed.
	kindFlag
	// kindZigzag choices encode a signed offset from a center; see
	// zigzagDecode and offsetOf.
	kindZigzag
)

// choiceMeta describes how a recorded choice maps to a value, so that the
// shrinker can reason in value space.
type choiceMeta struct {
	kind choiceKind
	// zz and tailPos describe zigzag choices: choices up to zz alternate
	// between the two sides of the center, choices beyond zz continue on
	// the positive side when tailPos is set and on the negative side
	// otherwise.
	zz      uint64
	tailPos bool
}

var (
	plainMeta = choiceMeta{kind: kindPlain}
	flagMeta  = choiceMeta{kind: kindFlag}
)

// Defaults shared by Check and Gen.Example.
const (
	defaultChecks    = 100
	defaultStartSize = 1
	defaultMaxSize   = 42
	defaultShrink    = 30 * time.Second
	defaultTestdata  = "testdata/pbt"
	maxDiscardRatio  = 10
)

// maxSpanDepth bounds generator recursion. Go cannot recover from a stack
// overflow, so a runaway recursive generator discards the test case instead.
const maxSpanDepth = 1000

// drawMode selects the distribution used when a choice is generated afresh.
type drawMode uint8

const (
	modeBiased  drawMode = iota // small values and edges are over-represented
	modeUniform                 // every value is equally likely
)

// span records which choices one generator consumed. Spans nest like the
// generators that produced them and are the unit the shrinker deletes,
// reorders and replaces.
type span struct {
	label      string
	start, end int
	parent     int // index of the enclosing span, or -1
	depth      int
	noshrink   bool // the shrinker must not change the choices inside
}

// drawRecord is one value drawn directly by the property, kept for the
// report.
type drawRecord struct {
	label string
	gen   string
	value any
}

type (
	stopTest    struct{}
	discardTest struct{ reason string }
)

// Property is a test case parameterized by the values it draws from t.
type Property func(t *T)

// T is the handle a property receives. It records every choice the
// generators make, offers the reporting methods of testing.TB, and carries
// the size parameter of the current test case.
//
// A property fails when it calls one of the Error, Fatal or Fail methods, or
// when it panics. Skip and Assume discard the current test case instead.
type T struct {
	name string

	rng     *rand.Rand // source of fresh choices; nil when replaying
	prefix  []uint64   // choices replayed before falling back to rng or zero
	choices []uint64
	metas   []choiceMeta
	spans   []span
	open    []int // stack of indices into spans

	size    int
	maxSize int

	draws []drawRecord

	// Statistics of this test case, merged into the run's totals when the
	// case passes.
	labels   []string
	measures []measurement
	covers   []coverRecord

	mu            sync.Mutex // guards failed, messages and logs, which Timeout touches from another goroutine
	failed        bool
	discarded     bool
	discardReason string
	messages      []string
	logs          []string
	panicVal      any
	stack         []byte
	cleanups      []func()
	onFail        []func()
}

type measurement struct {
	name  string
	value float64
}

type coverRecord struct {
	label string
	pct   float64
	hit   bool
}

func newT(name string, prefix []uint64, size, maxSize int, rng *rand.Rand) *T {
	return &T{name: name, rng: rng, prefix: prefix, size: size, maxSize: maxSize}
}

// draw records the next choice in [0, max] and returns it. While a prefix is
// being replayed the recorded choice is clamped to max; once the prefix and
// the random source are exhausted, every choice is 0, the simplest value.
func (t *T) draw(max uint64, meta choiceMeta, mode drawMode) uint64 {
	var v uint64
	switch i := len(t.choices); {
	case i < len(t.prefix):
		v = min(t.prefix[i], max)
	case t.rng != nil:
		v = fresh(t.rng, max, mode, t.size, t.maxSize)
	}
	t.choices = append(t.choices, v)
	t.metas = append(t.metas, meta)
	return v
}

// drawBool records a boolean choice that is true with probability p when it
// is generated afresh. False is the simpler value.
func (t *T) drawBool(p float64) bool {
	var v uint64
	switch i := len(t.choices); {
	case i < len(t.prefix):
		v = min(t.prefix[i], 1)
	case t.rng != nil:
		if t.rng.Float64() < p {
			v = 1
		}
	}
	t.choices = append(t.choices, v)
	t.metas = append(t.metas, flagMeta)
	return v == 1
}

// drawIndex records an index into weights. When generated afresh the index
// is sampled proportionally to its weight; an index with weight 0 is never
// generated but can be reached by shrinking.
func (t *T) drawIndex(weights []int, total int) int {
	n := uint64(len(weights) - 1)
	var v uint64
	switch i := len(t.choices); {
	case i < len(t.prefix):
		v = min(t.prefix[i], n)
	case t.rng != nil:
		r := t.rng.IntN(total)
		for k, w := range weights {
			if r < w {
				v = uint64(k)
				break
			}
			r -= w
		}
	}
	t.choices = append(t.choices, v)
	t.metas = append(t.metas, flagMeta)
	return int(v)
}

// forced records a choice whose value the generator dictates, for example
// the decision to continue a list that is still below its minimum length.
// Recording it keeps the sequence aligned with the one a free draw produces.
func (t *T) forced(v uint64) {
	t.choices = append(t.choices, v)
	t.metas = append(t.metas, flagMeta)
}

func (t *T) beginSpan(label string) {
	parent, depth := -1, 0
	if n := len(t.open); n > 0 {
		parent = t.open[n-1]
		depth = t.spans[parent].depth + 1
		if depth > maxSpanDepth {
			t.discard(fmt.Sprintf("generators nested more than %d levels deep in %s; bound the recursion, for example with Sized", maxSpanDepth, label))
		}
	}
	t.spans = append(t.spans, span{label: label, start: len(t.choices), end: -1, parent: parent, depth: depth})
	t.open = append(t.open, len(t.spans)-1)
}

func (t *T) endSpan() {
	n := len(t.open)
	i := t.open[n-1]
	t.open = t.open[:n-1]
	t.spans[i].end = len(t.choices)
}

func (t *T) closeSpans() {
	for len(t.open) > 0 {
		t.endSpan()
	}
}

// execute runs prop against t, turning the control-flow panics of FailNow and
// Skip into state and recording any other panic as a failure.
func (t *T) execute(prop Property) {
	defer t.closeSpans()
	defer func() {
		switch rec := recover().(type) {
		case nil:
		case stopTest:
			t.failed = true
		case discardTest:
			t.discarded = true
			t.discardReason = rec.reason
		default:
			t.failed = true
			t.panicVal = rec
			t.stack = debug.Stack()
		}
	}()
	defer t.runCleanups()
	prop(t)
}

func (t *T) runCleanups() {
	for i := len(t.cleanups) - 1; i >= 0; i-- {
		t.cleanups[i]()
	}
	t.cleanups = nil
}

func (t *T) discard(reason string) {
	panic(discardTest{reason: reason})
}

// Name returns the name of the test that called Check.
func (t *T) Name() string { return t.name }

// Helper exists so that *T satisfies interfaces modeled on testing.TB. It
// does nothing.
func (t *T) Helper() {}

// Size returns the size parameter of the current test case. Sizes start at
// the configured start size and grow by one per passing case up to the
// maximum; collection generators scale their lengths with it.
func (t *T) Size() int { return t.size }

// Errorf records a failure and continues the test case.
func (t *T) Errorf(format string, args ...any) {
	t.fail(fmt.Sprintf(format, args...))
}

// Error records a failure and continues the test case.
func (t *T) Error(args ...any) {
	t.fail(fmt.Sprint(args...))
}

func (t *T) fail(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failed = true
	t.messages = append(t.messages, msg)
}

// Fatalf records a failure and stops the test case.
func (t *T) Fatalf(format string, args ...any) {
	t.Errorf(format, args...)
	t.FailNow()
}

// Fatal records a failure and stops the test case.
func (t *T) Fatal(args ...any) {
	t.Error(args...)
	t.FailNow()
}

// Fail marks the test case as failed and continues.
func (t *T) Fail() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.failed = true
}

// FailNow marks the test case as failed and stops it. It must be called from
// the goroutine running the property.
func (t *T) FailNow() {
	t.Fail()
	panic(stopTest{})
}

// Failed reports whether the test case has failed.
func (t *T) Failed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.failed
}

// Logf records a message that is shown when the test case is the reported
// counterexample.
func (t *T) Logf(format string, args ...any) {
	t.log(fmt.Sprintf(format, args...))
}

// Log records a message that is shown when the test case is the reported
// counterexample.
func (t *T) Log(args ...any) {
	t.log(fmt.Sprint(args...))
}

func (t *T) log(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.logs = append(t.logs, msg)
}

// Skip discards the test case; it counts neither as passed nor as failed.
func (t *T) Skip(args ...any) { t.discard(fmt.Sprint(args...)) }

// Skipf discards the test case; it counts neither as passed nor as failed.
func (t *T) Skipf(format string, args ...any) { t.discard(fmt.Sprintf(format, args...)) }

// SkipNow discards the test case; it counts neither as passed nor as failed.
func (t *T) SkipNow() { t.discard("skipped") }

// Assume discards the test case unless cond holds. It is the equivalent of
// PropEr's ?IMPLIES. Check gives up when too many cases are discarded.
func (t *T) Assume(cond bool) {
	if !cond {
		t.discard("assumption failed")
	}
}

// Cleanup registers f to run after the test case, whether it passed or not.
func (t *T) Cleanup(f func()) { t.cleanups = append(t.cleanups, f) }

// OnFail registers f to run when this test case is reported as the
// counterexample, after shrinking. It is the equivalent of PropEr's
// ?WHENFAIL. Output written with Logf inside f appears in the report.
func (t *T) OnFail(f func()) { t.onFail = append(t.onFail, f) }

// Collect records v for the statistics that Check prints when the property
// passes, as PropEr's collect/2 does. Values are grouped by their %v text.
func (t *T) Collect(v any) { t.labels = append(t.labels, fmt.Sprint(v)) }

// Classify records label when cond holds, as PropEr's classify/3 does.
func (t *T) Classify(cond bool, label string) {
	if cond {
		t.labels = append(t.labels, label)
	}
}

// Aggregate records every value, as PropEr's aggregate/2 does with a list.
// Use it for the names of the commands a stateful test ran.
func (t *T) Aggregate(values ...any) {
	for _, v := range values {
		t.labels = append(t.labels, fmt.Sprint(v))
	}
}

// Measure records a numeric sample under name; Check prints its minimum,
// average and maximum, as PropEr's measure/3 does.
func (t *T) Measure(name string, value float64) {
	t.measures = append(t.measures, measurement{name: name, value: value})
}

// Cover declares that the cases for which cond holds must make up at least
// pct percent of all passing cases, as QuickCheck's cover does. Check warns
// when the observed share falls short, and fails instead when the
// CheckCoverage option is set.
func (t *T) Cover(cond bool, pct float64, label string) {
	t.covers = append(t.covers, coverRecord{label: label, pct: pct, hit: cond})
}

// runOnFail runs the OnFail callbacks of the reported counterexample.
func (t *T) runOnFail() {
	for _, f := range t.onFail {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					t.Logf("OnFail callback panicked: %v", rec)
				}
			}()
			f()
		}()
	}
}
