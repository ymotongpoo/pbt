// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package pbt

import (
	"math/rand/v2"
	"slices"
)

// Maximize declares the utility of the current test case for a targeted
// search, as PropEr's ?MAXIMIZE does. Once a case reports a utility, Check
// stops drawing fresh cases and instead mutates the best case found so far,
// looking for inputs with a higher utility, for the number of search steps
// in the configuration. The name labels the utility in the report.
func (t *T) Maximize(name string, utility float64) {
	t.hasUtility = true
	t.utilityName = name
	t.utility = utility
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
