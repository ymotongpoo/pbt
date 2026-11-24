// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"math"
	"math/rand/v2"
	"slices"
)

// Maximize declares a named utility of the current case for targeted search.
// Each objective has its own current and best case. Steps mutate the current
// case of one objective, cycling through names in order of first appearance;
// simulated annealing may accept a lower utility to escape local optima.
// Calling Maximize again with the same name replaces its value. A case that
// omits an objective cannot become its current case. SearchSteps sets the
// total budget shared by all objectives.
func (t *T) Maximize(name string, value float64) {
	if !t.hasUtility || t.utilityName == name {
		t.hasUtility, t.utilityName, t.utility = true, name, value
		return
	}
	for i := range t.moreUtilities {
		if t.moreUtilities[i].name == name {
			t.moreUtilities[i].value = value
			return
		}
	}
	t.moreUtilities = append(t.moreUtilities, utility{name, value})
}

type utility struct {
	name  string
	value float64
}

// Keep the first utility inline: the common single-objective case allocates
// no collection in T.
func (t *T) utilityCount() int {
	if !t.hasUtility {
		return 0
	}
	return 1 + len(t.moreUtilities)
}
func (t *T) utilityAt(i int) utility {
	if i == 0 {
		return utility{t.utilityName, t.utility}
	}
	return t.moreUtilities[i-1]
}

type objective struct {
	name                    string
	current, best           *T
	currentValue, bestValue float64
	uMin, uMax              float64
}

// Observe all reported utilities, but make the annealing decision only for
// the objective selected for this step. New objectives start from this case.
func (r *runner) observeObjectives(t *T, selected int, temp float64, rng *rand.Rand) {
	for i := 0; i < t.utilityCount(); i++ {
		u := t.utilityAt(i)
		index := -1
		for j := range r.objectives {
			if r.objectives[j].name == u.name {
				index = j
				break
			}
		}
		if index < 0 {
			r.objectives = append(r.objectives, objective{
				name: u.name, current: t, best: t, currentValue: u.value, bestValue: u.value,
				uMin: u.value, uMax: u.value,
			})
			continue
		}
		o := &r.objectives[index]
		o.uMin, o.uMax = min(o.uMin, u.value), max(o.uMax, u.value)
		if u.value > o.bestValue {
			o.best, o.bestValue = t, u.value
		}
		if index != selected {
			continue
		}
		scale := (o.uMax - o.uMin) / 100
		if u.value > o.currentValue || (scale > 0 && temp > 0 && rng.Float64() < math.Exp(-(o.currentValue-u.value)/(scale*temp))) {
			o.current, o.currentValue = t, u.value
		}
	}
}

// Minimize is Maximize with the sign reversed.
func (t *T) Minimize(name string, utility float64) {
	t.Maximize(name, -utility)
}

// restartProbability is how often the targeted search draws a fresh case
// instead of a neighbor of the best one, to escape local optima.
const restartProbability = 0.05

// mutate returns a neighbor of the best case's choice sequence. The
// temperature, from 1 down to 0 over the search, scales the size of the
// changes. The neighborhood mixes small changes of single values, random
// values, and deletion or duplication of whole spans, which grows or shrinks
// collections.
func mutate(rng *rand.Rand, best *T, temp float64) []uint64 {
	c := slices.Clone(best.choices)
	if len(c) == 0 {
		return c
	}
	switch rng.IntN(6) {
	case 0, 1, 2:
		// A log-uniform step whose largest size shrinks with the
		// temperature: wide jumps early, unit steps at the end.
		i := rng.IntN(len(c))
		width := 1 + rng.IntN(1+int(temp*40))
		delta := 1 + rng.Uint64N(uint64(1)<<width)
		if m := best.metas[i]; m.kind == kindZigzag {
			v := offsetOf(c[i], m)
			if rng.IntN(2) == 0 {
				v += int64(delta)
			} else {
				v -= int64(delta)
			}
			c[i] = choiceOf(v, m)
		} else if rng.IntN(2) == 0 {
			c[i] += delta
		} else if c[i] >= delta {
			c[i] -= delta
		} else {
			c[i] = 0
		}
	case 3:
		c[rng.IntN(len(c))] = rng.Uint64() >> rng.IntN(64)
	case 4:
		if sp, ok := randomSpan(rng, best); ok {
			c = slices.Concat(c[:int(sp.start)], c[int(sp.end):])
		}
	case 5:
		if sp, ok := randomSpan(rng, best); ok {
			c = slices.Concat(c[:int(sp.end)], c[int(sp.start):int(sp.end)], c[int(sp.end):])
		}
	}
	return c
}

func randomSpan(rng *rand.Rand, t *T) (span, bool) {
	if len(t.spans) == 0 {
		return span{}, false
	}
	sp := t.spans[rng.IntN(len(t.spans))]
	return sp, sp.end > sp.start && int(sp.end) <= len(t.choices)
}
