// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// CoverageGuided enables an experimental search using testing.Coverage.
// The -pbt.coverage flag does the same. It is off by default. Without
// coverage instrumentation it logs a hint once per run and uses the usual
// search. Run go test -cover -coverpkg=... to choose which packages count.
//
// Passing generated cases that increase coverage enter a FIFO pool of at
// most 64 seeds. Once the pool is nonempty, attempts alternate between
// mutating a uniformly selected seed and generating a fresh case. Mutations
// use the seed's size and count toward Checks like fresh cases. Maximize
// or Minimize takes priority: once a case starts the targeted search, its
// mutation schedule and SearchSteps budget apply instead. CheckExists
// ignores this option because its first passing case ends the search.
//
// Coverage counters are process-wide: parallel tests (including t.Parallel)
// and other goroutines pollute the signal, so novelty is only a heuristic.
// Only block coverage is available, without hit counts or edges. Each
// reading costs O(number of counters). The reported fraction is the live
// estimate from testing.Coverage, not the statement-weighted percentage
// printed by go test. The -coverpkg selection determines the signal; when
// pbt itself is instrumented, its execution contributes too.
//
// Coverage already reached by earlier tests cannot guide this run. A Seed
// alone cannot reproduce a coverage-guided schedule across different
// coverage histories; saved counterexamples still replay normally. The
// pool is local to one Check and is not saved between runs.
func CoverageGuided() Option { return func(c *config) { c.coverage = true } }

const coveragePoolSize = 64

// A low fixed temperature favors small mutations while retaining the
// existing mutator's random replacement and structural changes.
const coverageTemperature = 0.1

type coverageSearch struct {
	read         func() float64
	start, last  float64
	pool         []*T
	freshNext    bool
	cases, novel int
}

func newCoverageSearch(read func() float64) *coverageSearch {
	start := read()
	return &coverageSearch{read: read, start: start, last: start}
}

func (r *runner) startCoverage() *coverageSearch {
	if !r.cfg.coverage || r.exists {
		return nil
	}
	read := r.coverageRead
	if read == nil {
		if testing.CoverMode() == "" {
			r.tb.Logf("[pbt] coverage-guided: run is not instrumented; run go test -cover -coverpkg=... to enable it")
			return nil
		}
		read = testing.Coverage
	}
	return newCoverageSearch(read)
}

func (c *coverageSearch) seed(rng *rand.Rand) *T {
	if len(c.pool) == 0 {
		return nil
	}
	if c.freshNext {
		c.freshNext = false
		return nil
	}
	c.freshNext = true
	return c.pool[rng.IntN(len(c.pool))]
}

func (c *coverageSearch) observe(t *T) {
	now := c.read()
	grew := now > c.last
	c.last = now
	// Read rejected cases too, so their coverage is not attributed to the
	// next passing case. Only passing cases can become mutation seeds.
	if t.discarded || t.failed {
		return
	}
	c.cases++
	if !grew {
		return
	}
	c.novel++
	if len(t.choices) == 0 {
		return
	}
	// Keep only the data mutate and run need, not drawn values, callbacks
	// or the rest of the completed case's object graph.
	seed := &T{
		size: t.size, choices: slices.Clone(t.choices),
		metas: slices.Clone(t.metas), spans: slices.Clone(t.spans),
	}
	if len(c.pool) == coveragePoolSize {
		copy(c.pool, c.pool[1:])
		c.pool[len(c.pool)-1] = seed
	} else {
		c.pool = append(c.pool, seed)
	}
}

func (c *coverageSearch) report(tb TB) {
	tb.Logf("[pbt] coverage-guided: %d of %d passing cases reached new code, %d seeds kept, coverage %.1f%% -> %.1f%%",
		c.novel, c.cases, len(c.pool), 100*c.start, 100*c.last)
}
