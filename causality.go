// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
)

// Causality samples continuations of the first failing case before shrinking
// and appends their failure rates to its report. continuations is the number
// of attempts per prefix; 0 (the default) disables analysis. The flag
// -pbt.causality sets the same value. CheckExists ignores this option.
//
// Steps are the generated commands of Commands, including fsm.Commands.
// Each prefix keeps all choices through the end of its last command, then
// draws fresh choices at the original size. This includes earlier draws and
// rejected command attempts. After zero steps, all choices are fresh.
// Properties without commands are skipped: top-level draws need not describe
// a sequence of state changes. ParallelCommands is also skipped: generation
// order is not execution order, and branching may trim generated commands.
//
// Analysis uses at most 16*continuations additional property executions,
// apart from the usual stability replay. It samples up to eight evenly
// spaced prefixes and bisects the largest measured rise up to eight times.
// A step associated with a rise is a candidate cause, not proof of causation;
// sampling noise and nonmonotone rates can obscure it. A long trace may only
// be narrowed to a range within this budget. Discards do not enter the rate.
// If the full input fails to reproduce its original failure identity on the
// stability replay, analysis is skipped.
//
// Continuations use separate random streams derived from Seed. They run
// Setup and Cleanup as usual, so the property must reset its system on each
// run. They do not count as checks, affect run statistics, health checks or
// objectives, invoke OnFail, or save cases. With MaxFailures only the first
// failure is analysed, keeping the cost independent of the number of bugs.
// Saved cases are analysed as replayed; their original unshrunk input is not
// available. Reproducibility requires a deterministic property and seed.
func Causality(continuations int) Option {
	return func(c *config) { c.causality = continuations }
}

const causalityPoints = 8

type causalStep struct {
	end     int
	command Command
}

// Keep the command span's boundary even when endSpan drops a leaf span.
// This record neither retains extra shrink spans nor changes any choices.
type causalTrace struct {
	steps    []causalStep
	parallel bool
}

type causalPoint struct {
	prefix, valid, failures int
}

// analyzeCausality runs after the existing stability replay and before any
// shrinking. checkMultiple prepares all its failures before reporting them;
// keeping the lines on the runner analyses and reports only the first one.
func (r *runner) analyzeCausality(original *T, stable bool) {
	if r.cfg.causality == 0 || r.exists || len(r.causality) > 0 {
		return
	}
	r.causality = []string{fmt.Sprintf("causality (%d continuations per prefix, seed 0x%x):", r.cfg.causality, r.cfg.seed)}
	if !stable {
		r.causality = append(r.causality, "  skipped: the full input did not reproduce the same failure (flaky)")
		return
	}
	trace := original.causal
	if trace != nil && trace.parallel {
		r.causality = append(r.causality, "  skipped: ParallelCommands generation order is not execution order")
		return
	}
	if trace == nil || len(trace.steps) == 0 {
		r.causality = append(r.causality, "  skipped: no sequential command steps")
		return
	}
	r.causality = append(r.causality, "  steps are generated commands of the first failing case, before shrinking in this run")

	// Freeze command descriptions before continuations reset the model or
	// mutate arguments. Use the same call formatting and limit as reports.
	names := make([]string, len(trace.steps))
	for i, step := range trace.steps {
		names[i] = strings.TrimPrefix(formatDrawLimit(drawRecord{gen: "command", value: step.command}, r.cfg.valueLimit), "command = ")
	}
	// A separate runner keeps health, statistics, objectives and discovered
	// failures private. Continuations need no traces of their own.
	cfg := r.cfg
	cfg.causality = 0
	diagnostic := runner{tb: r.tb, prop: r.prop, cfg: cfg}
	var measured []causalPoint
	measure := func(k int) {
		for _, point := range measured {
			if point.prefix == k {
				return
			}
		}
		end := 0
		if k > 0 {
			end = trace.steps[k-1].end
		}
		point := causalPoint{prefix: k}
		// Domain separation avoids using any of the search's streams. Each
		// prefix/attempt has a stable seed independent of measurement order.
		seed := caseSeed(r.cfg.seed^0x63617573616c6974, uint64(k))
		for i := 0; i < r.cfg.causality; i++ {
			t := diagnostic.run(original.choices[:end], original.size, newRand(caseSeed(seed, uint64(i))))
			if !t.discarded {
				point.valid++
				if t.failed && t.failKey == original.failKey {
					point.failures++
				}
			}
		}
		measured = append(measured, point)
		slices.SortFunc(measured, func(a, b causalPoint) int { return cmp.Compare(a.prefix, b.prefix) })
	}
	n := len(trace.steps)
	for i := 0; i < causalityPoints; i++ {
		measure(int(int64(n) * int64(i) / (causalityPoints - 1)))
	}
	for range causalityPoints {
		lo, hi := largestCausalRise(measured)
		if hi == 0 || hi-lo == 1 {
			break
		}
		measure(lo + (hi-lo)/2)
	}
	lo, hi := largestCausalRise(measured)
	for _, point := range measured {
		line := fmt.Sprintf("  after %d of %d steps: ", point.prefix, n)
		if point.valid == 0 {
			line += fmt.Sprintf("all %d continuations discarded; rate unavailable", r.cfg.causality)
		} else {
			line += fmt.Sprintf("%d/%d fail with the same identity", point.failures, point.valid)
			if discarded := r.cfg.causality - point.valid; discarded > 0 {
				line += fmt.Sprintf(" (%d discarded)", discarded)
			}
		}
		if hi != 0 && point.prefix == hi && hi-lo == 1 {
			line += fmt.Sprintf(" <- largest measured rise at step %d: %s", hi, names[hi-1])
		}
		r.causality = append(r.causality, strings.Split(strings.ReplaceAll(line, "\n", "\n    "), "\n")...)
	}
	if hi == 0 {
		r.causality = append(r.causality, "  no rise found between prefixes with available rates")
	} else if hi-lo > 1 {
		r.causality = append(r.causality, fmt.Sprintf("  largest measured rise between steps %d and %d; prefix budget exhausted", lo+1, hi))
	}
}

// Return the boundaries of the largest positive rise. Ties favour the
// earliest interval; an unavailable rate never bridges two intervals.
func largestCausalRise(points []causalPoint) (lo, hi int) {
	best := 0.0
	for i := 1; i < len(points); i++ {
		a, b := points[i-1], points[i]
		if a.valid == 0 || b.valid == 0 {
			continue
		}
		rise := float64(b.failures)/float64(b.valid) - float64(a.failures)/float64(a.valid)
		if rise > best {
			lo, hi, best = a.prefix, b.prefix, rise
		}
	}
	return lo, hi
}
